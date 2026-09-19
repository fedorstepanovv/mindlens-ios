package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/evidence"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/metrics"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/review"
)

// decide leaves one record per lane and one for the static pass, whatever it decided,
// with the cost read off the execution file the workflow copied in. Nothing here is a
// real run's shape — see metrics.LaneRecord — so the record has to survive the guess
// being wrong: the last case feeds it garbage and expects "not reported".
func TestDecideRecordsOneMetricsLinePerLane(t *testing.T) {
	repo := gitRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// An exemplar on main, so the idiom lane has its evidence and a verdict to record.
	run("checkout", "-q", "main")
	view := filepath.Join(repo, "Packages/MindlensKit/Sources/Features/Authentication/SignInView.swift")
	if err := os.MkdirAll(filepath.Dir(view), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(view, []byte("import SwiftUI\n\nstruct SignInView: View {\n    var body: some View { Text(\"hi\") }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "a merged view")
	run("checkout", "-q", "feature/dashboard")

	if code := prepare(t.Context(), []string{"--repo", repo, "--config", "none.yml", "--base", "main", "--head", "feature/dashboard"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}
	writeLane(t, repo, evidence.Idiom, `{"verdict": "CONCERNS", "summary": "one nit", "findings": [
	  {"rule": "review/naming", "severity": "nit", "file": "Packages/MindlensKit/Sources/Features/Dashboard/MoodBadge.swift", "line": 3, "title": "t", "detail": "d", "fix": "f"}
	]}`)
	execPath := filepath.Join(repo, review.RunDir, review.ExecutionFile(evidence.Idiom))
	if err := os.WriteFile(execPath, []byte(`[{"type":"system"},{"type":"result","duration_ms":8000,"total_cost_usd":0.75,"num_turns":6}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	runDecide(t, repo, "--ran", "idiom=true", "--ran", "verification=false", "--ran", "spec=false")

	recs, err := metrics.Read(filepath.Join(repo, review.RunDir, review.MetricsFile))
	if err != nil {
		t.Fatal(err)
	}
	byLane := map[string]metrics.LaneRecord{}
	for _, r := range recs.Lanes {
		byLane[r.Lane] = r
	}
	if len(byLane) != 4 {
		t.Fatalf("static plus three lanes, got %d: %+v", len(byLane), recs.Lanes)
	}
	idiom := byLane["idiom"]
	if idiom.Verdict != gate.Concerns || len(idiom.Findings) != 1 || idiom.Findings[0].Rule != "review/naming" {
		t.Errorf("idiom record carries its verdict and findings: %+v", idiom)
	}
	if idiom.CostUSD == nil || *idiom.CostUSD != 0.75 || idiom.DurationMS == nil || *idiom.DurationMS != 8000 || idiom.Turns == nil || *idiom.Turns != 6 {
		t.Errorf("cost, duration and turns come off the execution file: %+v", idiom)
	}
	if idiom.SHA == "" || len(idiom.SHA) < 7 {
		t.Errorf("the SHA is resolved from git when the environment has none: %q", idiom.SHA)
	}
	if idiom.Branch != "feature/dashboard" {
		t.Errorf("branch: %q", idiom.Branch)
	}
	// Verification's evidence is missing here (no test target), so no judge was called
	// and the record says which condition fired, not just that one did.
	verification := byLane["verification"]
	if verification.Verdict != gate.CannotEvaluate || verification.Cause != gate.CauseEvidence || verification.Evidence == nil || verification.Evidence.OK {
		t.Errorf("verification record: %+v", verification)
	}
	if verification.Judged() {
		t.Error("a lane whose evidence was missing did not spend a judge run")
	}
	if spec := byLane["spec"]; spec.Skipped == "" || spec.Verdict != "" {
		t.Errorf("spec is skipped on a branch with no feature file: %+v", spec)
	}
	if static := byLane[metrics.StaticLane]; static.Verdict != gate.Pass || static.Evidence != nil {
		t.Errorf("the static pass: %+v", static)
	}

	// Run decide again with a file the reader cannot parse: the record is still
	// written, and the numbers are absent rather than wrong.
	if err := os.WriteFile(execPath, []byte("<html>rate limited</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDecide(t, repo, "--ran", "idiom=true")
	recs, err = metrics.Read(filepath.Join(repo, review.RunDir, review.MetricsFile))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs.Lanes) != 4 {
		t.Fatalf("a second decide starts the file over, got %d records", len(recs.Lanes))
	}
	for _, r := range recs.Lanes {
		if r.Lane == "idiom" && (r.CostUSD != nil || r.DurationMS != nil) {
			t.Errorf("an unreadable execution file yields no numbers: %+v", r)
		}
	}
}

// A docs-only pull request records nothing: there was no run to measure.
func TestDecideRecordsNothingWhenTheReviewWasSkipped(t *testing.T) {
	repo := gitRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("checkout", "-q", "main")
	run("checkout", "-q", "-b", "docs/only")
	if err := os.WriteFile(filepath.Join(repo, "NOTES.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "docs")
	if code := prepare(t.Context(), []string{"--repo", repo, "--config", "none.yml", "--base", "main", "--head", "docs/only"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}
	runDecide(t, repo)
	if _, err := os.Stat(filepath.Join(repo, review.RunDir, review.MetricsFile)); !os.IsNotExist(err) {
		t.Errorf("no metrics file for a skipped review, got %v", err)
	}
}

// outcomes, against a real history: a finding on the first push, then a second push
// that edits one flagged line, waives another rule, deletes a flagged file, and leaves
// one finding alone. Each becomes the class the record's doc comment promises.
func TestOutcomesClassifiesEachFindingAgainstTheMergedHead(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const badge = "Packages/MindlensKit/Sources/Features/Dashboard/MoodBadge.swift"
	const gone = "Packages/MindlensKit/Sources/Features/Dashboard/Gone.swift"

	run("init", "-q", "-b", "main")
	write("README.md", "base\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "feature/dashboard")
	write(badge, "import SwiftUI\n\nstruct MoodBadgeWidget: View {\n    let model = MoodBadgeViewModel()\n    var body: some View { Text(\"ok\") }\n}\n")
	write(gone, "import Foundation\nlet shared = Thing()\n")
	run("add", ".")
	run("commit", "-q", "-m", "first push")
	first := run("rev-parse", "HEAD")

	// The second push: line 3 renamed, a waiver for the print rule appended, Gone.swift
	// removed. Line 4 (the view model) is left exactly as it was.
	write(badge, "import SwiftUI\n\nstruct MoodBadge: View {\n    let model = MoodBadgeViewModel()\n    var body: some View { Text(\"ok\") }\n}\n// swiftgate:allow design/print-logging — debugging a preview\n")
	run("rm", "-q", gone)
	run("add", ".")
	run("commit", "-q", "-m", "second push")
	head := run("rev-parse", "HEAD")

	// The gate's records for the two runs. On the second, the judge repeated the view
	// model finding and dropped the one about line 5 without that line changing.
	records := filepath.Join(repo, "records", "metrics.jsonl")
	lane := func(sha string, findings ...gate.Finding) metrics.LaneRecord {
		return metrics.FromLane(metrics.Run{PR: 5, SHA: sha}, gate.Lane{Name: "idiom", Verdict: gate.Concerns, Findings: findings}, evidence.Result{Lane: evidence.Idiom}, metrics.Execution{})
	}
	naming := gate.Finding{Rule: "review/naming", Severity: gate.Warning, File: badge, Line: 3}
	mvvm := gate.Finding{Rule: "review/mvvm", Severity: gate.Nit, File: badge, Line: 4}
	body := gate.Finding{Rule: "review/body", Severity: gate.Nit, File: badge, Line: 5}
	singleton := gate.Finding{Rule: "arch/our-singleton", Severity: gate.Blocker, File: gone, Line: 2}
	print := gate.Finding{Rule: "design/print-logging", Severity: gate.Warning, File: badge, Line: 0}
	firstRun := lane(first, naming, mvvm, body, singleton)
	secondRun := lane(head, mvvm)
	secondRun.RecordedAt = firstRun.RecordedAt.Add(1)
	static := metrics.FromStatic(metrics.Run{PR: 5, SHA: first}, []gate.Finding{print})
	other := lane(first, naming)
	other.PR = 6 // another pull request's finding must not be classified here
	if err := metrics.Append(records, firstRun, static, secondRun, other); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(repo, "out", "outcomes.jsonl")
	var code int
	printed := stdout(t, func() {
		code = outcomes([]string{"--repo", repo, "--pr", "5", "--head", head, "--records", records, "--out", out})
	})
	if code != exitPass {
		t.Fatalf("outcomes exited %d:\n%s", code, printed)
	}
	recs, err := metrics.Read(out)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]metrics.Class{}
	for _, o := range recs.Outcomes {
		if o.PR != 5 || o.Head != head || o.SHA != first {
			t.Errorf("every outcome names the PR, the head that merged and the SHA the finding was raised on: %+v", o)
		}
		got[o.Rule] = o.Outcome
	}
	want := map[string]metrics.Class{
		"review/naming":        metrics.Changed,   // line 3 was edited
		"review/mvvm":          metrics.Untouched, // line 4 as it was, still reported at the head
		"review/body":          metrics.Resolved,  // line 5 as it was, not reported at the head
		"arch/our-singleton":   metrics.Changed,   // the file is gone
		"design/print-logging": metrics.Waived,    // the allow comment
	}
	for rule, class := range want {
		if got[rule] != class {
			t.Errorf("%s: want %s, got %s", rule, class, got[rule])
		}
	}
	if len(recs.Outcomes) != len(want) {
		t.Errorf("one outcome per distinct finding on #5, got %d", len(recs.Outcomes))
	}
	for _, s := range []string{"2 changed", "1 resolved", "1 waived", "1 untouched", "**changed**"} {
		if !strings.Contains(printed, s) {
			t.Errorf("the printed table is missing %q:\n%s", s, printed)
		}
	}

	// Under the override label everything left standing reads as waived, not noise.
	printed = stdout(t, func() {
		code = outcomes([]string{"--repo", repo, "--pr", "5", "--head", head, "--records", records, "--out", out, "--override"})
	})
	recs, _ = metrics.Read(out)
	for _, o := range recs.Outcomes {
		if o.Rule == "review/mvvm" && o.Outcome != metrics.Waived {
			t.Errorf("overridden: %+v", o)
		}
	}

	// And the whole loop closes: the metrics report reads both files and shows noise.
	report := stdout(t, func() {
		code = metricsCmd([]string{records, out})
	})
	if code != exitPass || !strings.Contains(report, "| idiom |") || !strings.Contains(report, "untouched") {
		t.Errorf("metrics over records and outcomes:\n%s", report)
	}
}
