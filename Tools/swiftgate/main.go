// Command swiftgate decides whether a pull request may merge into main.
//
// It runs in two phases, because the two halves of the review have different needs.
//
//	swiftgate prepare   diff the PR, run the deterministic rules, assert each judge
//	                    lane's evidence is on disk, write the brief
//	<the judges run>    Claude Code reads the brief, writes its findings
//	swiftgate decide    score the static findings and every lane's verdict, comment
//	                    on the PR, set the exit code, record one metrics line per lane
//	swiftgate outcomes  at merge time, say what became of each finding
//	swiftgate metrics   sum the records into the report that says which lane earns its keep
//
// Splitting them is what lets the judgement half run as Claude Code — billed to a
// subscription — while the decision stays here, in a process that can fail a build.
// Lanes are advisory: they emit a verdict, and the deterministic scorer in
// internal/gate alone decides. A lane whose evidence was missing, whose judge did not
// run, or whose output was not the contract is CANNOT_EVALUATE, and that blocks.
package main

import (
	"bytes"
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
	"time"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/evidence"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/ghpr"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/metrics"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/review"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

const (
	exitPass    = 0
	exitBlocked = 1
	exitError   = 2
)

const usage = `swiftgate — the merge gate for this repository.

  swiftgate prepare   Diff the pull request, run the deterministic rules, assert each
                      lane's evidence, and write the brief the judges read.
  swiftgate evidence  Assert one lane's evidence is on disk: --lane verification|idiom|spec.
                      Exit non-zero when it is not. Absence is read off a list, never judged.
  swiftgate decide    Score the static findings and every lane's verdict, comment on the
                      pull request, and exit non-zero if anything blocks. Appends one
                      metrics record per lane to .swiftgate/run/metrics.jsonl.
  swiftgate outcomes  At merge time, classify each finding the gate raised on a pull
                      request as changed, resolved, waived or untouched: --pr N --head <sha>.
  swiftgate metrics   Sum the records under the paths given (default .swiftgate/metrics)
                      into a Markdown report: verdicts, cost, duration and noise per lane,
                      fires per rule, rules that never fired. To fetch the records:
                        gh api "/repos/$R/actions/artifacts?per_page=100" --paginate \
                          --jq '.artifacts[] | select(.name | startswith("swiftgate-metrics-") or startswith("swiftgate-outcomes-")) | .id' \
                          | while read id; do gh api "/repos/$R/actions/artifacts/$id/zip" > "$id.zip"; unzip -oq "$id.zip" -d .swiftgate/metrics/"$id"; done
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
	case "evidence":
		os.Exit(evidenceCmd(os.Args[2:]))
	case "decide":
		os.Exit(decide(ctx, os.Args[2:]))
	case "outcomes":
		os.Exit(outcomes(os.Args[2:]))
	case "metrics":
		os.Exit(metricsCmd(os.Args[2:]))
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
	branch := fs.String("branch", envOr("GITHUB_HEAD_REF", ""), "head branch name; the spec lane reads the feature file it names")
	pr := fs.Int("pr", 0, "pull request number")
	_ = fs.Parse(args)

	cfg, err := loadConfig(*repoDir, *configPath)
	if err != nil {
		return fail(err)
	}

	baseRef := normaliseBase(*repoDir, *base)
	diff, err := scan.Collect(*repoDir, baseRef, *head, cfg.MaxDiffBytes)
	if err != nil {
		return fail(err)
	}

	dir := filepath.Join(*repoDir, *runDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail(err)
	}

	// From the base branch's tip, not the merge base: the newest merged code is the standard.
	exemplars := scan.Exemplars(*repoDir, baseRef, diff.SwiftFiles(), exemplarsPerFile, exemplarBytes)
	headBranch := branchName(*repoDir, *branch, *head)
	state := review.State{
		Meta:     review.Meta{Number: *pr, SHA: headSHA(), Branch: headBranch},
		Base:     diff.Base,
		Head:     *head,
		Files:    diff.Paths(),
		Evidence: evidence.All(evidenceInputs(*repoDir, diff, headBranch, len(exemplars))),
	}
	scoring := map[evidence.Lane]bool{}
	for _, lane := range cfg.lanes() {
		scoring[lane] = true
	}
	for _, lane := range evidence.Lanes {
		ev := state.Evidence[lane]
		fmt.Fprintf(os.Stderr, "swiftgate: evidence · %s\n", ev)
		switch {
		case !ev.Applies() || ev.OK():
		case scoring[lane]:
			fmt.Printf("::error::%s lane cannot evaluate: missing %s\n", lane, strings.Join(ev.Missing, "; "))
		default:
			fmt.Printf("::notice::%s lane is not scoring yet, and could not evaluate this: missing %s\n", lane, strings.Join(ev.Missing, "; "))
		}
		// A lane's step in the workflow runs on this output: it has something to judge,
		// its evidence is there, and the config lets it score.
		setOutput(string(lane), fmt.Sprint(ev.Applies() && ev.OK() && scoring[lane]))
	}
	setOutput("schema", compactJSON(review.Schema))

	if len(diff.SwiftFiles()) == 0 {
		state.Skipped = "no Swift changed in this pull request."
		fmt.Fprintf(os.Stderr, "swiftgate: %s Skipping the judges.\n", state.Skipped)
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
		fmt.Fprintf(os.Stderr, "swiftgate: no Flutter spec at %s — context only, the lanes do not need it\n", review.FlutterDir)
	}

	fmt.Fprintf(os.Stderr, "swiftgate: %d exemplar(s) retrieved from %s\n", len(exemplars), baseRef)
	if err := review.Prepare(dir, diff, state.Meta, static, exemplars, statErr == nil); err != nil {
		return fail(err)
	}
	state.Reviewed = true
	return finishPrepare(dir, state, static)
}

// --- evidence ----------------------------------------------------------------

// evidenceCmd asserts one lane's inputs on their own, for a local check or a workflow
// step that wants a visible red before spending a judge run. It reads the same list
// prepare does; it does not judge.
func evidenceCmd(args []string) int {
	fs := flag.NewFlagSet("evidence", flag.ExitOnError)
	repoDir := fs.String("repo", ".", "repository to check")
	configPath := fs.String("config", ".github/swiftgate.yml", "gate configuration")
	base := fs.String("base", envOr("GITHUB_BASE_REF", "main"), "branch this PR merges into")
	head := fs.String("head", "HEAD", "commit under review")
	branch := fs.String("branch", envOr("GITHUB_HEAD_REF", ""), "head branch name")
	laneName := fs.String("lane", "", "verification, idiom or spec")
	_ = fs.Parse(args)

	lane, ok := evidence.Parse(*laneName)
	if !ok {
		return fail(fmt.Errorf("--lane must be one of %v, got %q", evidence.Lanes, *laneName))
	}
	cfg, err := loadConfig(*repoDir, *configPath)
	if err != nil {
		return fail(err)
	}
	baseRef := normaliseBase(*repoDir, *base)
	diff, err := scan.Collect(*repoDir, baseRef, *head, cfg.MaxDiffBytes)
	if err != nil {
		return fail(err)
	}

	exemplars := scan.Exemplars(*repoDir, baseRef, diff.SwiftFiles(), exemplarsPerFile, exemplarBytes)
	ev := evidence.Check(lane, evidenceInputs(*repoDir, diff, branchName(*repoDir, *branch, *head), len(exemplars)))
	data, _ := json.MarshalIndent(ev, "", "  ")
	fmt.Println(string(data))
	fmt.Fprintf(os.Stderr, "swiftgate: %s\n", ev)
	if ev.Applies() && !ev.OK() {
		return exitBlocked
	}
	return exitPass
}

// How much of the repository an idiom judge is handed as its standard: up to three
// merged files per changed one, each capped so a long view does not crowd out the diff.
const (
	exemplarsPerFile = 3
	exemplarBytes    = 8_000
)

func evidenceInputs(repoDir string, diff scan.Diff, branch string, exemplars int) evidence.Inputs {
	return evidence.Inputs{RepoDir: repoDir, Diff: diff, Branch: branch, Exemplars: exemplars}
}

func compactJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		return s
	}
	return buf.String()
}

// branchName is the head branch: the flag, else the checked-out branch, else the head
// ref when it reads as a branch name. On a runner the checkout is detached, so the flag
// (fed from GITHUB_HEAD_REF) is the one that counts there.
func branchName(repoDir, flag, head string) string {
	if flag != "" {
		return flag
	}
	if b := scan.CurrentBranch(repoDir); b != "" {
		return b
	}
	if head != "HEAD" && !looksLikeSHA(head) {
		return head
	}
	return ""
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
	ran := map[evidence.Lane]bool{}
	fs.Func("ran", "whether a lane's judge step ran to completion, as <lane>=<true|false>; repeatable, default true", func(v string) error {
		name, value, ok := strings.Cut(v, "=")
		lane, known := evidence.Parse(name)
		if !ok || !known {
			return fmt.Errorf("--ran wants <lane>=<true|false>, got %q", v)
		}
		ran[lane] = value == "true"
		return nil
	})
	reviewer := fs.String("reviewer", "Claude Code", "what to credit in the lane footer")
	jsonOut := fs.String("json", "", "write the merged findings here")
	metricsOut := fs.String("metrics", review.MetricsFile, "write one metrics record per lane here, relative to the run dir; empty to skip")
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
	var static []gate.Finding

	if state.Skipped == "" {
		if err := readJSON(filepath.Join(dir, review.StaticFile), &static); err != nil {
			return fail(err)
		}
		for i := range static {
			static[i].Source = gate.FromStatic // the tag does not round-trip through JSON
		}
		result.Add(static...)

		// Only the lanes the config switches on can score; the rest were checked and
		// reported by prepare, so the summary shows what would block once they exist.
		for _, name := range cfg.lanes() {
			ev, known := state.Evidence[name]
			if !known {
				ev = evidence.Result{Lane: name, Missing: []string{"an evidence result from `swiftgate prepare`"}}
			}
			judgeRan, said := ran[name]
			lane := review.Lane(ev, judgeRan || !said, filepath.Join(dir, review.LaneFindingsFile(name)))
			result.Lanes = append(result.Lanes, lane)
			fmt.Fprintf(os.Stderr, "swiftgate: lane %s → %s (%d finding(s))\n", lane.Name, lane.Verdict, len(lane.Findings))
		}
	}

	result.Normalise()
	result.Override = overrideReason(ctx, cfg, state.Meta.Number, result)

	sha := envOr("SWIFTGATE_SHA", state.Meta.SHA)
	if sha == "" {
		sha = scan.Rev(*repoDir, state.Head)
	}
	if *metricsOut != "" && state.Skipped == "" {
		run := metrics.Run{PR: state.Meta.Number, SHA: sha, Branch: state.Meta.Branch, URL: runURL()}
		if err := recordMetrics(dir, filepath.Join(dir, *metricsOut), run, result, state); err != nil {
			fmt.Fprintf(os.Stderr, "swiftgate: metrics not recorded: %v\n", err)
		}
	}

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
		sha:     sha,
		comment: *comment,
		inline:  *inline,
		jsonOut: *jsonOut,
		label:   cfg.OverrideLabel,
	})
}

// recordMetrics writes what this run produced, one line per lane and one for the
// static pass, for the metrics artifact. It runs before the decision and never
// changes it. Cost and duration come off each lane's execution file when the workflow
// copied one in; a file the reader cannot make sense of is logged, and the record
// says "not reported" rather than a number.
func recordMetrics(dir, path string, run metrics.Run, result *gate.Result, state review.State) error {
	_ = os.Remove(path) // a re-run of decide in the same directory starts over
	records := []any{metrics.FromStatic(run, result.Findings)}
	for _, lane := range result.Lanes {
		name, _ := evidence.Parse(lane.Name)
		exec, execPath := metrics.Execution{}, filepath.Join(dir, review.ExecutionFile(name))
		if _, err := os.Stat(execPath); err == nil {
			if exec, err = metrics.ReadExecution(execPath); err != nil {
				fmt.Fprintf(os.Stderr, "swiftgate: %s execution file: %v — cost and duration not reported\n", lane.Name, err)
			}
		}
		records = append(records, metrics.FromLane(run, lane, state.Evidence[name], exec))
	}
	if err := metrics.Append(path, records...); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "swiftgate: %d metrics record(s) → %s\n", len(records), path)
	return nil
}

// --- outcomes ----------------------------------------------------------------

// outcomes runs when a pull request merges. For every finding the gate raised on it,
// it reads off git and the records what became of the finding, and writes one outcome
// record each. untouched ÷ fires is the noise rate the metrics report shows.
func outcomes(args []string) int {
	fs := flag.NewFlagSet("outcomes", flag.ExitOnError)
	repoDir := fs.String("repo", ".", "repository the pull request merged into")
	pr := fs.Int("pr", 0, "pull request number")
	head := fs.String("head", "", "the head commit that merged")
	override := fs.Bool("override", false, "the pull request merged under the override label")
	out := fs.String("out", filepath.Join(review.RunDir, "outcomes.jsonl"), "write the outcome records here")
	var records []string
	fs.Func("records", "a metrics .jsonl or a directory of them; repeatable", func(v string) error {
		records = append(records, v)
		return nil
	})
	_ = fs.Parse(args)
	if *pr == 0 || *head == "" {
		return fail(fmt.Errorf("outcomes needs --pr and --head"))
	}
	if len(records) == 0 {
		records = []string{".swiftgate/metrics"}
	}
	for i, r := range records {
		if !filepath.IsAbs(r) {
			records[i] = filepath.Join(*repoDir, r)
		}
	}
	headSHA := scan.Rev(*repoDir, *head)
	if headSHA == "" {
		return fail(fmt.Errorf("--head %q is not a commit in %s", *head, *repoDir))
	}

	recs, err := metrics.Read(records...)
	if err != nil {
		return fail(err)
	}
	raised := metrics.FirstRaised(recs.Lanes, *pr)
	if len(raised) == 0 {
		fmt.Fprintf(os.Stderr, "swiftgate: no findings recorded for #%d — nothing to classify\n", *pr)
		return exitPass
	}
	_, headRun := metrics.HeadRun(recs.Lanes, *pr, headSHA)
	if !headRun {
		_, headRun = metrics.HeadRun(recs.Lanes, *pr, *head)
	}
	reported := metrics.ReportedAt(recs.Lanes, *pr, headSHA)
	for k, v := range metrics.ReportedAt(recs.Lanes, *pr, *head) {
		reported[k] = v
	}

	var written []metrics.OutcomeRecord
	skipped := 0
	for _, r := range raised {
		ev := metrics.Evidence{Overridden: *override, HeadRun: headRun, StillReported: reported[r.Key]}
		change, err := scan.Change(*repoDir, r.SHA, headSHA, r.File)
		if err != nil {
			fmt.Fprintf(os.Stderr, "swiftgate: %s at %s:%d could not be classified: %v\n", r.Rule, r.File, r.Line, err)
			skipped++
			continue
		}
		ev.Deleted, ev.Touched = change.Deleted, change.Covers(r.Line)
		if !change.Deleted {
			ev.Waived = scan.Waived(*repoDir, headSHA, r.File, r.Rule)
		}
		class, reason := metrics.Classify(ev)
		written = append(written, metrics.OutcomeRecord{
			Kind: metrics.KindOutcome, Schema: metrics.SchemaVersion, RecordedAt: time.Now().UTC(),
			PR: *pr, SHA: r.SHA, Head: headSHA,
			Lane: r.Lane, Rule: r.Rule, File: r.File, Line: r.Line,
			Outcome: class, Reason: reason,
		})
	}

	outPath := *out
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(*repoDir, outPath)
	}
	_ = os.Remove(outPath)
	all := make([]any, 0, len(written))
	for _, w := range written {
		all = append(all, w)
	}
	if err := metrics.Append(outPath, all...); err != nil {
		return fail(err)
	}

	counts := map[metrics.Class]int{}
	fmt.Printf("## Outcomes for #%d at %s\n\n| Lane | Rule | Location | Outcome | Why |\n|---|---|---|---|---|\n", *pr, shortSHA(headSHA))
	for _, w := range written {
		counts[w.Outcome]++
		loc := w.File
		if w.Line > 0 {
			loc = fmt.Sprintf("%s:%d", w.File, w.Line)
		}
		fmt.Printf("| %s | `%s` | %s | **%s** | %s |\n", w.Lane, w.Rule, loc, w.Outcome, w.Reason)
	}
	var tail []string
	for _, c := range metrics.Classes {
		tail = append(tail, fmt.Sprintf("%d %s", counts[c], c))
	}
	fmt.Printf("\n%s", strings.Join(tail, " · "))
	if skipped > 0 {
		fmt.Printf(" · %d could not be classified (see the log)", skipped)
	}
	fmt.Printf("\n")
	if summary := os.Getenv("GITHUB_STEP_SUMMARY"); summary != "" {
		appendFile(summary, fmt.Sprintf("Outcomes for #%d: %s", *pr, strings.Join(tail, " · ")))
	}
	fmt.Fprintf(os.Stderr, "swiftgate: %d outcome record(s) → %s\n", len(written), outPath)
	return exitPass
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// --- metrics -----------------------------------------------------------------

// metricsCmd sums every record under the paths given and prints the report.
func metricsCmd(args []string) int {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)
	jsonOut := fs.String("json", "", "also write the summary as JSON here")
	_ = fs.Parse(args)
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{".swiftgate/metrics"}
	}
	recs, err := metrics.Read(paths...)
	if err != nil {
		return fail(err)
	}
	summary := metrics.Summarise(recs, scan.RuleIDs())
	fmt.Print(summary.Markdown())
	if *jsonOut != "" {
		if err := writeJSON(*jsonOut, summary); err != nil {
			return fail(err)
		}
	}
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		appendFile(path, summary.Markdown())
	}
	return exitPass
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

	ctxR := gate.ReportContext{Repo: repo, SHA: opt.sha, RunURL: runURL(), Override: result.Override}

	// One sticky comment for the scorer, one per lane that applies, each stamped with
	// the head SHA so a verdict about code that is gone reads as stale.
	comments := []struct{ marker, body string }{{gate.ReportMarker, result.Markdown(ctxR)}}
	for _, l := range result.Lanes {
		if l.Skipped == "" {
			comments = append(comments, struct{ marker, body string }{gate.LaneMarker(l.Name), l.Markdown(ctxR, result.Reviewer)})
		}
	}

	if opt.jsonOut != "" {
		if err := writeJSON(opt.jsonOut, result.All()); err != nil {
			fmt.Fprintf(os.Stderr, "swiftgate: %v\n", err)
		}
	}
	if summary := os.Getenv("GITHUB_STEP_SUMMARY"); summary != "" {
		appendFile(summary, result.Summary())
	}

	if opt.comment && opt.pr > 0 && repo != "" && githubToken() != "" {
		client := ghpr.New(githubToken(), repo)
		for _, c := range comments {
			if url, err := client.UpsertComment(ctx, opt.pr, c.marker, c.body); err != nil {
				fmt.Fprintf(os.Stderr, "swiftgate: could not post %s: %v\n", c.marker, err)
			} else {
				fmt.Fprintf(os.Stderr, "swiftgate: %s\n", url)
			}
		}
		if opt.inline && len(diff.Files) > 0 {
			if err := client.PostInline(ctx, opt.pr, opt.sha, diff, result.All()); err != nil {
				fmt.Fprintf(os.Stderr, "swiftgate: inline comments skipped: %v\n", err)
			}
		}
	} else {
		for _, c := range comments {
			fmt.Println(c.body)
		}
	}

	decision := result.Decision()
	switch {
	case result.Skipped != "":
		fmt.Fprintf(os.Stderr, "swiftgate: %s\n", result.Skipped)
		return exitPass
	case decision.Blocked && result.Override != "":
		fmt.Fprintf(os.Stderr, "swiftgate: blocked on %s — overridden by label %q\n", strings.Join(decision.Reasons, "; "), opt.label)
		return exitPass
	case decision.Blocked:
		fmt.Fprintf(os.Stderr, "swiftgate: BLOCKED — %s\n", strings.Join(decision.Reasons, "; "))
		return exitBlocked
	default:
		fmt.Fprintf(os.Stderr, "swiftgate: ready\n")
		return exitPass
	}
}

// overridePattern matches the justification a human must write to merge past a blocker.
var overridePattern = regexp.MustCompile(`(?im)^\s*(?:gate[- ]override|override)\s*:\s*(\S.*)$`)

// overrideReason returns that justification. The label alone is not enough: without a
// written reason the gate still blocks, which keeps the escape hatch honest.
func overrideReason(ctx context.Context, cfg Config, pr int, result *gate.Result) string {
	if !result.Blocked() || pr == 0 {
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
