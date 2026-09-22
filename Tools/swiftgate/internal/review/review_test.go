package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/evidence"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

func writeFindings(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), LaneFindingsFile(evidence.Idiom))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIngestReadsFindings(t *testing.T) {
	path := writeFindings(t, `{
      "verdict": "BLOCK",
      "summary": "Reads as translated Dart.",
      "findings": [
        {"rule": "flutter/translated-layering", "severity": "blocker",
         "file": "Sources/A.swift", "line": 12,
         "title": "t", "detail": "d", "proof": "SignInView holds no ViewModel", "fix": "f", "doc": "docs/PATTERNS.md"}
      ]
    }`)

	j, err := Ingest(path)
	if err != nil {
		t.Fatal(err)
	}
	if j.Verdict != gate.Block || j.Summary != "Reads as translated Dart." {
		t.Errorf("verdict and summary not carried: %q %q", j.Verdict, j.Summary)
	}
	if len(j.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(j.Findings))
	}
	if j.Findings[0].Severity != gate.Blocker {
		t.Errorf("severity not parsed: %q", j.Findings[0].Severity)
	}
	if j.Findings[0].Proof != "SignInView holds no ViewModel" {
		t.Errorf("the proof is what makes the finding reportable; it must be carried: %q", j.Findings[0].Proof)
	}
	if j.Findings[0].Source != gate.FromAgent {
		t.Errorf("findings from the reviewer must be attributed to it, got %q", j.Findings[0].Source)
	}
}

func TestIngestToleratesAFencedFile(t *testing.T) {
	// Claude Code sometimes writes a JSON file wrapped in a code fence. Blocking a
	// merge over that would be absurd.
	path := writeFindings(t, "```json\n{\"verdict\": \"PASS\", \"summary\": \"clean\", \"findings\": []}\n```\n")
	j, err := Ingest(path)
	if err != nil {
		t.Fatalf("a fenced findings file should still parse: %v", err)
	}
	if j.Verdict != gate.Pass || j.Summary != "clean" || len(j.Findings) != 0 {
		t.Errorf("unexpected result: %q %q / %d", j.Verdict, j.Summary, len(j.Findings))
	}
}

func TestIngestRefusesASeverityOutsideTheSchema(t *testing.T) {
	path := writeFindings(t, `{"verdict":"CONCERNS","summary":"v","findings":[
      {"rule":"a/b","severity":"critical","file":"Sources/B.swift","title":"t","detail":"d","proof":"p","fix":"f"}
    ]}`)
	if _, err := Ingest(path); err == nil || !strings.Contains(err.Error(), "critical") {
		t.Fatalf("a severity the schema does not name is a contract violation, got %v", err)
	}
}

func TestIngestDropsFindingsWithNoActionableSwiftLine(t *testing.T) {
	path := writeFindings(t, `{"verdict":"CONCERNS","summary":"v","findings":[
      {"rule":"a/c","severity":"warning","file":"","title":"t","detail":"d","proof":"p","fix":"f"},
      {"rule":"a/d","severity":"warning","file":"./Sources/B.swift","title":"t","detail":"d","proof":"p","fix":"f"}
    ]}`)

	j, err := Ingest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(j.Findings) != 1 {
		t.Fatalf("expected only the Swift finding to survive, got %d", len(j.Findings))
	}
	if j.Findings[0].File != "Sources/B.swift" {
		t.Errorf("leading ./ should be stripped, got %q", j.Findings[0].File)
	}
}

func TestIngestFailsLoudlyOnMissingBrokenOrOffContractFile(t *testing.T) {
	if _, err := Ingest(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("a missing findings file must be an error, not an empty clean review")
	}
	for name, body := range map[string]string{
		"not json":        "not json at all",
		"empty":           "",
		"prose verdict":   `{"verdict": "looks fine", "summary": "s", "findings": []}`,
		"harness verdict": `{"verdict": "CANNOT_EVALUATE", "summary": "s", "findings": []}`,
	} {
		if _, err := Ingest(writeFindings(t, body)); err == nil {
			t.Errorf("%s: must be an error, never a verdict", name)
		}
	}
}

