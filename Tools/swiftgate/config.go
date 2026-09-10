package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

// Config is the project's tuning of the gate, read from .github/swiftgate.yml.
// Rules live in code; how hard each one bites lives here, so retuning the gate is a
// one-line review rather than a Go change.
type Config struct {
	// MaxDiffBytes caps what is written into the reviewer's brief. It reads the rest
	// from the checkout rather than being handed a diff that dwarfs the rubric.
	MaxDiffBytes int `yaml:"max_diff_bytes"`
	// OverrideLabel is the PR label that lets a human merge past a blocker, provided
	// they also give a reason.
	OverrideLabel string `yaml:"override_label"`
	// Severities retunes or disables individual rules by id.
	Severities map[string]string `yaml:"severities"`
}

func defaultConfig() Config {
	return Config{
		MaxDiffBytes:  180_000,
		OverrideLabel: "gate-override",
	}
}

func loadConfig(repoDir, path string) (Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(path)))
	if os.IsNotExist(err) {
		return cfg, nil // the defaults are the documented behaviour
	}
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// severities converts the config's strings into the engine's type.
func (c Config) severities() scan.Severities {
	out := scan.Severities{}
	for id, s := range c.Severities {
		out[id] = gate.ParseSeverity(s)
	}
	return out
}
