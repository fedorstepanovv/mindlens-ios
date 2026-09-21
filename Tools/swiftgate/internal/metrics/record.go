// Package metrics records what each judge lane did on each run, and aggregates those
// records into the report that says which lane earns its keep.
//
// One record per lane per run, one line of JSONL each, written by `swiftgate decide`
// and uploaded as a workflow artifact. A second kind of record, written by
// `swiftgate outcomes` when the pull request merges, says what became of each finding.
// `swiftgate metrics` reads both kinds off disk and sums them. Nothing here changes a
// verdict: the records describe the gate, they do not drive it.
package metrics

import (
	"time"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/evidence"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
)

// SchemaVersion is bumped when a field changes meaning. A reader refuses a newer
// version rather than misreading it.
const SchemaVersion = 1

// Kind tells the two record types apart on one JSONL stream.
type Kind string

const (
	KindLane    Kind = "lane"
	KindOutcome Kind = "outcome"
)

// StaticLane is the deterministic pass, recorded as a lane so its rules are counted
// beside the judges'. It has no evidence gate, no judge, no cost.
const StaticLane = "static"

// Run identifies the gate run a record came from.
type Run struct {
	PR     int
	SHA    string
	Branch string
	URL    string
}

// LaneRecord is one lane on one run.
//
// The cost and duration fields are optional and read off the execution file the
// claude-code-action publishes as its execution_file output. Nobody here has seen that
// file's real shape: ReadExecution accepts the shapes it plausibly has and leaves these
// nil when it finds none of them. A nil field means "not reported", never zero. The
// first real run uploads the raw file beside these records so the guess can be checked.
type LaneRecord struct {
	Kind       Kind      `json:"kind"`
	Schema     int       `json:"schema"`
	RecordedAt time.Time `json:"recorded_at"`

	PR     int    `json:"pr"`
	SHA    string `json:"sha"`
	Branch string `json:"branch,omitempty"`
	RunURL string `json:"run_url,omitempty"`

	// Lane is verification, idiom, spec, or StaticLane.
	Lane string `json:"lane"`
	// Verdict is empty when the lane was skipped.
	Verdict gate.Verdict `json:"verdict,omitempty"`
	// Cause is set with CANNOT_EVALUATE: which harness condition fired.
	Cause gate.Cause `json:"cause,omitempty"`
	// Reason is the judge's summary, or what was missing.
	Reason string `json:"reason,omitempty"`
	// Skipped says why the lane had nothing to judge.
	Skipped string `json:"skipped,omitempty"`
	// Evidence is the gate result the verdict was scored from. Nil for the static pass.
	Evidence *EvidenceSummary `json:"evidence,omitempty"`
	Findings []FindingRecord  `json:"findings"`

	// From the execution file, when it said. See the type comment.
	DurationMS *int64   `json:"duration_ms,omitempty"`
	CostUSD    *float64 `json:"cost_usd,omitempty"`
	Turns      *int     `json:"turns,omitempty"`
}

// EvidenceSummary is evidence.Result without the lane name, which the record carries.
type EvidenceSummary struct {
	OK      bool     `json:"ok"`
	Present []string `json:"present,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

// FindingRecord is enough of a finding to count it and to match an outcome to it.
// The detail and the fix stay on the pull request; they are prose, not a metric.
type FindingRecord struct {
	Rule     string        `json:"rule"`
	Severity gate.Severity `json:"severity"`
	File     string        `json:"file"`
	Line     int           `json:"line"`
	Title    string        `json:"title,omitempty"`
}

// Judged reports whether a judge was actually called for this record: a lane that was
// skipped, or could not evaluate for want of evidence or a run, spent nothing.
func (r LaneRecord) Judged() bool {
	if r.Lane == StaticLane || r.Skipped != "" || r.Verdict == "" {
		return false
	}
	return r.Verdict != gate.CannotEvaluate || r.Cause == gate.CauseOutput
}

// FromLane builds the record for one lane from what the scorer had in hand.
func FromLane(run Run, lane gate.Lane, ev evidence.Result, exec Execution) LaneRecord {
	r := LaneRecord{
		Kind: KindLane, Schema: SchemaVersion, RecordedAt: time.Now().UTC(),
		PR: run.PR, SHA: run.SHA, Branch: run.Branch, RunURL: run.URL,
		Lane: lane.Name, Verdict: lane.Verdict, Cause: lane.Cause, Reason: lane.Reason, Skipped: lane.Skipped,
		Evidence:   &EvidenceSummary{OK: ev.OK(), Present: ev.Present, Missing: ev.Missing},
		Findings:   findingRecords(lane.Findings),
		DurationMS: exec.DurationMS, CostUSD: exec.CostUSD, Turns: exec.Turns,
	}
	return r
}

// FromStatic records the deterministic pass. Its verdict is the one its findings
// imply, so the static column reads like the lanes' do.
func FromStatic(run Run, findings []gate.Finding) LaneRecord {
	return LaneRecord{
		Kind: KindLane, Schema: SchemaVersion, RecordedAt: time.Now().UTC(),
		PR: run.PR, SHA: run.SHA, Branch: run.Branch, RunURL: run.URL,
		Lane: StaticLane, Verdict: gate.VerdictFromFindings(findings),
		Findings: findingRecords(findings),
	}
}

func findingRecords(fs []gate.Finding) []FindingRecord {
	out := make([]FindingRecord, 0, len(fs))
	for _, f := range fs {
		out = append(out, FindingRecord{Rule: f.Rule, Severity: f.Severity, File: f.File, Line: f.Line, Title: f.Title})
	}
	return out
}

// Class is what became of a finding by the time the pull request merged.
type Class string

const (
	// Changed: the lines the finding pointed at were edited after it was raised, or
	// the file is gone. The author acted where the finding pointed.
	Changed Class = "changed"
	// Resolved: the last run before the merge no longer reported it, and the lines it
	// pointed at did not change. Either the fix was elsewhere or the judge was not
	// consistent; it is counted apart from the other three so that shows.
	Resolved Class = "resolved"
	// Waived: a `// swiftgate:allow <rule>` now covers it, or the pull request merged
	// under the override label. Read and answered — not noise.
	Waived Class = "waived"
	// Untouched: the lines are as they were and the finding still stood, or nothing
	// ran at the head to say otherwise. This is the noise numerator.
	Untouched Class = "untouched"
)

// Classes is every class, in the order the report shows them.
var Classes = []Class{Changed, Resolved, Waived, Untouched}

// OutcomeRecord is one finding's fate, written at merge time.
type OutcomeRecord struct {
	Kind       Kind      `json:"kind"`
	Schema     int       `json:"schema"`
	RecordedAt time.Time `json:"recorded_at"`

	PR int `json:"pr"`
	// SHA is the head the finding was first raised on; Head is the head that merged.
	SHA  string `json:"sha"`
	Head string `json:"head"`

	Lane    string `json:"lane"`
	Rule    string `json:"rule"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Outcome Class  `json:"outcome"`
	// Reason is one line saying what the classifier saw.
	Reason string `json:"reason"`
}

// Key identifies one finding across runs of the same pull request.
type Key struct {
	Lane, Rule, File string
	Line             int
}

func (f FindingRecord) key(lane string) Key {
	return Key{Lane: lane, Rule: f.Rule, File: f.File, Line: f.Line}
}

func (o OutcomeRecord) key() Key {
	return Key{Lane: o.Lane, Rule: o.Rule, File: o.File, Line: o.Line}
}
