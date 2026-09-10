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

// ParseSeverity maps a config string onto a Severity, defaulting to Warning so a
// typo in the config downgrades a rule rather than silently blocking every PR.
func ParseSeverity(s string) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "blocker", "error", "block":
		return Blocker
	case "nit", "info":
		return Nit
	case "off", "ignore", "none":
		return Off
	default:
		return Warning
	}
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

// Result is everything one run of the gate produced.
type Result struct {
	Findings []Finding
	// Skipped is set when there was nothing to review, e.g. a docs-only PR.
	Skipped string
	// Usage is filled in by the agent pass. Zero when only static rules ran.
	Usage Usage
	// Override, when non-empty, is the reason a human gave for merging past a
	// blocker. Recorded in the report; it does not hide the findings.
	Override string
}

// Usage is what the review cost.
type Usage struct {
	Model             string
	InputTokens       int64
	OutputTokens      int64
	CacheReadTokens   int64
	CacheWriteTokens  int64
	Turns             int
	EstimatedUSDCents float64
}

// Add merges another pass's findings in.
func (r *Result) Add(fs ...Finding) { r.Findings = append(r.Findings, fs...) }

// Blockers returns the findings that stop the merge.
func (r *Result) Blockers() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity == Blocker {
			out = append(out, f)
		}
	}
	return out
}

// Passed reports whether the merge may proceed.
func (r *Result) Passed() bool {
	return len(r.Blockers()) == 0 || r.Override != ""
}

// Normalise drops disabled findings, removes duplicates, and orders the list so the
// report reads worst-first. Two passes noticing the same line is the common case —
// the static rule wins, because it is the one a human can verify by reading it.
func (r *Result) Normalise() {
	seen := make(map[string]int, len(r.Findings))
	var out []Finding

	for _, f := range r.Findings {
		if f.Severity == Off {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d", f.Rule, f.File, f.Line)
		if i, dup := seen[key]; dup {
			if f.Source == FromStatic && out[i].Source != FromStatic {
				out[i] = f
			}
			continue
		}
		seen[key] = len(out)
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

	r.Findings = out
}

// Counts returns how many of each severity survived.
func (r *Result) Counts() (blockers, warnings, nits int) {
	for _, f := range r.Findings {
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