func TestSchemaIsValidJSONAndNamesTheContract(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(Schema), &schema); err != nil {
		t.Fatalf("the schema is not JSON: %v", err)
	}
	for _, want := range []string{`"PASS"`, `"CONCERNS"`, `"BLOCK"`, `"findings"`, `"summary"`, `"proof"`} {
		if !strings.Contains(Schema, want) {
			t.Errorf("schema should name %s", want)
		}
	}
	if strings.Contains(Schema, "CANNOT_EVALUATE") {
		t.Error("the harness's verdict must not be one a judge can return")
	}
	// The schema is the first place the proof is asked for, and Ingest is the second.
	// Asking in only one place is how it becomes optional in practice.
	required := schema["properties"].(map[string]any)["findings"].(map[string]any)["items"].(map[string]any)["required"].([]any)
	var hasProof bool
	for _, f := range required {
		hasProof = hasProof || f == "proof"
	}
	if !hasProof {
		t.Errorf("proof must be required of every finding, not merely offered: %v", required)
	}
}

// ADR 0021: a finding without a proof is not reported. It is counted, because a judge
// that mostly writes findings it cannot support is the one the kill rule is about —
// and a silent drop would hide exactly that.
func TestIngestDropsUnprovenFindingsAndCountsThem(t *testing.T) {
	path := writeFindings(t, `{"verdict":"CONCERNS","summary":"v","findings":[
      {"rule":"a/b","severity":"warning","file":"Sources/A.swift","line":1,"title":"no proof","detail":"d","fix":"f"},
      {"rule":"a/c","severity":"warning","file":"Sources/B.swift","line":2,"title":"blank proof","detail":"d","proof":"   ","fix":"f"},
      {"rule":"a/d","severity":"warning","file":"Sources/C.swift","line":3,"title":"proven","detail":"d","proof":"loadFailed() never sets error; line 3 returns early","fix":"f"}
    ]}`)

	j, err := Ingest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(j.Findings) != 1 || j.Findings[0].Title != "proven" {
		t.Fatalf("only the proven finding may be reported, got %+v", j.Findings)
	}
	if j.Unproven != 2 {
		t.Errorf("both unproven findings must be counted, got %d", j.Unproven)
	}
}

// A judge can still say BLOCK with nothing reportable behind it. That reads as
// "BLOCK, 0 findings, 2 unproven" — the truth about the judge. Quietly downgrading it
// to PASS would hide the lane most worth cutting.
func TestAVerdictStandsWhenEveryFindingBehindItWasUnproven(t *testing.T) {
	present := evidence.Result{Lane: evidence.Idiom, Present: []string{"the diff"}}
	path := writeFindings(t, `{"verdict":"BLOCK","summary":"translated","findings":[
      {"rule":"a/b","severity":"blocker","file":"Sources/A.swift","line":1,"title":"t","detail":"d","fix":"f"}
    ]}`)
	lane := Lane(present, true, false, path)
	if lane.Verdict != gate.Block || len(lane.Findings) != 0 || lane.Unproven != 1 {
		t.Errorf("want BLOCK with 0 findings and 1 unproven, got %+v", lane)
	}
	if lane.Blocks {
		t.Error("the lane was not granted; it must not carry Blocks")
	}
	if !lane.Advisory() {
		t.Error("an ungranted lane saying BLOCK is exactly what Advisory is for")
	}
	if body := lane.Markdown(gate.ReportContext{}, "x"); !strings.Contains(body, "proof") || !strings.Contains(body, "Advisory") {
		t.Errorf("the lane comment must say what was dropped and that it stopped nothing:\n%s", body)
	}
}

// The order of Lane's checks is the contract. Each way a judgement can be absent is
// CANNOT_EVALUATE, and none of them is ever read as a pass.
func TestLaneIsCannotEvaluateWheneverTheJudgementIsAbsent(t *testing.T) {
	present := evidence.Result{Lane: evidence.Idiom, Present: []string{"the diff"}}
	clean := writeFindings(t, `{"verdict": "PASS", "summary": "clean", "findings": []}`)

	cases := map[string]gate.Lane{
		"evidence missing":               Lane(evidence.Result{Lane: evidence.Idiom, Missing: []string{"a Dart file"}}, true, true, clean),
		"judge did not run":              Lane(present, false, true, clean),
		"findings file absent":           Lane(present, true, true, filepath.Join(t.TempDir(), "absent.json")),
		"findings file not the contract": Lane(present, true, true, writeFindings(t, "not json")),
	}
	for name, lane := range cases {
		if lane.Verdict != gate.CannotEvaluate {
			t.Errorf("%s: want CANNOT_EVALUATE, got %q", name, lane.Verdict)
		}
		if lane.Reason == "" {
			t.Errorf("%s: the reason must say what was missing", name)
		}
	}
	if !strings.Contains(cases["evidence missing"].Reason, "a Dart file") {
		t.Error("the evidence gate's missing list must reach the report")
	}
	if !strings.Contains(cases["judge did not run"].Reason, "Re-run") {
		t.Error("a judge that did not run should tell the reader how to recover")
	}
}

