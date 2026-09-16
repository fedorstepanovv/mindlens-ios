// Command swiftgate decides whether a pull request may merge into main.
//
// It runs in two phases, because the two halves of the review have different needs.
//
//	swiftgate prepare   diff the PR, run the deterministic rules, write the brief
//	<the reviewer runs>  Claude Code reads the brief, writes its findings
//	swiftgate decide    merge both halves, comment on the PR, set the exit code
//
// Splitting them is what lets the judgement half run as Claude Code — billed to a
// subscription — while the decision stays here, in a process that can fail a build.
// Blocking findings exit non-zero; wired to a required status check, that exit code is
// the thing that actually stops a merge.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

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

const usage = `swiftgate — the merge gate for this repository.

  swiftgate prepare   Diff the pull request, run the deterministic rules, and write the
                      brief the reviewer reads.
  swiftgate decide    Merge both halves of the review, comment on the pull request, and
                      exit non-zero if anything blocks.
  swiftgate check     Run the deterministic rules and print the report. No reviewer, no
                      API key, no network. This is the one to run locally.

Run a subcommand with -h for its flags.`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(exitError)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	switch os.Args[1] {
	case "prepare":
		os.Exit(prepare(ctx, os.Args[2:]))
	case "decide":
		os.Exit(decide(ctx, os.Args[2:]))
	case "check":
		os.Exit(check(os.Args[2:]))
	case "-h", "--help", "help":
		fmt.Println(usage)
		os.Exit(exitPass)
	default:
		fmt.Fprintf(os.Stderr, "swiftgate: unknown command %q\n\n%s\n", os.Args[1], usage)
		os.Exit(exitError)
	}
}

// --- prepare -----------------------------------------------------------------

func prepare(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("prepare", flag.ExitOnError)
	repoDir := fs.String("repo", ".", "repository to review")
	runDir := fs.String("dir", review.RunDir, "where to write the brief and findings")
	configPath := fs.String("config", ".github/swiftgate.yml", "gate configuration")
	base := fs.String("base", envOr("GITHUB_BASE_REF", "main"), "branch this PR merges into")
	head := fs.String("head", "HEAD", "commit under review")
	pr := fs.Int("pr", 0, "pull request number")
	_ = fs.Parse(args)

	cfg, err := loadConfig(*repoDir, *configPath)
	if err != nil {
		return fail(err)
	}

	diff, err := scan.Collect(*repoDir, normaliseBase(*repoDir, *base), *head, cfg.MaxDiffBytes)
	if err != nil {
		return fail(err)
	}

	dir := filepath.Join(*repoDir, *runDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail(err)
	}

	state := review.State{
		Meta:  review.Meta{Number: *pr, SHA: headSHA()},
		Base:  diff.Base,
		Head:  *head,
		Files: diff.Paths(),
	}

	if len(diff.SwiftFiles()) == 0 {
		state.Skipped = "no Swift changed in this pull request."
		fmt.Fprintf(os.Stderr, "swiftgate: %s Skipping the reviewer.\n", state.Skipped)
		setOutput("review", "false")
		return finishPrepare(dir, state, nil)
	}

	if meta := prMetadata(ctx, *pr); meta != nil {
		state.Meta.Title, state.Meta.Body = meta.Title, meta.Body
	}

	static := scan.Static{RepoDir: *repoDir, Severities: cfg.severities()}.Run(diff)
	fmt.Fprintf(os.Stderr, "swiftgate: %d Swift file(s) changed, %d deterministic finding(s)\n",
		len(diff.SwiftFiles()), len(static))

	flutter := filepath.Join(*repoDir, review.FlutterDir)
	_, statErr := os.Stat(flutter)
	if statErr != nil {
		fmt.Fprintf(os.Stderr, "swiftgate: no Flutter spec at %s — the review will judge the Swift alone\n", review.FlutterDir)
	}

	if err := review.Prepare(dir, diff, state.Meta, static, statErr == nil); err != nil {
		return fail(err)
	}
	state.Reviewed = true
	setOutput("review", "true")
	return finishPrepare(dir, state, static)
}

func finishPrepare(dir string, state review.State, static []gate.Finding) int {
	if err := writeJSON(filepath.Join(dir, review.StaticFile), static); err != nil {
		return fail(err)
	}
	if err := review.SaveState(filepath.Join(dir, review.StateFile), state); err != nil {
		return fail(err)
	}
	return exitPass
}

