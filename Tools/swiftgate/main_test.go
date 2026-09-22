package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/evidence"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/review"
)

// gitRepo builds a repository with a Swift change on a feature branch off main — the
// smallest pull request the gate would review.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	write("README.md", "base\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "feature/dashboard")
	write("Packages/MindlensKit/Sources/Features/Dashboard/MoodBadge.swift",
		"import SwiftUI\n\nstruct MoodBadge: View {\n    var body: some View { Text(\"ok\") }\n}\n")
	run("add", ".")
	run("commit", "-q", "-m", "a Swift change")
	return dir
}

// stdout captures what a subcommand prints — the report goes there when no PR is set.
func stdout(t *testing.T, f func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	f()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func writeLane(t *testing.T, repo string, lane evidence.Lane, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, review.RunDir, review.LaneFindingsFile(lane)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runDecide(t *testing.T, repo string, args ...string) (int, string) {
	t.Helper()
	return runDecideWith(t, repo, "none.yml", args...)
}

func runDecideWith(t *testing.T, repo, config string, args ...string) (int, string) {
	t.Helper()
	var code int
	out := stdout(t, func() {
		code = decide(t.Context(), append([]string{"--repo", repo, "--config", config}, args...))
	})
	return code, out
}

// granting writes a config that grants the named lanes `blocks: true` and leaves the
// rest advisory, and returns its repository-relative path. The default — no file at
// all — is every lane advisory, which is where a lane starts (ADR 0021).
func granting(t *testing.T, repo string, lanes ...evidence.Lane) string {
	t.Helper()
	blocking := map[evidence.Lane]bool{}
	for _, l := range lanes {
		blocking[l] = true
	}
	body := "lanes:\n"
	for _, l := range evidence.Lanes {
		body += fmt.Sprintf("  %s:\n    blocks: %t\n", l, blocking[l])
	}
	if err := os.WriteFile(filepath.Join(repo, "granting.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return "granting.yml"
}

// The bug this guards, from PR #1: the judge ran with no standard to hold the change
// against — its checkout of the standard had produced an empty tree — and reported a
// clean diff, and the gate reported Passed. The standard is now an exemplar from the
// base branch; here main holds no Swift at all, so there is none.
//
// A lane that could not look must never read as a lane that found nothing. Since ADR
// 0021 that holds in both standings: advisory, the verdict is reported and recorded
// and the merge does not wait on it; granted, it blocks. What it may never be is PASS.
func TestLaneWithoutItsEvidenceCannotEvaluate(t *testing.T) {
	repo := gitRepo(t)
	config := granting(t, repo, evidence.Idiom)
	if code := prepare(t.Context(), []string{"--repo", repo, "--config", "none.yml", "--base", "main", "--head", "feature/dashboard"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}
	writeLane(t, repo, evidence.Idiom, `{"verdict": "PASS", "summary": "Nothing to compare against; the Swift looks fine on its own.", "findings": []}`)

	// Advisory — the default, and where every lane starts.
	code, out := runDecide(t, repo, "--ran", "idiom=true")
	if code != exitPass {
		t.Fatalf("an advisory lane stops nothing, got exit %d:\n%s", code, out)
	}
	for _, want := range []string{"CANNOT_EVALUATE", "exemplar", "advisory"} {
		if !strings.Contains(out, want) {
			t.Errorf("an advisory lane is still reported, and says why; missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(recorded(t, repo), `"verdict":"CANNOT_EVALUATE"`) {
		t.Error("an advisory verdict must reach the metrics record, or there is nothing to earn the grant with")
	}

	// Granted — the same verdict, and now it blocks.
	code, out = runDecideWith(t, repo, config, "--ran", "idiom=true")
	if code != exitBlocked {
		t.Fatalf("a granted idiom lane with no exemplar must block, got exit %d:\n%s", code, out)
	}
	for _, want := range []string{"lane idiom: CANNOT_EVALUATE", "exemplar"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report must say the lane could not evaluate and why; missing %q:\n%s", want, out)
		}
	}
}

// With a resembling file on main the lane has its standard, the brief carries it, and
// the judge's structured verdict stands.
func TestExemplarsAreRetrievedFromTheBaseAndTheVerdictStands(t *testing.T) {
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

	config := granting(t, repo, evidence.Verification)
	if code := prepare(t.Context(), []string{"--repo", repo, "--config", "none.yml", "--base", "main", "--head", "feature/dashboard"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}
	brief, _ := os.ReadFile(filepath.Join(repo, review.RunDir, review.ContextFile))
	if !strings.Contains(string(brief), "SignInView.swift") || !strings.Contains(string(brief), "struct SignInView: View") {
		t.Fatalf("the brief must carry the exemplar and its body:\n%s", brief)
	}
	if !strings.Contains(string(brief), "proof") {
		t.Error("the judge must be told a finding needs a proof; the schema alone is not where it reads its instructions")
	}

	writeLane(t, repo, evidence.Idiom, `{"verdict": "CONCERNS", "summary": "Shaped like SignInView, one nit.", "findings": [
	  {"rule": "review/naming", "severity": "nit", "file": "Packages/MindlensKit/Sources/Features/Dashboard/MoodBadge.swift", "line": 3, "title": "t", "detail": "d", "proof": "SignInView names its type for the screen; this one does not", "fix": "f"}
	]}`)
	// Only the idiom lane has a judge in this run; the other two did not run.
	code, out := runDecideWith(t, repo, config, "--ran", "idiom=true", "--ran", "verification=false", "--ran", "spec=false")
	if !strings.Contains(out, "[IDIOM]") || !strings.Contains(out, "**CONCERNS**") {
		t.Errorf("the idiom lane's own comment should carry its verdict:\n%s", out)
	}
	// The verification lane applies (production Swift changed), it is granted here, and
	// its judge did not run — so the PR is blocked by that lane, not by idiom.
	if code != exitBlocked || !strings.Contains(out, "lane verification: CANNOT_EVALUATE") {
		t.Errorf("a granted lane whose judge did not run blocks, exit %d:\n%s", code, out)
	}
	// A nit is not worth a human reading the diff for.
	if !strings.Contains(out, "Read: pass") {
		t.Errorf("a small change off the risk class with one nit is pass:\n%s", out)
	}
}

// A verdict outside the contract is a contract violation, never a pass.
func TestSchemaViolationIsCannotEvaluate(t *testing.T) {
	repo := gitRepo(t)
	// The verification lane's evidence: a test target for the touched feature.
	tests := filepath.Join(repo, "Packages/MindlensKit/Tests/DashboardTests")
	if err := os.MkdirAll(tests, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tests, "MoodBadgeTests.swift"), []byte("import Testing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := prepare(t.Context(), []string{"--repo", repo, "--config", "none.yml", "--base", "main", "--head", "feature/dashboard"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}
	writeLane(t, repo, evidence.Verification, `{"verdict": "LGTM", "summary": "fine", "findings": []}`)
	code, out := runDecideWith(t, repo, granting(t, repo, evidence.Verification), "--ran", "verification=true")
	if code != exitBlocked || !strings.Contains(out, `"LGTM" is not PASS, CONCERNS or BLOCK`) {
		t.Errorf("want a block naming the bad verdict, exit %d:\n%s", code, out)
	}
}

// recorded is the metrics JSONL decide wrote for the run.
func recorded(t *testing.T, repo string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repo, review.RunDir, review.MetricsFile))
	if err != nil {
		t.Fatalf("no metrics record: %v", err)
	}
	return string(body)
}

// A pull request that changes no Swift still ran the gate, and the records used to be
// silent about it — so the numbers could say how each lane did on the runs it judged
// and never how often there was anything to judge. Coverage is half of whether a lane
// earns its keep (ADR 0021).
func TestAPullRequestWithNoSwiftIsRecordedAsSkipped(t *testing.T) {
	repo := gitRepo(t)
	if code := prepare(t.Context(), []string{"--repo", repo, "--config", "none.yml", "--base", "main", "--head", "main"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}
	code, out := runDecide(t, repo)
	if code != exitPass || !strings.Contains(out, "Skipped") {
		t.Fatalf("a docs-only pull request passes and says it was skipped, exit %d:\n%s", code, out)
	}
	// A pull request can change no Swift and still touch AGENTS.md, a workflow or the
	// gate. "The judges skipped it" is not "read none of it", so the level is still on
	// the comment.
	if !strings.Contains(out, "Read: ") {
		t.Errorf("a skipped review still gets a review level:\n%s", out)
	}
	records := recorded(t, repo)
	for _, lane := range evidence.Lanes {
		if !strings.Contains(records, `"lane":"`+string(lane)+`"`) {
			t.Errorf("every lane needs a record saying it was skipped, so coverage can be counted; %s missing:\n%s", lane, records)
		}
	}
	if !strings.Contains(records, `"skipped":"no Swift changed`) {
		t.Errorf("the record must say why it was skipped:\n%s", records)
	}
}

// The review level is assigned from the diff, and the gate says it before anything
// else in its comment. This one touches Tools/ — the gate itself — so it is full.
func TestTheRiskClassForcesAFullRead(t *testing.T) {
	repo := gitRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	path := filepath.Join(repo, "Tools/swiftgate/main.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "touch the gate")

	if code := prepare(t.Context(), []string{"--repo", repo, "--config", "none.yml", "--base", "main", "--head", "feature/dashboard"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}
	_, out := runDecide(t, repo)
	if !strings.Contains(out, "Read: full") || !strings.Contains(out, "Tools/swiftgate/main.go") {
		t.Errorf("a change to the gate is read in full, and the comment names the file:\n%s", out)
	}
}
