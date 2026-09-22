package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/evidence"
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

// The same argument as the severity above, one level up. `blocks: true` under a
// misspelt lane name reads like a granted lane and is a lane that never runs, and the
// pull request would go green with nothing having judged it.
func TestAMisspeltLaneNameFailsTheRun(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "swiftgate.yml"),
		[]byte("lanes:\n  idium:\n    blocks: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(repo, "swiftgate.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = cfg.lanes()
	if err == nil {
		t.Fatal("a lane the gate does not have must be a config error")
	}
	for _, want := range []string{"idium", "idiom"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must name what was written and what exists; missing %q in %q", want, err)
		}
	}
}

// Every lane starts advisory, and a config file that names some lanes does not leave
// the others running from the defaults underneath it — the file says what runs.
func TestLanesDefaultToAdvisoryAndTheFileIsAuthoritative(t *testing.T) {
	cfg, err := loadConfig(t.TempDir(), "absent.yml")
	if err != nil {
		t.Fatal(err)
	}
	lanes, err := cfg.lanes()
	if err != nil {
		t.Fatal(err)
	}
	if len(lanes) != len(evidence.Lanes) {
		t.Fatalf("with no config every lane runs, got %v", lanes)
	}
	for _, l := range lanes {
		if cfg.lane(l).Blocks {
			t.Errorf("%s must start advisory (ADR 0021)", l)
		}
		if cfg.lane(l).Model == "" {
			t.Errorf("%s needs a model, or its records cannot say which judge produced them", l)
		}
	}
	if len(cfg.blocking()) != 0 {
		t.Errorf("nothing blocks by default, got %v", cfg.blocking())
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "swiftgate.yml"),
		[]byte("lanes:\n  spec:\n    blocks: true\n    model: m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = loadConfig(repo, "swiftgate.yml")
	if err != nil {
		t.Fatal(err)
	}
	lanes, err = cfg.lanes()
	if err != nil {
		t.Fatal(err)
	}
	if len(lanes) != 1 || lanes[0] != evidence.Spec {
		t.Fatalf("a file naming one lane runs that lane and no other, got %v", lanes)
	}
	if !cfg.lane(evidence.Spec).Blocks || cfg.lane(evidence.Idiom).Blocks {
		t.Error("the grant is per lane, and a lane the file omits is not granted")
	}
	// The other keys keep their defaults, so a file that only retunes lanes does not
	// silently zero the diff cap or the override label.
	if cfg.MaxDiffBytes == 0 || cfg.OverrideLabel == "" || len(cfg.RiskPaths) == 0 || cfg.Pass.MaxChangedLines == 0 {
		t.Errorf("a silent key keeps its default: %+v", cfg)
	}
}

// The old shape was a list of names. It cannot mean anything under the new one, and a
// file left in the old shape must fail rather than be read as "no lanes at all".
func TestTheOldLaneListIsRefusedRatherThanReadAsNoLanes(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "swiftgate.yml"),
		[]byte("lanes: [verification, idiom, spec]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(repo, "swiftgate.yml"); err == nil {
		t.Fatal("a sequence where a mapping is expected must be a config error")
	}
}