// --- decide ------------------------------------------------------------------

func decide(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("decide", flag.ExitOnError)
	repoDir := fs.String("repo", ".", "repository under review")
	runDir := fs.String("dir", review.RunDir, "where prepare wrote the brief")
	configPath := fs.String("config", ".github/swiftgate.yml", "gate configuration")
	comment := fs.Bool("comment", false, "post the verdict to the pull request")
	inline := fs.Bool("inline", false, "also attach findings to the lines they are about")
	reviewerRan := fs.Bool("reviewer-ran", true, "whether the reviewer step actually executed")
	reviewer := fs.String("reviewer", "Claude Code", "what to credit in the report footer")
	jsonOut := fs.String("json", "", "write the merged findings here")
	_ = fs.Parse(args)

	cfg, err := loadConfig(*repoDir, *configPath)
	if err != nil {
		return fail(err)
	}

	dir := filepath.Join(*repoDir, *runDir)
	state, err := review.LoadState(filepath.Join(dir, review.StateFile))
	if err != nil {
		return fail(fmt.Errorf("no prepared run in %s — did `swiftgate prepare` run first? %w", *runDir, err))
	}

	result := &gate.Result{Skipped: state.Skipped, Reviewer: *reviewer}

	if state.Skipped == "" {
		var static []gate.Finding
		if err := readJSON(filepath.Join(dir, review.StaticFile), &static); err != nil {
			return fail(err)
		}
		for i := range static {
			static[i].Source = gate.FromStatic // the tag does not round-trip through JSON
		}
		result.Add(static...)

		if state.Reviewed {
			verdict, agentFindings, err := review.Ingest(filepath.Join(dir, review.FindingsFile))
			switch {
			case err != nil && !*reviewerRan:
				result.Add(review.Incomplete("The reviewer step did not run to completion."))
			case err != nil:
				result.Add(review.Incomplete(err.Error()))
			default:
				result.Verdict = verdict
				result.Add(agentFindings...)
				fmt.Fprintf(os.Stderr, "swiftgate: reviewer reported %d finding(s)\n", len(agentFindings))
			}
		}
	}

	result.Normalise()
	result.Override = overrideReason(ctx, cfg, state.Meta.Number, result)

	// Re-diffing here would be wasteful, but inline comments need the added-line map to
	// avoid a 422 on the whole review. Only pay for it when inline is actually on.
	var diff scan.Diff
	if *inline {
		if d, err := scan.Collect(*repoDir, state.Base, state.Head, cfg.MaxDiffBytes); err == nil {
			diff = d
		}
	}

	return report(ctx, result, diff, reportOptions{
		pr:      state.Meta.Number,
		sha:     envOr("SWIFTGATE_SHA", state.Meta.SHA),
		comment: *comment,
		inline:  *inline,
		jsonOut: *jsonOut,
		label:   cfg.OverrideLabel,
	})
}

// --- check -------------------------------------------------------------------

// check is the local path: deterministic rules only, printed to stdout. No run
// directory, no reviewer, nothing to clean up.
func check(args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	repoDir := fs.String("repo", ".", "repository to check")
	configPath := fs.String("config", ".github/swiftgate.yml", "gate configuration")
	base := fs.String("base", envOr("GITHUB_BASE_REF", "main"), "branch to diff against")
	head := fs.String("head", "HEAD", "commit to check")
	_ = fs.Parse(args)

	cfg, err := loadConfig(*repoDir, *configPath)
	if err != nil {
		return fail(err)
	}
	diff, err := scan.Collect(*repoDir, normaliseBase(*repoDir, *base), *head, cfg.MaxDiffBytes)
	if err != nil {
		return fail(err)
	}

	result := &gate.Result{}
	if len(diff.SwiftFiles()) == 0 {
		result.Skipped = "no Swift changed."
	} else {
		result.Add(scan.Static{RepoDir: *repoDir, Severities: cfg.severities()}.Run(diff)...)
	}
	result.Normalise()

	return report(context.Background(), result, scan.Diff{}, reportOptions{label: cfg.OverrideLabel})
}

// --- shared ------------------------------------------------------------------

type reportOptions struct {
	pr      int
	sha     string
	comment bool
	inline  bool
	jsonOut string
	label   string
}

