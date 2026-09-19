package gate

import (
	"fmt"
	"strings"
)

// Verdict is what a judge lane says about a pull request. Three of the four are a
// judgement. The fourth is not, and a judge may not return it.
type Verdict string

const (
	Pass     Verdict = "PASS"
	Concerns Verdict = "CONCERNS"
	Block    Verdict = "BLOCK"
	// CannotEvaluate is set by the harness alone: the lane's evidence gate failed, the
	// judge never ran, or what it wrote back was not the contract. It blocks. "I could
	// not look" must never read as "I looked and found little" — on PR #1 the Flutter
	// checkout produced an empty tree, the reviewer said so in prose, and the gate
	// reported Passed.
	CannotEvaluate Verdict = "CANNOT_EVALUATE"
)

// ParseVerdict accepts the judgements a judge may return, and nothing else. A judge
// cannot talk its way into the harness's state, and a typo is a contract violation the
// caller turns into CannotEvaluate rather than a pass.
func ParseVerdict(s string) (Verdict, bool) {
	switch v := Verdict(strings.ToUpper(strings.TrimSpace(s))); v {
	case Pass, Concerns, Block:
		return v, true
	}
	return "", false
}

func (v Verdict) rank() int {
	switch v {
	case Pass:
		return 0
	case Concerns:
		return 1
	case Block:
		return 2
	default:
		return 3
	}
}

// Worse returns the more severe of two verdicts. A judge that lists a blocker and says
// PASS has contradicted itself; the findings win, because they are what a reader acts on.
func Worse(a, b Verdict) Verdict {
	if b.rank() > a.rank() {
		return b
	}
	return a
}

// VerdictFromFindings is the verdict a judge implies by the severities it chose. It is
// cross-checked against the verdict the judge returned, and the worse one stands.
func VerdictFromFindings(fs []Finding) Verdict {
	v := Pass
	for _, f := range fs {
		switch f.Severity {
		case Blocker:
			return Block
		case Warning:
			v = Concerns
		}
	}
	return v
}

// Lane is one judge's result: what it concluded, why, and the findings behind it.
type Lane struct {
	Name    string  `json:"lane"`
	Verdict Verdict `json:"verdict"`
	// Reason is the judge's own summary — or, for CannotEvaluate, what was missing.
	Reason   string    `json:"reason"`
	Findings []Finding `json:"findings"`
	// Skipped, when set, says why the lane had nothing to judge on this pull request.
	// A lane that does not apply is not a lane that failed, and it does not score.
	Skipped string `json:"skipped,omitempty"`
}

// Decision is what the scorer decided, with every condition that made it so. Each
// reason is one row of the blocking table: enumerable, and never a severity a model
// chose on the day.
type Decision struct {
	Blocked bool
	Reasons []string
}

// Score is the deterministic scorer, and the only thing that decides the merge. Lanes
// are advisory; they emit verdicts. It blocks on exactly three conditions: a static
// blocker, a lane that said BLOCK, or a lane that could not evaluate.
func Score(static []Finding, lanes []Lane) Decision {
	var d Decision
	if n := len(blockers(static)); n > 0 {
		d.Reasons = append(d.Reasons, fmt.Sprintf("%d static blocker(s)", n))
	}
	for _, l := range lanes {
		if l.Skipped != "" {
			continue
		}
		switch l.Verdict {
		case Block:
			d.Reasons = append(d.Reasons, fmt.Sprintf("lane %s: BLOCK", l.Name))
		case CannotEvaluate:
			d.Reasons = append(d.Reasons, fmt.Sprintf("lane %s: CANNOT_EVALUATE — %s", l.Name, l.Reason))
		}
	}
	d.Blocked = len(d.Reasons) > 0
	return d
}

func blockers(fs []Finding) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Severity == Blocker {
			out = append(out, f)
		}
	}
	return out
}
