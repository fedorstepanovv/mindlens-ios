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

// FlutterDir is where CI checks the Flutter app out, relative to the repository. It is
// context, not evidence: a lane needs nothing from it to run.
const FlutterDir = ".swiftgate/flutter"

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
        "required": ["rule", "severity", "file", "title", "detail", "fix"],
        "properties": {
          "rule": {"type": "string", "description": "stable kebab-case category/name"},
          "severity": {"type": "string", "enum": ["blocker", "warning", "nit"]},
          "file": {"type": "string", "description": "repository-relative Swift file"},
          "line": {"type": "integer", "description": "1-indexed; omit for a whole-file finding"},
          "title": {"type": "string"},
          "detail": {"type": "string"},
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
	Meta    Meta     `json:"meta"`
	Base    string   `json:"base"`
	Head    string   `json:"head"`
	Files   []string `json:"files"`
	Skipped string   `json:"skipped,omitempty"`
	// Reviewed records whether a reviewer was expected to run at all.
	Reviewed bool `json:"reviewed"`
	// Evidence is every lane's gate result, read off disk by `prepare` before any
	// judge runs. `decide` scores from it; it never re-derives it.
	Evidence map[evidence.Lane]evidence.Result `json:"evidence,omitempty"`
}

// LaneFindingsFile is where the workflow puts a lane's structured output for decide.
func LaneFindingsFile(lane evidence.Lane) string { return "lane-" + string(lane) + ".json" }

// Prepare writes the brief the reviewer reads. Returns false when there is nothing to
// review, so the caller can skip the reviewer entirely rather than spend a run on it.
func Prepare(dir string, d scan.Diff, meta Meta, static []gate.Finding, exemplars []scan.Exemplar, flutterAvailable bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ContextFile),
		[]byte(brief(d, meta, static, exemplars, flutterAvailable)), 0o644)
}

// brief renders the pull request for a reader who has the repository on disk but has
// not seen the change.
func brief(d scan.Diff, meta Meta, static []gate.Finding, exemplars []scan.Exemplar, flutterAvailable bool) string {
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
			fmt.Fprintf(&b, " Its feature file is `docs/features/%s.md`; the spec lane judges against the open step there.", name)
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

	b.WriteString("\n## The Flutter spec\n\n")
	if flutterAvailable {
		fmt.Fprintf(&b, "The Flutter app is checked out at `%s`; its Dart sources are under `%s/lib`. "+
			"It is a product spec — what a screen does, how a flow sequences, what the copy says. "+
			"Open it only to check that, never as a model of how to build it.\n", FlutterDir, FlutterDir)
	} else {
		b.WriteString("Not checked out in this run, and not needed: the exemplars are the standard.\n")
	}

	b.WriteString("\n## Already reported — do not repeat these\n\n")
	if len(static) == 0 {
		b.WriteString("The deterministic pass found nothing.\n")
	}
	for _, f := range static {
		fmt.Fprintf(&b, "- `%s` at %s — %s\n", f.Rule, f.Location(), f.Title)
	}

	fmt.Fprintf(&b, "\n## Diff\n\n```diff\n%s\n```\n", d.Unified)
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
	Fix      string `json:"fix"`
	Doc      string `json:"doc"`
}

// Ingest reads a judge's structured output.
//
// A missing file is an error, not a clean review: if the judge did not run, the gate has
// no basis for a verdict. So is a verdict outside the contract — the schema should have
// stopped it, and a harness that trusted the schema alone would pass a lane on a typo.
func Ingest(path string) (verdict gate.Verdict, summary string, findings []gate.Finding, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return "", "", nil, fmt.Errorf("%s is empty — the judge produced no structured output", filepath.Base(path))
	}

	// A fenced block is tolerated: the contract is the JSON, not the bytes around it.
	var report Report
	if err := json.Unmarshal(unfence(data), &report); err != nil {
		return "", "", nil, fmt.Errorf("the judge's output is not valid JSON: %w", err)
	}
	verdict, ok := gate.ParseVerdict(report.Verdict)
	if !ok {
		return "", "", nil, fmt.Errorf("the judge's verdict %q is not PASS, CONCERNS or BLOCK", report.Verdict)
	}

	for _, f := range report.Findings {
		path := strings.TrimPrefix(strings.TrimSpace(f.File), "./")
		// The gate reviews Swift. A finding pinned to the Dart spec, or to nothing,
		// has no line a reviewer can act on.
		if path == "" || strings.HasPrefix(path, FlutterDir) {
			continue
		}
		findings = append(findings, gate.Finding{
			Rule:     NormaliseRule(f.Rule),
			Severity: gate.ParseSeverity(f.Severity),
			File:     path,
			Line:     f.Line,
			Title:    strings.TrimSpace(f.Title),
			Detail:   strings.TrimSpace(f.Detail),
			Fix:      strings.TrimSpace(f.Fix),
			Doc:      strings.TrimSpace(f.Doc),
			Source:   gate.FromAgent,
		})
	}
	return verdict, strings.TrimSpace(report.Summary), findings, nil
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
func Lane(ev evidence.Result, judgeRan bool, findingsPath string) gate.Lane {
	lane := gate.Lane{Name: string(ev.Lane)}
	cannot := func(reason string) gate.Lane {
		lane.Verdict, lane.Reason = gate.CannotEvaluate, reason
		return lane
	}
	switch {
	case !ev.Applies():
		lane.Skipped = ev.Skipped
		return lane
	case !ev.OK():
		return cannot("missing " + strings.Join(ev.Missing, "; "))
	case !judgeRan:
		return cannot("the judge step did not run to completion. Re-run the failed job; if it keeps " +
			"failing, check the CLAUDE_CODE_OAUTH_TOKEN secret and the subscription's rate limit.")
	}
	verdict, summary, findings, err := Ingest(findingsPath)
	if err != nil {
		return cannot("the judge returned nothing the harness can read: " + err.Error())
	}
	lane.Verdict, lane.Reason, lane.Findings = gate.Worse(verdict, gate.VerdictFromFindings(findings)), summary, findings
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