// report writes every output the run produces, then turns the verdict into an exit
// code. Reporting happens before the decision so a blocked PR still gets told why.
func report(ctx context.Context, result *gate.Result, diff scan.Diff, opt reportOptions) int {
	repo := os.Getenv("GITHUB_REPOSITORY")

	body := result.Markdown(gate.ReportContext{
		Repo: repo, SHA: opt.sha, RunURL: runURL(),
		Passed: result.Passed(), Override: result.Override,
	})

	if opt.jsonOut != "" {
		if err := writeJSON(opt.jsonOut, result.Findings); err != nil {
			fmt.Fprintf(os.Stderr, "swiftgate: %v\n", err)
		}
	}
	if summary := os.Getenv("GITHUB_STEP_SUMMARY"); summary != "" {
		appendFile(summary, result.Summary())
	}

	if opt.comment && opt.pr > 0 && repo != "" && githubToken() != "" {
		client := ghpr.New(githubToken(), repo)
		if url, err := client.UpsertComment(ctx, opt.pr, body); err != nil {
			fmt.Fprintf(os.Stderr, "swiftgate: could not post the review comment: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "swiftgate: %s\n", url)
		}
		if opt.inline && len(diff.Files) > 0 {
			if err := client.PostInline(ctx, opt.pr, opt.sha, diff, result.Findings); err != nil {
				fmt.Fprintf(os.Stderr, "swiftgate: inline comments skipped: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "swiftgate: %d blocker(s) overridden by label %q\n", blockers, opt.label)
		return exitPass
	case blockers > 0:
		fmt.Fprintf(os.Stderr, "swiftgate: BLOCKED — %d blocker(s), %d warning(s), %d nit(s)\n", blockers, warnings, nits)
		return exitBlocked
	default:
		fmt.Fprintf(os.Stderr, "swiftgate: passed — %d warning(s), %d nit(s)\n", warnings, nits)
		return exitPass
	}
}

// overridePattern matches the justification a human must write to merge past a blocker.
var overridePattern = regexp.MustCompile(`(?im)^\s*(?:gate[- ]override|override)\s*:\s*(\S.*)$`)

// overrideReason returns that justification. The label alone is not enough: without a
// written reason the gate still blocks, which keeps the escape hatch honest.
func overrideReason(ctx context.Context, cfg Config, pr int, result *gate.Result) string {
	if len(result.Blockers()) == 0 || pr == 0 {
		return ""
	}
	meta := prMetadata(ctx, pr)
	if meta == nil || !meta.HasLabel(cfg.OverrideLabel) {
		return ""
	}
	m := overridePattern.FindStringSubmatch(meta.Body)
	if m == nil {
		fmt.Fprintf(os.Stderr, "swiftgate: label %q is set but the PR body has no `Gate override: <reason>` line — still blocked\n", cfg.OverrideLabel)
		return ""
	}
	return strings.TrimSpace(m[1])
}

var prCache *ghpr.PullRequest

func prMetadata(ctx context.Context, pr int) *ghpr.PullRequest {
	if prCache != nil {
		return prCache
	}
	repo, token := os.Getenv("GITHUB_REPOSITORY"), githubToken()
	if pr == 0 || repo == "" || token == "" {
		return nil
	}
	got, err := ghpr.New(token, repo).PullRequest(ctx, pr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "swiftgate: could not read PR metadata: %v\n", err)
		return nil
	}
	prCache = &got
	return prCache
}

// normaliseBase turns a branch name into something git can diff against. On a runner the
// base branch exists only as a remote ref.
func normaliseBase(repoDir, base string) string {
	if strings.Contains(base, "/") || looksLikeSHA(base) {
		return base
	}
	for _, candidate := range []string{base, "origin/" + base} {
		if scan.RefExists(repoDir, candidate) {
			return candidate
		}
	}
	return base
}

func looksLikeSHA(s string) bool {
	if len(s) < 7 {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool {
		return !strings.ContainsRune("0123456789abcdef", r)
	})
}

func headSHA() string {
	if v := os.Getenv("SWIFTGATE_SHA"); v != "" {
		return v
	}
	return os.Getenv("GITHUB_SHA")
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

// setOutput publishes a step output so the workflow can skip the reviewer when there is
// nothing for it to read.
func setOutput(key, value string) {
	if path := os.Getenv("GITHUB_OUTPUT"); path != "" {
		appendFile(path, fmt.Sprintf("%s=%s", key, value))
	}
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, v)
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

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "swiftgate: %v\n", err)
	return exitError
}
