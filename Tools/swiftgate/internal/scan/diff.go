// Package scan turns a pull request into the two things the gate reviews: the set of
// lines this PR actually added, and a readable diff to hand the model.
package scan

import (
	"bufio"
	"fmt"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

// AddedLine is one line this PR introduced.
type AddedLine struct {
	Number int
	Text   string
}

// ChangedFile is one file the PR touched.
type ChangedFile struct {
	Path   string
	Status string // A, M, R...
	Added  []AddedLine
}

// IsSwift reports whether this is Swift source the idiom rules apply to.
func (c ChangedFile) IsSwift() bool { return strings.HasSuffix(c.Path, ".swift") }

// IsTest reports whether the file is test code. Several rules are relaxed there —
// a force-unwrap in a test that should crash loudly is not the same defect.
func (c ChangedFile) IsTest() bool {
	return strings.Contains(c.Path, "/Tests/") ||
		strings.Contains(c.Path, "/TestSupport/") ||
		strings.HasSuffix(c.Path, "Tests.swift")
}

// FeatureTarget returns the feature a file belongs to, or "" when it lives outside
// Packages/MindlensKit/Sources/Features.
func (c ChangedFile) FeatureTarget() string {
	const prefix = "Packages/MindlensKit/Sources/Features/"
	if !strings.HasPrefix(c.Path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(c.Path, prefix)
	if i := strings.Index(rest, "/"); i > 0 {
		return rest[:i]
	}
	return ""
}

// Diff is the whole reviewable change.
type Diff struct {
	Base    string
	Head    string
	Files   []ChangedFile
	Unified string // human-readable diff, capped
}

// SwiftFiles returns just the Swift files, which is what the idiom rules run on.
func (d Diff) SwiftFiles() []ChangedFile {
	var out []ChangedFile
	for _, f := range d.Files {
		if f.IsSwift() {
			out = append(out, f)
		}
	}
	return out
}

// Paths lists every changed path.
func (d Diff) Paths() []string {
	out := make([]string, 0, len(d.Files))
	for _, f := range d.Files {
		out = append(out, f.Path)
	}
	return out
}

// Collect reads the diff between the merge base of base..head and head. Using the
// merge base rather than the base tip matters: without it, every commit that landed
// on main since the branch started shows up as this PR's work.
func Collect(repoDir, baseRef, headRef string, maxDiffBytes int) (Diff, error) {
	base, err := mergeBase(repoDir, baseRef, headRef)
	if err != nil {
		return Diff{}, err
	}

	d := Diff{Base: base, Head: headRef}

	// -U0 gives exact added-line numbers with no context lines to misattribute.
	raw, err := git(repoDir, "diff", "--unified=0", "--find-renames", "--no-color", base, headRef)
	if err != nil {
		return Diff{}, err
	}
	d.Files = parseUnified(raw)

	// A second pass with context, for the model to read.
	unified, err := git(repoDir, "diff", "--unified=3", "--find-renames", "--no-color", base, headRef)
	if err != nil {
		return Diff{}, err
	}
	d.Unified = capString(unified, maxDiffBytes)

	return d, nil
}

func mergeBase(repoDir, baseRef, headRef string) (string, error) {
	out, err := git(repoDir, "merge-base", baseRef, headRef)
	if err != nil {
		// A shallow clone has no common ancestor to find. Fall back to the base ref
		// and say so, rather than silently reviewing the entire history.
		return "", fmt.Errorf("no merge base for %s..%s (is the checkout shallow? fetch-depth: 0): %w", baseRef, headRef, err)
	}
	return strings.TrimSpace(out), nil
}

// parseUnified reads `git diff -U0` output into per-file added lines.
func parseUnified(raw string) []ChangedFile {
	var files []ChangedFile
	var current *ChangedFile
	lineNo := 0

	flush := func() {
		if current != nil {
			files = append(files, *current)
			current = nil
		}
	}

	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	for sc.Scan() {
		line := sc.Text()

		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()

		case strings.HasPrefix(line, "+++ "):
			p := strings.TrimPrefix(line, "+++ ")
			if p == "/dev/null" {
				current = nil // deletion: nothing added to review
				continue
			}
			current = &ChangedFile{Path: strings.TrimPrefix(p, "b/"), Status: "M"}

		case strings.HasPrefix(line, "new file mode"):
			if current != nil {
				current.Status = "A"
			}

		case strings.HasPrefix(line, "@@"):
			lineNo = parseHunkStart(line)

		case strings.HasPrefix(line, "+") && current != nil && lineNo > 0:
			current.Added = append(current.Added, AddedLine{Number: lineNo, Text: line[1:]})
			lineNo++
		}
	}
	flush()

	// `new file mode` precedes `+++`, so patch up statuses by re-reading headers.
	return markAdditions(raw, files)
}

// markAdditions sets Status "A" for files git reported as new. The header ordering in
// unified diffs puts the mode line before the path, so it is simpler to do a second
// pass than to carry state backwards.
func markAdditions(raw string, files []ChangedFile) []ChangedFile {
	added := map[string]bool{}
	var pending bool
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			pending = false
		case strings.HasPrefix(line, "new file mode"):
			pending = true
		case strings.HasPrefix(line, "+++ ") && pending:
			added[strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")] = true
			pending = false
		}
	}
	for i := range files {
		if added[files[i].Path] {
			files[i].Status = "A"
		}
	}
	return files
}

// parseHunkStart pulls the new-file start line out of "@@ -1,2 +34,5 @@".
func parseHunkStart(header string) int {
	i := strings.Index(header, "+")
	if i < 0 {
		return 0
	}
	rest := header[i+1:]
	end := strings.IndexAny(rest, ", ")
	if end < 0 {
		return 0
	}
	n, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0
	}
	return n
}

// Base returns the file's base name, used by rules that judge naming.
func Base(p string) string { return path.Base(p) }

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr)
	}
	return string(out), nil
}

func capString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n\n… diff truncated at %d bytes. Use the read_file tool for anything you need in full.\n", max)
}

// RefExists reports whether git can resolve a ref in this repository. Used to tell a
// local branch name from one that only exists as a remote ref on a CI checkout.
func RefExists(repoDir, ref string) bool {
	_, err := git(repoDir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}
