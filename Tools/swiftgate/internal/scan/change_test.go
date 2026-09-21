package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// changeRepo is three commits: a file, an edit to its third line, and a waiver added
// at the bottom. The SHAs come back in that order.
func changeRepo(t *testing.T) (dir string, shas []string) {
	t.Helper()
	dir = t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "A.swift"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string) {
		run("add", ".")
		run("commit", "-q", "-m", msg)
		shas = append(shas, run("rev-parse", "HEAD")[:40])
	}
	run("init", "-q", "-b", "main")
	write("line 1\nline 2\nline 3\nline 4\nline 5\n")
	commit("base")
	write("line 1\nline 2\nline three, edited\nline 4\nline 5\n")
	commit("edit line 3")
	write("line 1\nline 2\nline three, edited\nline 4\nline 5\n// swiftgate:allow design/print-logging — a test double\n")
	commit("waive")
	return dir, shas
}

func TestChangeCoversTheLinesAHunkReplaced(t *testing.T) {
	dir, shas := changeRepo(t)
	c, err := Change(dir, shas[0], shas[1], "A.swift")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Covers(3) {
		t.Errorf("line 3 was edited: %+v", c)
	}
	for _, line := range []int{1, 2, 4, 5} {
		if c.Covers(line) {
			t.Errorf("line %d was not touched: %+v", line, c)
		}
	}
	if !c.Covers(0) {
		t.Error("a whole-file finding is covered by any change")
	}
}

func TestChangeOfAnInsertionCoversNoOldLine(t *testing.T) {
	dir, shas := changeRepo(t)
	c, err := Change(dir, shas[1], shas[2], "A.swift")
	if err != nil {
		t.Fatal(err)
	}
	if c.Covers(3) || c.Covers(5) {
		t.Errorf("appending a line touches no existing one: %+v", c)
	}
	if !c.Changed() || !c.Covers(0) {
		t.Errorf("but the file did change: %+v", c)
	}
}

func TestChangeIsEmptyWhenNothingHappened(t *testing.T) {
	dir, shas := changeRepo(t)
	c, err := Change(dir, shas[1], shas[1], "A.swift")
	if err != nil || c.Changed() {
		t.Errorf("same commit, no change: %+v %v", c, err)
	}
}

func TestChangeReportsADeletedFile(t *testing.T) {
	dir, shas := changeRepo(t)
	cmd := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "rm", "-q", "A.swift")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	cmd = exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "gone")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	c, err := Change(dir, shas[2], "HEAD", "A.swift")
	if err != nil || !c.Deleted || !c.Covers(3) {
		t.Errorf("deleted: %+v %v", c, err)
	}
	if _, err := Change(dir, shas[2], "HEAD", "Nope.swift"); err == nil {
		t.Error("a path that never existed is an error, not a deletion")
	}
}

func TestWaivedReadsTheAllowCommentAtARef(t *testing.T) {
	dir, shas := changeRepo(t)
	if Waived(dir, shas[1], "A.swift", "design/print-logging") {
		t.Error("no waiver before the third commit")
	}
	if !Waived(dir, shas[2], "A.swift", "design/print-logging") {
		t.Error("the waiver is there at the third commit")
	}
	if Waived(dir, shas[2], "A.swift", "design/hardcoded-color") {
		t.Error("a waiver names one rule")
	}
}
