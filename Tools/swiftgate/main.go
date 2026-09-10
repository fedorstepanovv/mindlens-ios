// Command swiftgate decides whether a pull request may merge into main.
//
// It runs two passes over the lines a PR added. The first is deterministic: a rule
// catalogue for the Flutter habits and non-idiomatic Swift that a regular expression
// can see. The second hands the diff, the project's own documents, and the Flutter app
// that serves as the product spec to Claude, and asks the question grep cannot: does
// this read as native Swift, or as translated Dart?
//
// Blocking findings exit non-zero. That exit code, wired to a required status check,
// is the thing that actually stops a merge.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/ghpr"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/review"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

const (
	exitPass    = 0
	exitBlocked = 1
	exitError   = 2
)

type options struct {
	repoDir    string
	flutterDir string
	configPath string
	base       string
	head       string
	pr         int
	staticOnly bool
	comment    bool
	inline     bool
	jsonOut    string
	verbose    bool
	timeout    time.Duration
}

func main() {
	opt := options{}
	flag.StringVar(&opt.repoDir, "repo", ".", "Swift repository to review")
	flag.StringVar(&opt.flutterDir, "flutter", ".swiftgate/flutter", "checkout of the Flutter app used as the product spec; ignored when absent")
	flag.StringVar(&opt.configPath, "config", ".github/swiftgate.yml", "gate configuration, relative to the repository")
	flag.StringVar(&opt.base, "base", envOr("GITHUB_BASE_REF", "main"), "branch this PR merges into")
	flag.StringVar(&opt.head, "head", "HEAD", "commit under review")
	flag.IntVar(&opt.pr, "pr", envInt("SWIFTGATE_PR", 0), "pull request number; enables commenting")
	flag.BoolVar(&opt.staticOnly, "static-only", false, "run only the deterministic pass (no API key needed)")
	flag.BoolVar(&opt.comment, "comment", false, "post the verdict to the pull request")
	flag.BoolVar(&opt.inline, "inline", false, "also attach findings to the lines they are about (best-effort)")
	flag.StringVar(&opt.jsonOut, "json", "", "write findings to this file as JSON")
	flag.BoolVar(&opt.verbose, "verbose", false, "log each turn the reviewer takes")
	flag.DurationVar(&opt.timeout, "timeout", 20*time.Minute, "give up after this long")
	flag.Parse()

	os.Exit(run(opt))
}

func run(opt options) int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, opt.timeout)
	defer cancelTimeout()

	logw := io.Discard
	if opt.verbose {
		logw = os.Stderr
	}

	cfg, err := loadConfig(opt.repoDir, opt.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "swiftgate: %v\n", err)
		return exitError
	}

	base := normaliseBase(opt.repoDir, opt.base)
	diff, err := scan.Collect(opt.repoDir, base, opt.head, cfg.MaxDiffBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "swiftgate: %v\n", err)
		return exitError
	}

	result := &gate.Result{}

	if len(diff.SwiftFiles()) == 0 {
		result.Skipped = "no Swift changed in this pull request."
		return finish(ctx, opt, cfg, result, diff, "")
	}

	// Deterministic pass first. It is free, it is certain, and its findings are handed
	// to the reviewer so the two passes do not say the same thing twice.
	static := scan.Static{RepoDir: opt.repoDir, Severities: cfg.severities()}
	staticFindings := static.Run(diff)
	result.Add(staticFindings...)
	fmt.Fprintf(os.Stderr, "swiftgate: %d file(s) changed, %d static finding(s)\n", len(diff.SwiftFiles()), len(staticFindings))

	verdict := ""
	if !opt.staticOnly {
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			fmt.Fprintln(os.Stderr, "swiftgate: ANTHROPIC_API_KEY is not set — cannot run the idiom review")
			return exitError
		}

		meta := review.Meta{}
		if pr := prMetadata(ctx, opt); pr != nil {
			meta.Title, meta.Body = pr.Title, pr.Body
		}

		findings, v, usage, err := review.Review(ctx, review.Config{
			RepoDir:    opt.repoDir,
			FlutterDir: flutterCheckout(opt),
			Model:      cfg.Model,
			Effort:     cfg.Effort,
			MaxTurns:   cfg.MaxTurns,
			Log:        logw,
		}, diff, meta, staticFindings)
		if err != nil {
			fmt.Fprintf(os.Stderr, "swiftgate: %v\n", err)
			return exitError
		}
		result.Add(findings...)
		result.Usage = usage
		verdict = v
		fmt.Fprintf(os.Stderr, "swiftgate: reviewer added %d finding(s) over %d turn(s), ~$%.2f\n",
			len(findings), usage.Turns, usage.EstimatedUSDCents/100)
	}

	result.Normalise()
	result.Override = overrideReason(ctx, opt, cfg, result)

	return finish(ctx, opt, cfg, result, diff, verdict)
}

