// Package gate holds the vocabulary the two review passes share: what a finding is,
// how severe it is, and whether a set of them should stop a merge.
package gate

import (
	"fmt"
	"sort"
	"strings"
)

// Severity decides whether a finding stops the merge. Only Blocker does.
type Severity string

const (
	// Blocker fails the check. Reserved for rules the project states as absolute:
	// a broken module boundary, a Flutter pattern transplanted whole, a missing
	// test the testing contract calls non-negotiable.
	Blocker Severity = "blocker"
	// Warning is reported and does not fail. Something a reviewer should look at.
	Warning Severity = "warning"
	// Nit is reported quietly. Style, naming, small idiom drift.
	Nit Severity = "nit"
)

// ParseSeverity maps a config or judge string onto a Severity, and refuses anything
// outside the vocabulary. It used to default to Warning, which meant a typo in the
// config silently downgraded a blocker; a gate that fails open on a misspelling is not
// a gate, so the caller turns false into an error.
func ParseSeverity(s string) (Severity, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "blocker", "error", "block":
		return Blocker, true
	case "warning", "warn":
		return Warning, true
	case "nit", "info":
		return Nit, true
	case "off", "ignore", "none":
		return Off, true
	}
	return "", false
}

// Off disables a rule entirely.
const Off Severity = "off"

func (s Severity) rank() int {
	switch s {
	case Blocker:
		return 0
	case Warning:
		return 1
	case Nit:
		return 2
	default:
		return 3
	}
}

// Emoji is the marker used wherever findings are rendered for a human.
func (s Severity) Emoji() string {
	switch s {
	case Blocker:
		return "🛑"
	case Warning:
		return "⚠️"
	default:
		return "💬"
	}
}

// Source records which pass produced a finding, so the report can say whether a
// human should second-guess it. Static findings are deterministic; agent findings
// are a model's judgement.
type Source string

const (
	FromStatic Source = "static"
	FromAgent  Source = "agent"
)

// Finding is one thing wrong with the diff.
type Finding struct {
	// Rule is a stable identifier such as "arch/feature-imports-feature". It is what
	// an inline `// swiftgate:allow` comment names and what the config tunes.
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	// File is repo-relative. Line is 1-indexed; 0 means the finding is about the
	// file as a whole rather than a particular line.
	File string `json:"file"`
	Line int    `json:"line"`
	// Title is one sentence naming the problem.
	Title string `json:"title"`
	// Detail says why it is a problem *here*, referring to the actual code.
	Detail string `json:"detail"`
	// Fix says what to write instead. Concrete, not "consider refactoring".
	Fix string `json:"fix"`
	// Doc cites the rule's source of authority, e.g. "docs/PATTERNS.md § ViewModels".
	Doc    string `json:"doc,omitempty"`
	Source Source `json:"-"`
}

// Location renders "path:line", or just the path for file-level findings.
func (f Finding) Location() string {
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	return f.File
}

// Result is everything one run of the gate produced: the deterministic findings, and
// one Lane per judge that ran. The two are kept apart because they are scored apart —
// a static blocker blocks, a lane's findings are the signal behind its verdict.
type Result struct {
	// Findings are the deterministic pass's. Lane findings live on the lane.
	Findings []Finding
	Lanes    []Lane
	// Skipped is set when there was nothing to review, e.g. a docs-only PR.
	Skipped string
	// Reviewer names what produced the judgement half, for the report footer.
	Reviewer string
	// Override, when non-empty, is the reason a human gave for merging past a
	// blocker. Recorded in the report; it does not hide the findings.
	Override string
}

// Add merges static findings in.
func (r *Result) Add(fs ...Finding) { r.Findings = append(r.Findings, fs...) }

// Decision runs the scorer over what this run produced.
func (r *Result) Decision() Decision { return Score(r.Findings, r.Lanes) }

// Blocked reports whether the scorer would stop the merge, before any override.
func (r *Result) Blocked() bool { return r.Decision().Blocked }

// Passed reports whether the merge may proceed.
func (r *Result) Passed() bool { return !r.Blocked() || r.Override != "" }

// All returns every finding, static first, for the outputs that want one list.
func (r *Result) All() []Finding {
	out := append([]Finding(nil), r.Findings...)
	for _, l := range r.Lanes {
		out = append(out, l.Findings...)
	}
	return out
}

// Normalise drops disabled findings, removes duplicates, and orders each list so the
// report reads worst-first. Two passes noticing the same line is the common case —
// the static rule wins, because it is the one a human can verify by reading it.
func (r *Result) Normalise() {
	r.Findings = normalise(r.Findings, nil)
	static := make(map[string]bool, len(r.Findings))
	for _, f := range r.Findings {
		static[key(f)] = true
	}
	for i := range r.Lanes {
		r.Lanes[i].Findings = normalise(r.Lanes[i].Findings, static)
	}
}

func key(f Finding) string { return fmt.Sprintf("%s|%s|%d", f.Rule, f.File, f.Line) }

func normalise(fs []Finding, drop map[string]bool) []Finding {
	seen := make(map[string]bool, len(fs))
	var out []Finding
	for _, f := range fs {
		k := key(f)
		if f.Severity == Off || seen[k] || drop[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Severity.rank() != b.Severity.rank() {
			return a.Severity.rank() < b.Severity.rank()
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return out
}

// Counts returns how many of each severity are in a list.
func Counts(fs []Finding) (blockers, warnings, nits int) {
	for _, f := range fs {
		switch f.Severity {
		case Blocker:
			blockers++
		case Warning:
			warnings++
		case Nit:
			nits++
		}
	}
	return
}
