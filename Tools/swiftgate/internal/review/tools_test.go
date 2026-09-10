package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
)

func testWorkspace(t *testing.T) (*workspace, string, string) {
	t.Helper()
	repo := t.TempDir()
	flutter := t.TempDir()

	mustWrite(t, filepath.Join(repo, "Sources", "A.swift"), "import Foundation\nlet a = 1\nlet b = 2\n")
	mustWrite(t, filepath.Join(flutter, "lib", "insights_cubit.dart"), "class InsightsCubit extends Cubit<InsightsState> {}\n")
	mustWrite(t, filepath.Join(filepath.Dir(repo), "outside.txt"), "secret\n")

	w, err := newWorkspace(repo, flutter)
	if err != nil {
		t.Fatal(err)
	}
	return w, repo, flutter
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRejectsTraversal(t *testing.T) {
	w, _, _ := testWorkspace(t)
	for _, bad := range []string{"../outside.txt", "Sources/../../outside.txt", "/etc/passwd"} {
		if _, err := w.resolve(bad); err == nil {
			t.Errorf("resolve(%q) should have been refused", bad)
		}
	}
}

func TestResolveSplitsRepoAndFlutter(t *testing.T) {
	w, repo, flutter := testWorkspace(t)

	got, err := w.resolve("Sources/A.swift")
	if err != nil || got != filepath.Join(repo, "Sources/A.swift") {
		t.Fatalf("repo path resolved to %q (%v)", got, err)
	}

	got, err = w.resolve("flutter/lib/insights_cubit.dart")
	if err != nil || got != filepath.Join(flutter, "lib/insights_cubit.dart") {
		t.Fatalf("flutter path resolved to %q (%v)", got, err)
	}
}

func TestFlutterPathFailsCleanlyWhenSpecMissing(t *testing.T) {
	w, err := newWorkspace(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.resolve("flutter/lib/main.dart"); err == nil {
		t.Fatal("expected an explanatory error when the spec repo is absent")
	}
}

func TestReadFileRespectsLineRange(t *testing.T) {
	w, _, _ := testWorkspace(t)
	out, err := w.readFile(context.Background(), readFileInput{Path: "Sources/A.swift", StartLine: 2, EndLine: 2})
	if err != nil {
		t.Fatal(err)
	}
	text := out.OfText.Text
	if !strings.Contains(text, "2\tlet a = 1") {
		t.Errorf("expected only line 2, got:\n%s", text)
	}
	if strings.Contains(text, "let b = 2") {
		t.Errorf("range was not honoured:\n%s", text)
	}
}

func TestSearchFindsDartAndReportsRepoRelativePaths(t *testing.T) {
	w, _, _ := testWorkspace(t)
	out, err := w.search(context.Background(), searchInput{Pattern: "Cubit", Path: "flutter/lib", Ext: "dart"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.OfText.Text, "flutter/lib/insights_cubit.dart:1") {
		t.Errorf("expected a flutter-prefixed hit, got:\n%s", out.OfText.Text)
	}
}

func TestCollectorNormalisesFindings(t *testing.T) {
	w, _, _ := testWorkspace(t)
	c := &collector{}

	_, err := c.tool(w)(context.Background(), reportInput{
		Verdict: "Reads as translated Dart.",
		Findings: []agentFinding{
			{Rule: "Flutter/Translated Layering", Severity: "BLOCKER", File: "./Sources/A.swift", Line: 3, Title: "t", Detail: "d", Fix: "f"},
			{Rule: "cleanup", Severity: "whatever", File: "Sources/A.swift", Title: "t2", Detail: "d", Fix: "f"},
			{Rule: "spec-only", Severity: "blocker", File: "flutter/lib/x.dart", Title: "t3", Detail: "d", Fix: "f"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !c.called {
		t.Fatal("collector should record that the verdict landed")
	}
	if len(c.findings) != 2 {
		t.Fatalf("a finding pinned to the Dart spec has no Swift line to act on and should be dropped; got %d", len(c.findings))
	}
	if c.findings[0].Rule != "flutter/translated-layering" {
		t.Errorf("rule not normalised: %q", c.findings[0].Rule)
	}
	if c.findings[0].Severity != gate.Blocker {
		t.Errorf("severity not parsed: %q", c.findings[0].Severity)
	}
	if c.findings[0].File != "Sources/A.swift" {
		t.Errorf("leading ./ not stripped: %q", c.findings[0].File)
	}
	if c.findings[1].Rule != "review/cleanup" {
		t.Errorf("a rule with no category should be namespaced: %q", c.findings[1].Rule)
	}
	if got := normaliseRule("Flutter Translated Layering"); got != "review/flutter-translated-layering" {
		t.Errorf("a free-text rule should still become a usable id, got %q", got)
	}
	if c.findings[1].Severity != gate.Warning {
		t.Errorf("an unrecognised severity should fall back to warning, got %q", c.findings[1].Severity)
	}
}

func TestBuildToolsProducesTheFourTools(t *testing.T) {
	w, _, _ := testWorkspace(t)
	tools, err := buildTools(w, &collector{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"read_file": true, "list_directory": true, "search_code": true, "report_findings": true}
	for _, tool := range tools {
		delete(want, tool.Name())
		if tool.InputSchema().Properties == nil {
			t.Errorf("%s has no generated schema", tool.Name())
		}
	}
	if len(want) != 0 {
		t.Errorf("missing tools: %v", want)
	}
}

func TestSystemPromptCarriesTheRubricAndACacheBreakpoint(t *testing.T) {
	blocks := System("../../../..")
	if len(blocks) != 2 {
		t.Fatalf("expected instructions plus rubric, got %d block(s)", len(blocks))
	}
	if !strings.Contains(blocks[1].Text, "## CLAUDE.md") {
		t.Error("the rubric should include CLAUDE.md")
	}
	if !strings.Contains(blocks[1].Text, "## docs/PATTERNS.md") {
		t.Error("the rubric should include docs/PATTERNS.md")
	}
	// The rubric is identical between pull requests; without a breakpoint every run
	// pays full price for it.
	if blocks[1].CacheControl.Type == "" {
		t.Error("the rubric block should carry a cache breakpoint")
	}
	if blocks[0].CacheControl.Type != "" {
		t.Error("only the last system block should carry the breakpoint")
	}
}