// finish writes every output the run produces, then converts the verdict into an exit
// code. Reporting happens before the exit decision so a blocked PR still gets told why.
func finish(ctx context.Context, opt options, cfg Config, result *gate.Result, diff scan.Diff, verdict string) int {
	repo := os.Getenv("GITHUB_REPOSITORY")
	sha := envOr("SWIFTGATE_SHA", os.Getenv("GITHUB_SHA"))

	body := result.Markdown(gate.ReportContext{
		Repo:     repo,
		SHA:      sha,
		RunURL:   runURL(),
		Passed:   result.Passed(),
		Override: result.Override,
	})
	if verdict != "" && result.Skipped == "" {
		body = strings.Replace(body, "\n---\n\n<sub>", "\n> "+strings.ReplaceAll(verdict, "\n", "\n> ")+"\n\n---\n\n<sub>", 1)
	}

	if opt.jsonOut != "" {
		if err := writeJSON(opt.jsonOut, result); err != nil {
			fmt.Fprintf(os.Stderr, "swiftgate: %v\n", err)
		}
	}
	if summary := os.Getenv("GITHUB_STEP_SUMMARY"); summary != "" {
		appendFile(summary, result.Summary())
	}

	if opt.comment && opt.pr > 0 && repo != "" {
		if token := githubToken(); token != "" {
			client := ghpr.New(token, repo)
			if url, err := client.UpsertComment(ctx, opt.pr, body); err != nil {
				fmt.Fprintf(os.Stderr, "swiftgate: could not post the review comment: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "swiftgate: %s\n", url)
			}
			if opt.inline {
				if err := client.PostInline(ctx, opt.pr, sha, diff, result.Findings); err != nil {
					fmt.Fprintf(os.Stderr, "swiftgate: inline comments skipped: %v\n", err)
				}
			}
		}
	} else {
		fmt.Println(body)
	}

	blockers, warnings, nits := result.Counts()
	switch {
	case result.Skipped != "":
		fmt.Fprintf(os.Stderr, "swiftgate: %s\n", result.Skipped)
		return exitPass
	case result.Override != "":
		fmt.Fprintf(os.Stderr, "swiftgate: %d blocker(s) overridden by label %q\n", blockers, cfg.OverrideLabel)
		return exitPass
	case blockers > 0:
		fmt.Fprintf(os.Stderr, "swiftgate: BLOCKED — %d blocker(s), %d warning(s), %d nit(s)\n", blockers, warnings, nits)
		return exitBlocked
	default:
		fmt.Fprintf(os.Stderr, "swiftgate: passed — %d warning(s), %d nit(s)\n", warnings, nits)
		return exitPass
	}
}

// overrideReason returns the justification a human gave for merging past a blocker.
// The label alone is not enough: without a written reason the gate still blocks, which
// keeps the escape hatch honest and leaves a record in the PR.
var overridePattern = regexp.MustCompile(`(?im)^\s*(?:gate[- ]override|override)\s*:\s*(\S.*)$`)

func overrideReason(ctx context.Context, opt options, cfg Config, result *gate.Result) string {
	if len(result.Blockers()) == 0 || opt.pr == 0 {
		return ""
	}
	pr := prMetadata(ctx, opt)
	if pr == nil || !pr.HasLabel(cfg.OverrideLabel) {
		return ""
	}
	m := overridePattern.FindStringSubmatch(pr.Body)
	if m == nil {
		fmt.Fprintf(os.Stderr, "swiftgate: label %q is set but the PR body has no `Gate override: <reason>` line — still blocked\n", cfg.OverrideLabel)
		return ""
	}
	return strings.TrimSpace(m[1])
}

var prCache *ghpr.PullRequest

func prMetadata(ctx context.Context, opt options) *ghpr.PullRequest {
	if prCache != nil {
		return prCache
	}
	repo, token := os.Getenv("GITHUB_REPOSITORY"), githubToken()
	if opt.pr == 0 || repo == "" || token == "" {
		return nil
	}
	pr, err := ghpr.New(token, repo).PullRequest(ctx, opt.pr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "swiftgate: could not read PR metadata: %v\n", err)
		return nil
	}
	prCache = &pr
	return prCache
}

// normaliseBase turns a branch name into something git can diff against. On a runner
// the base branch exists only as a remote ref.
func normaliseBase(repoDir, base string) string {
	if strings.Contains(base, "/") || looksLikeSHA(base) {
		return base
	}
	for _, candidate := range []string{base, "origin/" + base} {
		if refExists(repoDir, candidate) {
			return candidate
		}
	}
	return base
}

func looksLikeSHA(s string) bool {
	if len(s) < 7 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func refExists(repoDir, ref string) bool {
	return scan.RefExists(repoDir, ref)
}

func flutterCheckout(opt options) string {
	if opt.flutterDir == "" {
		return ""
	}
	p := opt.flutterDir
	if !filepath.IsAbs(p) {
		p = filepath.Join(opt.repoDir, p)
	}
	if _, err := os.Stat(p); err != nil {
		fmt.Fprintf(os.Stderr, "swiftgate: no Flutter spec checkout at %s — reviewing the Swift on its own\n", p)
		return ""
	}
	return p
}

func githubToken() string {
	for _, key := range []string{"SWIFTGATE_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func runURL() string {
	server, repo, id := os.Getenv("GITHUB_SERVER_URL"), os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID")
	if server == "" || repo == "" || id == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/actions/runs/%s", server, repo, id)
}

func writeJSON(path string, r *gate.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(r.Findings)
}

func appendFile(path, content string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(content + "\n")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return fallback
}
