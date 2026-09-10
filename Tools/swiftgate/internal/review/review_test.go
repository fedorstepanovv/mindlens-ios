package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

func writeFindings(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FindingsFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIngestReadsFindings(t *testing.T) {
	path := writeFindings(t, `{
      "verdict": "Reads as translated Dart.",
      "findings": [
        {"rule": "flutter/translated-layering", "severity": "blocker",
         "file": "Sources/A.swift", "line": 12,
         "title": "t", "detail": "d", "fix": "f", "doc": "docs/PATTERNS.md"}
      ]
    }`)

	verdict, findings, err := Ingest(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "Reads as translated Dart." {
		t.Errorf("verdict not carried: %q", verdict)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != gate.Blocker {
		t.Errorf("severity not parsed: %q", findings[0].Severity)
	}
	if findings[0].Source != gate.FromAgent {
		t.Errorf("findings from the reviewer must be attributed to it, got %q", findings[0].Source)
	}
}

func TestIngestToleratesAFencedFile(t *testing.T) {
	// Claude Code sometimes writes a JSON file wrapped in a code fence. Blocking a
	// merge over that would be absurd.
	path := writeFindings(t, "```json\n{\"verdict\": \"clean\", \"findings\": []}\n```\n")
	verdict, findings, err := Ingest(path)
	if err != nil {
		t.Fatalf("a fenced findings file should still parse: %v", err)
	}
	if verdict != "clean" || len(findings) != 0 {
		t.Errorf("unexpected result: %q / %d", verdict, len(findings))
	}
}

func TestIngestDropsFindingsWithNoActionableSwiftLine(t *testing.T) {
	path := writeFindings(t, `{"verdict":"v","findings":[
      {"rule":"a/b","severity":"warning","file":".swiftgate/flutter/lib/x.dart","title":"t","detail":"d","fix":"f"},
      {"rule":"a/c","severity":"warning","file":"","title":"t","detail":"d","fix":"f"},
      {"rule":"a/d","severity":"warning","file":"./Sources/B.swift","title":"t","detail":"d","fix":"f"}
    ]}`)

	_, findings, err := Ingest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected only the Swift finding to survive, got %d", len(findings))
	}
	if findings[0].File != "Sources/B.swift" {
		t.Errorf("leading ./ should be stripped, got %q", findings[0].File)
	}
}

func TestIngestFailsLoudlyOnMissingOrBrokenFile(t *testing.T) {
	if _, _, err := Ingest(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("a missing findings file must be an error, not an empty clean review")
	}
	path := writeFindings(t, "not json at all")
	if _, _, err := Ingest(path); err == nil {
		t.Error("unparseable findings must be an error")
	}
}

func TestIncompleteBlocksAndSaysHowToRecover(t *testing.T) {
	f := Incomplete("rate limited")
	if f.Severity != gate.Blocker {
		t.Error("a review that did not run is not evidence the code is clean; it must block")
	}
	if !strings.Contains(f.Fix, "Re-run") {
		t.Error("the finding should tell the reader how to recover")
	}
	if !strings.Contains(f.Detail, "rate limited") {
		t.Error("the underlying reason should be carried through")
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

	if err := Prepare(dir, d, Meta{Title: "Add insights"}, static, true); err != nil {
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
		"design/hardcoded-color",            // told not to repeat it
		FlutterDir + "/lib",                 // told where the spec is
		filepath.Join(RunDir, FindingsFile), // told where to write
		"+let x = 1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the brief is missing %q:\n%s", want, text)
		}
	}
}

func TestPrepareSaysSoWhenTheSpecIsAbsent(t *testing.T) {
	dir := t.TempDir()
	if err := Prepare(dir, scan.Diff{Base: "abc"}, Meta{}, nil, false); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ContextFile))
	if !strings.Contains(string(body), "Not available in this run") {
		t.Error("the reviewer must be told the spec is missing rather than guessing about it")
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
