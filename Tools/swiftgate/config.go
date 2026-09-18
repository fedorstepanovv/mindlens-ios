package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/evidence"
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
	// Lanes names the judge lanes that score. Every lane's evidence is checked and
	// reported; only these can block, so a lane is switched on here once its skill exists.
	Lanes []string `yaml:"lanes"`
}

func defaultConfig() Config {
	return Config{
		MaxDiffBytes:  180_000,
		OverrideLabel: "gate-override",
		Lanes:         []string{string(evidence.Idiom)},
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

// lanes converts the config's strings into lane names, dropping any it does not know
// with a note rather than scoring a lane that cannot exist.
func (c Config) lanes() []evidence.Lane {
	var out []evidence.Lane
	for _, name := range c.Lanes {
		if l, ok := evidence.Parse(name); ok {
			out = append(out, l)
		} else {
			fmt.Fprintf(os.Stderr, "swiftgate: config names a lane %q that does not exist — ignored\n", name)
		}
	}
	return out
}