func TestLaneReadsTheJudgeOnlyWhenEverythingElseHolds(t *testing.T) {
	present := evidence.Result{Lane: evidence.Idiom, Present: []string{"the diff"}}
	path := writeFindings(t, `{"verdict": "PASS", "summary": "one real problem", "findings": [
	  {"rule": "a/b", "severity": "warning", "file": "Sources/A.swift", "line": 1, "title": "t", "detail": "d", "proof": "p", "fix": "f"}
	]}`)
	lane := Lane(present, true, true, path)
	if lane.Verdict != gate.Concerns || len(lane.Findings) != 1 || lane.Reason != "one real problem" {
		t.Errorf("a judge that says PASS over a warning has contradicted itself; the findings win: %+v", lane)
	}

	skipped := Lane(evidence.Result{Lane: evidence.Spec, Skipped: "not a feature branch"}, false, true, path)
	if skipped.Skipped == "" || skipped.Verdict != "" {
		t.Errorf("a lane that does not apply is skipped, not judged and not failed: %+v", skipped)
	}
}

func TestNormaliseRule(t *testing.T) {
	for in, want := range map[string]string{
		"flutter/translated-layering": "flutter/translated-layering",
		"Flutter/Translated Layering": "flutter/translated-layering",
		"cleanup":                     "review/cleanup",
		"  DESIGN/Custom Component  ": "design/custom-component",
		"":                            "review/unspecified",
	} {
		if got := NormaliseRule(in); got != want {
			t.Errorf("NormaliseRule(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrepareWritesABriefTheSkillCanRead(t *testing.T) {
	dir := t.TempDir()
	d := scan.Diff{
		Base:    "abcdef1234567890",
		Unified: "--- a/A.swift\n+++ b/A.swift\n+let x = 1\n",
		Files: []scan.ChangedFile{{
			Path: "Packages/MindlensKit/Sources/Features/Dashboard/A.swift", Status: "A",
			Added: []scan.AddedLine{{Number: 1, Text: "let x = 1"}},
		}},
	}
	static := []gate.Finding{{Rule: "design/hardcoded-color", File: "A.swift", Line: 3, Title: "colour"}}

	exemplars := []scan.Exemplar{{For: "Packages/MindlensKit/Sources/Features/Dashboard/A.swift", Path: "mindlens/RootView.swift", Body: "struct RootView: View {}"}}
	if err := Prepare(dir, d, Meta{Title: "Add insights"}, static, exemplars); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(filepath.Join(dir, ContextFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)

	for _, want := range []string{
		"Add insights",
		"Features/Dashboard/A.swift",
		"design/hardcoded-color",  // told not to repeat it
		"mindlens/RootView.swift", // the exemplar, and its body
		"struct RootView: View {}",
		"structured output", // told how to answer
		"+let x = 1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the brief is missing %q:\n%s", want, text)
		}
	}
}

func TestPrepareSaysSoWhenThereAreNoExemplars(t *testing.T) {
	dir := t.TempDir()
	if err := Prepare(dir, scan.Diff{Base: "abc"}, Meta{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ContextFile))
	if !strings.Contains(string(body), "None were found") {
		t.Error("the judge must be told what is absent rather than left to guess")
	}
}

func TestStateRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), StateFile)
	want := State{Meta: Meta{Number: 7, Title: "t", SHA: "abc"}, Base: "b", Head: "h", Reviewed: true}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.Number != 7 || !got.Reviewed || got.Base != "b" {
		t.Errorf("state did not round-trip: %+v", got)
	}
}
