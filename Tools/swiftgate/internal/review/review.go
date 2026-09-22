// Package review is the seam between the gate and the reviewer.
//
// The reviewer is Claude Code, running as a step in the same job. This package writes
// the brief it reads and ingests the findings it writes back. It deliberately does not
// call a model: the gate holds the decision, the reviewer holds the judgement, and
// keeping those in separate processes is what lets the review bill against a Claude
// subscription instead of the API.
package review

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/evidence"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

// RunDir is the working directory the two halves share. It is git-ignored: the brief
// and the findings are build artefacts, not source.
const RunDir = ".swiftgate/run"

// Paths inside RunDir. The lane skills hardcode ContextFile, so its name is a contract
// with .claude/skills/lane-*/SKILL.md — change both together.
const (
	ContextFile = "context.md"
	StaticFile  = "static-findings.json"
	StateFile   = "state.json"
)

// Schema is the contract every lane's judge returns, as the JSON Schema the Claude Code
// action validates its structured output against. Ingest is the other half; the two
// name the same fields. A verdict outside the enum never reaches Ingest — and if one
// did, Ingest refuses it.
const Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["verdict", "summary", "findings"],
  "properties": {
    "verdict": {"type": "string", "enum": ["PASS", "CONCERNS", "BLOCK"]},
    "summary": {"type": "string", "description": "One paragraph. Say it plainly."},
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["rule", "severity", "file", "title", "detail", "proof", "fix"],
        "properties": {
          "rule": {"type": "string", "description": "stable kebab-case category/name"},
          "severity": {"type": "string", "enum": ["blocker", "warning", "nit"]},
          "file": {"type": "string", "description": "repository-relative Swift file"},
          "line": {"type": "integer", "description": "1-indexed; omit for a whole-file finding"},
          "title": {"type": "string"},
          "detail": {"type": "string"},
          "proof": {"type": "string", "minLength": 1, "description": "Why this is true and not a guess: the concrete input or state and the line where it fails, or the exemplar it contradicts. A finding without one is dropped and never reported."},
          "fix": {"type": "string"},
          "doc": {"type": "string", "description": "the project document that says so"}
        }
      }
    }
  }
}`

// Meta is what the gate knows about the pull request itself.
type Meta struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	SHA    string `json:"sha"`
	// Branch is the head branch. The spec lane reads the feature file it names.
	Branch string `json:"branch,omitempty"`
}

// State is what `prepare` hands to `decide`, so the second half does not have to
// recompute the diff or re-read the pull request.
type State struct {
	Meta  Meta     `json:"meta"`
	Base  string   `json:"base"`
	Head  string   `json:"head"`
	Files []string `json:"files"`
	// Changed is the changed-line count the review level is assigned from, counted the
	// way Tools/check-pr-conventions.sh counts it.
	Changed int    `json:"changed"`
	Skipped string `json:"skipped,omitempty"`
	// Reviewed records whether a reviewer was expected to run at all.
	Reviewed bool `json:"reviewed"`
	// Evidence is every lane's gate result, read off disk by `prepare` before any
	// judge runs. `decide` scores from it; it never re-derives it.
	Evidence map[evidence.Lane]evidence.Result `json:"evidence,omitempty"`
}

// LaneFindingsFile is where the workflow puts a lane's structured output for decide.
func LaneFindingsFile(lane evidence.Lane) string { return "lane-" + string(lane) + ".json" }

// ExecutionFile is where the workflow copies the Claude Code action's execution log for
// a lane, when the step produced one. decide reads cost and duration off it for the
// metrics record; it is optional, and its absence changes no verdict.
func ExecutionFile(lane evidence.Lane) string { return "execution-" + string(lane) + ".json" }

// MetricsFile is the JSONL decide appends one record per lane to, for the metrics
// artifact. Its shape is internal/metrics.LaneRecord.
const MetricsFile = "metrics.jsonl"

// Prepare writes the brief the reviewer reads. Returns false when there is nothing to
// review, so the caller can skip the reviewer entirely rather than spend a run on it.
func Prepare(dir string, d scan.Diff, meta Meta, static []gate.Finding, exemplars []scan.Exemplar) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ContextFile),
		[]byte(brief(d, meta, static, exemplars)), 0o644)
}

// brief renders the pull request for a reader who has the repository on disk but has
// not seen the change.
func brief(d scan.Diff, meta Meta, static []gate.Finding, exemplars []scan.Exemplar) string {
	var b strings.Builder

	b.WriteString("# The pull request under review\n\n")
	if meta.Title != "" {
		fmt.Fprintf(&b, "**%s**\n\n", meta.Title)
	}
	if body := strings.TrimSpace(meta.Body); body != "" {
		fmt.Fprintf(&b, "%s\n\n", body)
	}
	fmt.Fprintf(&b, "Merging into `%s`. %d changed file(s).\n\n", shortSHA(d.Base), len(d.Files))
	if meta.Branch != "" {
		fmt.Fprintf(&b, "Branch `%s`.", meta.Branch)
		if name, ok := strings.CutPrefix(meta.Branch, "feature/"); ok && name != "" {
			fmt.Fprintf(&b, " Its feature file, if this is a product feature, is `docs/features/%s.md`; the spec lane judges against the open step there.", name)
		}
		b.WriteString("\n\n")
	}

	b.WriteString("## Changed files\n\n")
	for _, f := range d.Files {
		status := "modified"
		if f.Status == "A" {
			status = "new"
		}
		fmt.Fprintf(&b, "- `%s` (%s, +%d)\n", f.Path, status, len(f.Added))
	}

	b.WriteString("\n## Exemplars — the Swift we write here\n\n")
	if len(exemplars) == 0 {
		b.WriteString("None were found for these files.\n")
	} else {
		b.WriteString("Merged files that resemble the changed ones in kind, name and neighbourhood. " +
			"They are the standard: judge whether the change is shaped like them, not whether it resembles Dart.\n")
	}
	for _, e := range exemplars {
		fmt.Fprintf(&b, "\n### %s\n\n```swift\n%s\n```\n", e.Describe(), strings.TrimRight(e.Body, "\n"))
	}

	b.WriteString("\n## Already reported — do not repeat these\n\n")
	if len(static) == 0 {
		b.WriteString("The deterministic pass found nothing.\n")
	}
	for _, f := range static {
		fmt.Fprintf(&b, "- `%s` at %s — %s\n", f.Rule, f.Location(), f.Title)
	}

	fmt.Fprintf(&b, "\n## Diff\n\n```diff\n%s\n```\n", d.Unified)
	b.WriteString("\nEvery finding needs a `proof`: the concrete input or state and the line where it fails, " +
		"or the exemplar it contradicts. A finding without one is dropped before anyone reads it, so it is " +
		"work you did for nothing — leave it out instead.\n")
	b.WriteString("\nReturn your verdict as the structured output and stop. Write no file, post no comment.\n")

	return b.String()
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// Report is the shape a judge returns: Schema, decoded.
type Report struct {
	Verdict  string       `json:"verdict"`
	Summary  string       `json:"summary"`
	Findings []rawFinding `json:"findings"`
}

type rawFinding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Proof    string `json:"proof"`
	Fix      string `json:"fix"`
	Doc      string `json:"doc"`
}

// Judgement is what one judge returned, with the contract applied to it.
type Judgement struct {
	Verdict  gate.Verdict
	Summary  string
	Findings []gate.Finding
	// Unproven is how many findings were dropped for want of a proof. Counted rather
	// than discarded quietly: Anthropic's substantive-comment rate went from 16% to
	// 54% by requiring a proof, and the count is how this repository sees whether a
	// lane is mostly writing things it cannot support (ADR 0021).
	Unproven int
}

// Ingest reads a judge's structured output.
//
// A missing file is an error, not a clean review: if the judge did not run, the gate has
// no basis for a verdict. So is a verdict outside the contract — the schema should have
// stopped it, and a harness that trusted the schema alone would pass a lane on a typo.
// The same argument applies to the proof: the schema requires it, and this drops any
// finding that arrives without one rather than trusting that it could not.
func Ingest(path string) (Judgement, error) {
	var j Judgement
	data, err := os.ReadFile(path)
	if err != nil {
		return j, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return j, fmt.Errorf("%s is empty — the judge produced no structured output", filepath.Base(path))
	}

	// A fenced block is tolerated: the contract is the JSON, not the bytes around it.
	var report Report
	if err := json.Unmarshal(unfence(data), &report); err != nil {
		return j, fmt.Errorf("the judge's output is not valid JSON: %w", err)
	}
	verdict, ok := gate.ParseVerdict(report.Verdict)
	if !ok {
		return j, fmt.Errorf("the judge's verdict %q is not PASS, CONCERNS or BLOCK", report.Verdict)
	}

	for _, f := range report.Findings {
		path := strings.TrimPrefix(strings.TrimSpace(f.File), "./")
		// The gate reviews Swift. A finding pinned to nothing has no line a reviewer
		// can act on.
		if path == "" {
			continue
		}
		severity, ok := gate.ParseSeverity(f.Severity)
		if !ok {
			return Judgement{}, fmt.Errorf("the judge's severity %q is not blocker, warning or nit", f.Severity)
		}
		proof := strings.TrimSpace(f.Proof)
		if proof == "" {
			j.Unproven++
			continue
		}
		j.Findings = append(j.Findings, gate.Finding{
			Rule:     NormaliseRule(f.Rule),
			Severity: severity,
			File:     path,
			Line:     f.Line,
			Title:    strings.TrimSpace(f.Title),
			Detail:   strings.TrimSpace(f.Detail),
			Proof:    proof,
			Fix:      strings.TrimSpace(f.Fix),
			Doc:      strings.TrimSpace(f.Doc),
			Source:   gate.FromAgent,
		})
	}
	j.Verdict, j.Summary = verdict, strings.TrimSpace(report.Summary)
	return j, nil
}

var fence = regexp.MustCompile("(?s)^\\s*```(?:json)?\\s*(.*?)\\s*```\\s*$")

func unfence(data []byte) []byte {
	if m := fence.FindSubmatch(data); m != nil {
		return m[1]
	}
	return data
}

var ruleShape = regexp.MustCompile(`[^a-z0-9/\-]+`)

// NormaliseRule turns whatever the reviewer wrote into an id the config can address.
func NormaliseRule(s string) string {
	s = ruleShape.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	s = strings.Trim(s, "-/")
	if s == "" {
		return "review/unspecified"
	}
	if !strings.Contains(s, "/") {
		s = "review/" + s
	}
	return s
}

// Lane turns what the harness knows about one judge into its result. The order of the
// checks is the contract: no evidence, no run, and no valid file each become
// CANNOT_EVALUATE before anything the judge said is read. Only after all three hold
// does the judge's own output stand.
//
// blocks says whether this lane's verdict can stop the merge on this run. It changes
// nothing here — every lane is evaluated and reported the same way — and is carried so
// the scorer and the report can say which verdicts were advisory.
func Lane(ev evidence.Result, judgeRan, blocks bool, findingsPath string) gate.Lane {
	lane := gate.Lane{Name: string(ev.Lane), Blocks: blocks}
	cannot := func(cause gate.Cause, reason string) gate.Lane {
		lane.Verdict, lane.Cause, lane.Reason = gate.CannotEvaluate, cause, reason
		return lane
	}
	switch {
	case !ev.Applies():
		lane.Skipped = ev.Skipped
		return lane
	case !ev.OK():
		return cannot(gate.CauseEvidence, "missing "+strings.Join(ev.Missing, "; "))
	case !judgeRan:
		return cannot(gate.CauseJudge, "the judge step did not run to completion. Re-run the failed job; if it keeps "+
			"failing, check the CLAUDE_CODE_OAUTH_TOKEN secret and the subscription's rate limit.")
	}
	j, err := Ingest(findingsPath)
	if err != nil {
		return cannot(gate.CauseOutput, "the judge returned nothing the harness can read: "+err.Error())
	}
	// The judge's own verdict stands even when every finding behind it was dropped for
	// want of a proof: that reads as "BLOCK, 0 findings, 2 unproven", which is the
	// truth about the judge and the number the kill rule is measured on. Quietly
	// downgrading it to PASS would hide exactly the lane worth cutting.
	lane.Verdict = gate.Worse(j.Verdict, gate.VerdictFromFindings(j.Findings))
	lane.Reason, lane.Findings, lane.Unproven = j.Summary, j.Findings, j.Unproven
	return lane
}

// SaveState and LoadState carry what `prepare` learned across to `decide`.
func SaveState(path string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadState(path string) (State, error) {
	var s State
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(data, &s)
}
