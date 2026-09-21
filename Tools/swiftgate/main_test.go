package main

import (
	"bytes"
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
	var code int
	out := stdout(t, func() {
		code = decide(t.Context(), append([]string{"--repo", repo, "--config", "none.yml"}, args...))
	})
	return code, out
}

// The bug this guards, from PR #1: the judge ran with no standard to hold the change
// against — its checkout of the standard had produced an empty tree — and reported a
// clean diff, and the gate reported Passed. The standard is now an exemplar from the base
// branch; here main holds no Swift at all, so there is none, and the lane must say so and
// block.
//
// A lane that could not look must never read as a lane that found nothing.
func TestLaneWithoutItsEvidenceCannotEvaluateAndBlocks(t *testing.T) {
	repo := gitRepo(t)
	if code := prepare(t.Context(), []string{"--repo", repo, "--config", "none.yml", "--base", "main", "--head", "feature/dashboard"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}
	writeLane(t, repo, evidence.Idiom, `{"verdict": "PASS", "summary": "Nothing to compare against; the Swift looks fine on its own.", "findings": []}`)

	code, out := runDecide(t, repo, "--ran", "idiom=true")
	if code != exitBlocked {
		t.Fatalf("an idiom judge with no exemplar must block, got exit %d:\n%s", code, out)
	}
	for _, want := range []string{"CANNOT_EVALUATE", "exemplar"} {
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

	if code := prepare(t.Context(), []string{"--repo", repo, "--config", "none.yml", "--base", "main", "--head", "feature/dashboard"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}
	brief, _ := os.ReadFile(filepath.Join(repo, review.RunDir, review.ContextFile))
	if !strings.Contains(string(brief), "SignInView.swift") || !strings.Contains(string(brief), "struct SignInView: View") {
		t.Fatalf("the brief must carry the exemplar and its body:\n%s", brief)
	}

	writeLane(t, repo, evidence.Idiom, `{"verdict": "CONCERNS", "summary": "Shaped like SignInView, one nit.", "findings": [
	  {"rule": "review/naming", "severity": "nit", "file": "Packages/MindlensKit/Sources/Features/Dashboard/MoodBadge.swift", "line": 3, "title": "t", "detail": "d", "fix": "f"}
	]}`)
	// Only the idiom lane has a judge in this run; the other two did not run.
	code, out := runDecide(t, repo, "--ran", "idiom=true", "--ran", "verification=false", "--ran", "spec=false")
	if !strings.Contains(out, "[IDIOM]") || !strings.Contains(out, "**CONCERNS**") {
		t.Errorf("the idiom lane's own comment should carry its verdict:\n%s", out)
	}
	// The verification lane applies (production Swift changed) and its judge did not run,
	// so the PR is blocked — by that lane, not by idiom.
	if code != exitBlocked || !strings.Contains(out, "lane verification: CANNOT_EVALUATE") {
		t.Errorf("a lane whose judge did not run blocks, exit %d:\n%s", code, out)
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
	code, out := runDecide(t, repo, "--ran", "verification=true")
	if code != exitBlocked || !strings.Contains(out, `"LGTM" is not PASS, CONCERNS or BLOCK`) {
		t.Errorf("want a block naming the bad verdict, exit %d:\n%s", code, out)
	}
}
