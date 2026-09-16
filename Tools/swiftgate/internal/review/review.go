// Package review is the seam between the gate and the reviewer.
//
// The reviewer is Claude Code, running as a step in the same job. This package writes
// the brief it reads and ingests the findings it writes back. It deliberately does not
// call a model: the gate holds the decision, the reviewer holds the judgement, and
// keeping those in separate processes is what lets the review bill against a Claude
// subscription instead of the API.
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

// RunDir is the working directory the two halves share. It is git-ignored: the brief
// and the findings are build artefacts, not source.
const RunDir = ".swiftgate/run"

// Paths inside RunDir. The skill hardcodes ContextFile and FindingsFile, so these names
// are a contract with .claude/skills/idiom-review/SKILL.md — change both together.
const (
	ContextFile  = "context.md"
	FindingsFile = "agent-findings.json"
	StaticFile   = "static-findings.json"
	StateFile    = "state.json"
)

// FlutterDir is where CI checks the Flutter app out, relative to the repository.
const FlutterDir = ".swiftgate/flutter"

// Meta is what the gate knows about the pull request itself.
type Meta struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	SHA    string `json:"sha"`
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
}

// Prepare writes the brief the reviewer reads. Returns false when there is nothing to
// review, so the caller can skip the reviewer entirely rather than spend a run on it.
func Prepare(dir string, d scan.Diff, meta Meta, static []gate.Finding, flutterAvailable bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ContextFile),
		[]byte(brief(d, meta, static, flutterAvailable)), 0o644)
}

// brief renders the pull request for a reader who has the repository on disk but has
// not seen the change.
func brief(d scan.Diff, meta Meta, static []gate.Finding, flutterAvailable bool) string {
	var b strings.Builder

	b.WriteString("# The pull request under review\n\n")
	if meta.Title != "" {
		fmt.Fprintf(&b, "**%s**\n\n", meta.Title)
	}
	if body := strings.TrimSpace(meta.Body); body != "" {
		fmt.Fprintf(&b, "%s\n\n", body)
	}
	fmt.Fprintf(&b, "Merging into `%s`. %d changed file(s).\n\n", shortSHA(d.Base), len(d.Files))

	b.WriteString("## Changed files\n\n")
	for _, f := range d.Files {
		status := "modified"
		if f.Status == "A" {
			status = "new"
		}
		fmt.Fprintf(&b, "- `%s` (%s, +%d)\n", f.Path, status, len(f.Added))
	}

	b.WriteString("\n## The Flutter spec\n\n")
	if flutterAvailable {
		fmt.Fprintf(&b, "The Flutter app is checked out at `%s`; its Dart sources are under `%s/lib`. "+
			"Grep it to find the screen a Swift file corresponds to, and judge whether structure was carried over.\n",
			FlutterDir, FlutterDir)
	} else {
		b.WriteString("**Not available in this run.** Judge the Swift on its own merits and on the " +
			"project's documents. Do not speculate about what the Dart looks like.\n")
	}

	b.WriteString("\n## Already reported — do not repeat these\n\n")
	if len(static) == 0 {
		b.WriteString("The deterministic pass found nothing.\n")
	}
	for _, f := range static {
		fmt.Fprintf(&b, "- `%s` at %s — %s\n", f.Rule, f.Location(), f.Title)
	}

	fmt.Fprintf(&b, "\n## Diff\n\n```diff\n%s\n```\n", d.Unified)
	fmt.Fprintf(&b, "\nWrite your findings to `%s` and stop.\n", filepath.Join(RunDir, FindingsFile))

	return b.String()
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// Report is the shape the reviewer writes back. It mirrors the JSON block documented in
// the skill; the two must stay in step.
type Report struct {
	Verdict  string       `json:"verdict"`
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

// Ingest reads the reviewer's findings.
//
// A missing file is reported as such rather than treated as a clean review: if the
// reviewer did not run, the gate has no basis for saying the change is idiomatic.
func Ingest(path string) (verdict string, findings []gate.Finding, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}

	// Claude Code sometimes wraps a JSON file in a fenced block. Accept both rather
	// than blocking a merge over a formatting habit.
	var report Report
	if err := json.Unmarshal(unfence(data), &report); err != nil {
		return "", nil, fmt.Errorf("the reviewer's findings file is not valid JSON: %w", err)
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
	return strings.TrimSpace(report.Verdict), findings, nil
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

// Incomplete is the finding raised when the reviewer was expected but produced nothing.
// It blocks, because "the reviewer did not run" is not evidence that the code is clean —
// and it is re-runnable, which the message says.
func Incomplete(reason string) gate.Finding {
	return gate.Finding{
		Rule:     "gate/review-incomplete",
		Severity: gate.Blocker,
		File:     "",
		Title:    "The idiom review did not complete",
		Detail: "The reviewer produced no findings file, so nothing has judged whether this " +
			"change reads as native Swift.\n\n> " + reason,
		Fix: "Re-run the failed job. If it keeps failing, check the CLAUDE_CODE_OAUTH_TOKEN " +
			"secret and whether the subscription has run into its rate limit.",
		Source: gate.FromAgent,
	}
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
