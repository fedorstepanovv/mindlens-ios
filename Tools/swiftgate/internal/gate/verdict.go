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
	// judge never ran, or what it wrote back was not the contract. "I could not look"
	// must never read as "I looked and found little" — on PR #1 the Flutter checkout
	// produced an empty tree, the reviewer said so in prose, and the gate reported
	// Passed. It stays visible on every lane; it stops the merge only on a lane the
	// config has granted `blocks: true` (ADR 0021).
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

// Cause says which of the three harness conditions made a lane CannotEvaluate. The
// metrics count them apart: a gate that fires on infrastructure more than on missing
// evidence is a gate that is too strict for its inputs (ADR 0014, "what would change this").
type Cause string

const (
	// CauseEvidence: the lane's inputs were not on disk, so no judge was called.
	CauseEvidence Cause = "evidence"
	// CauseJudge: the judge step did not run to completion — a rate limit, a bad token.
	CauseJudge Cause = "judge"
	// CauseOutput: the judge ran and what it returned was not the contract.
	CauseOutput Cause = "output"
)

// Lane is one judge's result: what it concluded, why, and the findings behind it.
type Lane struct {
	Name    string  `json:"lane"`
	Verdict Verdict `json:"verdict"`
	// Reason is the judge's own summary — or, for CannotEvaluate, what was missing.
	Reason string `json:"reason"`
	// Cause is set with CannotEvaluate and nothing else.
	Cause    Cause     `json:"cause,omitempty"`
	Findings []Finding `json:"findings"`
	// Skipped, when set, says why the lane had nothing to judge on this pull request.
	// A lane that does not apply is not a lane that failed, and it does not score.
	Skipped string `json:"skipped,omitempty"`
	// Unproven is how many findings the judge returned without a proof and so were
	// dropped before anyone read them (ADR 0021). Counted, not hidden: a lane that
	// mostly writes findings it cannot prove is a lane the kill rule is about.
	Unproven int `json:"unproven,omitempty"`
	// Blocks records whether this lane's verdict could stop the merge on this run, so
	// the report can say "advisory" without re-reading the config.
	Blocks bool `json:"blocks"`
}

// Advisory reports whether the lane had something to say that a granted lane would
// have blocked on.
func (l Lane) Advisory() bool {
	return !l.Blocks && (l.Verdict == Block || l.Verdict == CannotEvaluate)
}

// Decision is what the scorer decided, with every condition that made it so. Each
// reason is one row of the blocking table: enumerable, and never a severity a model
// chose on the day.
type Decision struct {
	Blocked bool
	Reasons []string
}

// Score is the deterministic scorer, and the only thing that decides the merge. It
// blocks on exactly three conditions: a static blocker, a lane that said BLOCK, or a
// lane that could not evaluate — and the last two only from a lane the config has
// granted `blocks: true`.
//
// A lane starts advisory and earns blocking on its record (ADR 0021): ≥10 judged pull
// requests, noise ≤30%, at least one finding classified `changed`. Until then its
// verdict is reported on the pull request and written to the metrics line, and the
// merge does not wait on it. The static rules are the deterministic blocking layer
// throughout, and they are not subject to this.
func Score(static []Finding, lanes []Lane) Decision {
	var d Decision
	if n := len(blockers(static)); n > 0 {
		d.Reasons = append(d.Reasons, fmt.Sprintf("%d static blocker(s)", n))
	}
	for _, l := range lanes {
		if l.Skipped != "" || !l.Blocks {
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
