package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A misspelt severity used to parse as "warning", so a typo in .github/swiftgate.yml
// silently turned a blocker into advice. The config is the one place the gate's
// temperament is set; a mistake there fails the run and names itself.
func TestMisspeltSeverityFailsTheRunInsteadOfDowngradingTheRule(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "swiftgate.yml"),
		[]byte("severities:\n  flutter/observable-object: blokcer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(repo, "swiftgate.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = cfg.severities()
	if err == nil {
		t.Fatal("a severity outside the vocabulary must be a config error, not a warning")
	}
	for _, want := range []string{"blokcer", "flutter/observable-object"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must name the value and the rule; missing %q in %q", want, err)
		}
	}

	// End to end: prepare on a real Swift change with that config must not exit 0.
	gitDir := gitRepo(t)
	if err := os.WriteFile(filepath.Join(gitDir, "swiftgate.yml"),
		[]byte("severities:\n  flutter/observable-object: blokcer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := prepare(t.Context(), []string{"--repo", gitDir, "--config", "swiftgate.yml", "--base", "main", "--head", "feature/dashboard"}); code != exitError {
		t.Fatalf("prepare must fail on a misspelt severity, got exit %d", code)
	}
	// And on the pull request that introduces the typo, which touches no Swift at all —
	// not on the next one that does.
	if code := prepare(t.Context(), []string{"--repo", gitDir, "--config", "swiftgate.yml", "--base", "main", "--head", "main"}); code != exitError {
		t.Fatalf("prepare must fail on a misspelt severity even with no Swift changed, got exit %d", code)
	}
}

func TestEverySeverityTheConfigDocumentsParses(t *testing.T) {
	cfg := Config{Severities: map[string]string{"a": "blocker", "b": "warning", "c": "nit", "d": "off"}}
	if _, err := cfg.severities(); err != nil {
		t.Fatalf("the documented vocabulary must parse: %v", err)
	}
}
