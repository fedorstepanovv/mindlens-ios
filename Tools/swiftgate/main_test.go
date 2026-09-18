package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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

// The bug this guards, from PR #1: the Flutter checkout step succeeded and produced an
// empty tree. `prepare` saw the directory, told the reviewer the spec was there, the
// reviewer said in prose that it could find nothing, and the gate reported Passed.
//
// A lane that could not look must say so, and the scorer must block on it. "I could
// not evaluate" is never "I evaluated and found little".
func TestEmptyFlutterCheckoutCannotEvaluateAndBlocks(t *testing.T) {
	repo := gitRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, review.FlutterDir), 0o755); err != nil {
		t.Fatal(err)
	}

	if code := prepare(t.Context(), []string{"--repo", repo, "--base", "main", "--head", "feature/dashboard"}); code != exitPass {
		t.Fatalf("prepare exited %d", code)
	}

	// The reviewer ran and, with nothing to compare against, reported a clean diff.
	clean := `{"verdict": "I could not find the Dart source; the Swift looks fine on its own.", "findings": []}`
	if err := os.WriteFile(filepath.Join(repo, review.RunDir, review.FindingsFile), []byte(clean), 0o644); err != nil {
		t.Fatal(err)
	}

	var code int
	out := stdout(t, func() {
		code = decide(t.Context(), []string{"--repo", repo, "--reviewer-ran", "true"})
	})

	if code != exitBlocked {
		t.Fatalf("an idiom review with no Dart to compare against must block, got exit %d:\n%s", code, out)
	}
	for _, want := range []string{"CANNOT_EVALUATE", review.FlutterDir} {
		if !strings.Contains(out, want) {
			t.Errorf("the report must say the lane could not evaluate and why; missing %q:\n%s", want, out)
		}
	}
}
