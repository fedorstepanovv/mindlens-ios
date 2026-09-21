package scan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LineRange is a run of lines on the old side of a diff, [Start, Start+Count).
type LineRange struct {
	Start, Count int
}

// FileChange is what happened to one file between two commits, read for the outcome
// classifier: whether it is gone, and which of its old lines a hunk replaced.
type FileChange struct {
	Deleted bool
	// Old are the replaced ranges, in the old file's numbering. A pure insertion has
	// Count 0 and covers no line.
	Old []LineRange
}

// Changed reports whether anything happened to the file at all.
func (c FileChange) Changed() bool { return c.Deleted || len(c.Old) > 0 }

// Covers reports whether the change touched a line the finding pointed at. Line 0 is
// a whole-file finding: any change to the file counts.
func (c FileChange) Covers(line int) bool {
	if c.Deleted {
		return true
	}
	if line <= 0 {
		return len(c.Old) > 0
	}
	for _, r := range c.Old {
		if line >= r.Start && line < r.Start+r.Count {
			return true
		}
	}
	return false
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+\d+(?:,\d+)? @@`)

// Change diffs one file between two commits with no context lines, so the old-side
// ranges are exact. A file absent at `to` is Deleted; a rename counts as deleted too,
// because a finding's path no longer names anything a reader can open.
func Change(repoDir, from, to, path string) (FileChange, error) {
	var c FileChange
	if _, err := git(repoDir, "cat-file", "-e", to+":"+path); err != nil {
		if _, err := git(repoDir, "cat-file", "-e", from+":"+path); err != nil {
			return c, fmt.Errorf("%s exists at neither %s nor %s", path, short(from), short(to))
		}
		c.Deleted = true
		return c, nil
	}
	raw, err := git(repoDir, "diff", "--unified=0", "--no-color", from, to, "--", path)
	if err != nil {
		return c, err
	}
	for _, line := range strings.Split(raw, "\n") {
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		c.Old = append(c.Old, LineRange{Start: start, Count: count})
	}
	return c, nil
}

var allowComment = regexp.MustCompile(`//\s*swiftgate:allow\s+([\w/\-]+)`)

// Waived reports whether the file at `ref` carries a `// swiftgate:allow <rule>` for
// the rule. Anywhere in the file counts: the static pass honours a waiver on the line
// or on the line above, but for an outcome the question is whether a human wrote the
// rule's name down and said why, not where.
func Waived(repoDir, ref, path, rule string) bool {
	body, err := git(repoDir, "show", ref+":"+path)
	if err != nil {
		return false
	}
	for _, m := range allowComment.FindAllStringSubmatch(body, -1) {
		if m[1] == rule {
			return true
		}
	}
	return false
}

func short(ref string) string {
	if len(ref) > 7 {
		return ref[:7]
	}
	return ref
}

// Rev resolves a ref to its full SHA, or returns "" when git cannot.
func Rev(repoDir, ref string) string {
	out, err := git(repoDir, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
