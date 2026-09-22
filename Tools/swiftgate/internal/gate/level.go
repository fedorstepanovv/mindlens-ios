package gate

import (
	"fmt"
	"path"
	"strings"
)

// Level is how much of a pull request a human reads before merging it. A human always
// merges (ADR 0017); the level is advice about reading, never a lock, and it means
// nothing until the deterministic layer is green.
type Level string

const (
	// Pass: nothing. The gate approved on objective criteria.
	LevelPass Level = "pass"
	// Brief: the gate's comment and the pull request body.
	LevelBrief Level = "brief"
	// Full: the diff.
	LevelFull Level = "full"
)

// DefaultRiskPaths is ADR 0017's risk class, and the default for `risk_paths:` in
// .github/swiftgate.yml. Touching any of these forces the full level.
//
// A pattern with no slash matches a file's name at any depth — that is how `Auth*`,
// `Keychain*` and `TokenRefresher*` reach the token code wherever it moves to, which
// is what the ADR asks for. `**` matches any number of path segments.
var DefaultRiskPaths = []string{
	"Packages/*/Sources/Networking/**",
	"Packages/*/Sources/Persistence/**",
	"Config/**",
	".github/**",
	"Tools/**",
	".claude/**",
	"AGENTS.md",
	"CLAUDE.md",
	"Auth*",
	"Keychain*",
	"TokenRefresher*",
	"*.entitlements",
	"Info.plist",
}

// Review is the level and every condition that set it. The reasons are the whole of
// the justification: a level with no reason a reader can check is a number a model
// could have chosen on the day.
type Review struct {
	Level   Level
	Reasons []string
}

// LevelInputs is everything the level is computed from. Nothing here is a judgement:
// paths off the diff, a line count, the findings the lanes returned, and the size cap
// the conventions check enforces.
type LevelInputs struct {
	// Files is every changed path, repository-relative.
	Files []string
	// Changed is the changed-line count, counted the way Tools/check-pr-conventions.sh
	// counts it so the two numbers on one pull request agree.
	Changed int
	// RiskPaths is the configured risk class.
	RiskPaths []string
	// PassMaxChangedLines is the pass level's ceiling.
	PassMaxChangedLines int
	// SizeCap is MINDLENS_PR_SIZE_CAP, and SizeOverridden says a human lifted it. Over
	// the cap on an override is the one case the ADR names by itself.
	SizeCap        int
	SizeOverridden bool
	// Findings is every finding the lanes returned, after the unproven ones were
	// dropped. A lane finding at warning or above forces full — with its proof, which
	// is the only kind that reaches here.
	Findings []Finding
}

// Assess returns the review level and its reasons.
func Assess(in LevelInputs) Review {
	var full, notPass []string

	if risky := matchAny(in.Files, in.RiskPaths); len(risky) > 0 {
		full = append(full, "touches the risk class: "+list(risky))
	}
	if f, ok := firstSerious(in.Findings); ok {
		full = append(full, fmt.Sprintf("a lane finding at %s with a proof — %s (`%s`)", f.Severity, f.Title, f.Location()))
	}
	if in.SizeOverridden && in.SizeCap > 0 && in.Changed > in.SizeCap {
		full = append(full, fmt.Sprintf("%d changed lines, over the cap of %d, merged under a size override", in.Changed, in.SizeCap))
	}
	if len(full) > 0 {
		return Review{Level: LevelFull, Reasons: full}
	}

	if in.PassMaxChangedLines > 0 && in.Changed > in.PassMaxChangedLines {
		notPass = append(notPass, fmt.Sprintf("%d changed lines, over the %d the pass level allows", in.Changed, in.PassMaxChangedLines))
	}
	if adrs := matchAny(in.Files, []string{"docs/decisions/[0-9][0-9][0-9][0-9]-*.md"}); len(adrs) > 0 {
		notPass = append(notPass, "adds a decision: "+list(adrs))
	}
	if deps := matchAny(in.Files, []string{"Package.swift", "Package.resolved"}); len(deps) > 0 {
		notPass = append(notPass, "changes a dependency: "+list(deps))
	}
	if len(notPass) > 0 {
		return Review{Level: LevelBrief, Reasons: notPass}
	}

	return Review{Level: LevelPass, Reasons: []string{
		fmt.Sprintf("%d changed lines, no risk path, no new decision or dependency", in.Changed),
	}}
}

// Label is the label the gate sets on the pull request.
func (r Review) Label() string { return "review:" + string(r.Level) }

// Sentence is the line at the top of the scorer's comment: what to read, and why.
func (r Review) Sentence() string {
	var b strings.Builder
	fmt.Fprintf(&b, "**Read: %s** — %s", r.Level, r.reads())
	for _, reason := range r.Reasons {
		fmt.Fprintf(&b, "\n- %s", reason)
	}
	return b.String()
}

func (r Review) reads() string {
	switch r.Level {
	case LevelPass:
		return "nothing. Merge on the gate's approval (ADR 0017)."
	case LevelFull:
		return "the diff."
	default:
		return "this comment and the pull request body."
	}
}

// firstSerious is the worst lane finding at warning or above, if there is one.
func firstSerious(fs []Finding) (Finding, bool) {
	for _, want := range []Severity{Blocker, Warning} {
		for _, f := range fs {
			if f.Severity == want {
				return f, true
			}
		}
	}
	return Finding{}, false
}

func matchAny(files, patterns []string) []string {
	var out []string
	for _, f := range files {
		for _, p := range patterns {
			if MatchPath(p, f) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// list renders matched paths for a reason line, naming the first two and counting the
// rest — a reason nobody can read is not a reason.
func list(paths []string) string {
	switch {
	case len(paths) == 1:
		return "`" + paths[0] + "`"
	case len(paths) == 2:
		return fmt.Sprintf("`%s`, `%s`", paths[0], paths[1])
	default:
		return fmt.Sprintf("`%s`, `%s` and %d more", paths[0], paths[1], len(paths)-2)
	}
}

// MatchPath reports whether a repository-relative path matches a risk-class pattern.
//
// Three shapes, all of which ADR 0017's list uses: a pattern with no slash matches the
// file's name at any depth; `**` matches any number of segments; anything else is
// path.Match, segment by segment.
func MatchPath(pattern, p string) bool {
	if !strings.Contains(pattern, "/") {
		ok, _ := path.Match(pattern, path.Base(p))
		return ok
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(p, "/"))
}

func matchSegments(pattern, segments []string) bool {
	switch {
	case len(pattern) == 0:
		return len(segments) == 0
	case pattern[0] == "**":
		if len(pattern) == 1 {
			// At least one segment: `Config/**` is the files under Config, and the
			// directory entry itself is not one of them.
			return len(segments) > 0
		}
		for i := range segments {
			if matchSegments(pattern[1:], segments[i:]) {
				return true
			}
		}
		return false
	case len(segments) == 0:
		return false
	default:
		ok, _ := path.Match(pattern[0], segments[0])
		return ok && matchSegments(pattern[1:], segments[1:])
	}
}
