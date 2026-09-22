package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	// Lanes is every judge lane, by name. A lane absent from the map does not run at
	// all; one present with blocks:false runs, reports and records, and stops nothing.
	Lanes map[string]LaneConfig `yaml:"lanes"`
	// RiskPaths are the globs that force the full review level (ADR 0017). A pattern
	// with no slash matches a file's name at any depth, so `Auth*` reaches
	// Packages/MindlensKit/Sources/Networking/AuthEndpoints.swift; `**` matches any
	// number of path segments.
	RiskPaths []string `yaml:"risk_paths"`
	// Pass is when a human need read nothing before merging.
	Pass PassRule `yaml:"pass"`
}

// LaneConfig is one lane's tuning: whether its verdict can stop a merge, and which
// model judges it.
type LaneConfig struct {
	// Blocks is ADR 0021's grant. False — the default, and where every lane starts —
	// means the lane's BLOCK and CANNOT_EVALUATE are reported on the pull request and
	// written to the metrics record, and stop nothing. It is set true only by a
	// reviewed pull request citing `swiftgate metrics`, and revoked the same way.
	Blocks bool `yaml:"blocks"`
	// Model is handed to the lane's runner and written into its metrics record, so a
	// lane's noise rate stays comparable across a model change instead of silently
	// becoming a number about a different judge.
	Model string `yaml:"model"`
}

// PassRule is the objective half of the pass level: the rest of ADR 0017's criteria —
// no risk path, no new ADR, dependency or entitlement — are what the repository is,
// not knobs, and live in internal/gate.
type PassRule struct {
	MaxChangedLines int `yaml:"max_changed_lines"`
}

func defaultConfig() Config {
	return Config{
		MaxDiffBytes:  180_000,
		OverrideLabel: "gate-override",
		// Advisory, all three: ADR 0021. The defaults are the documented behaviour, so
		// a run with no config file behaves as the file says rather than more strictly.
		Lanes: map[string]LaneConfig{
			string(evidence.Verification): {Model: "claude-opus-5"},
			string(evidence.Idiom):        {Model: "claude-sonnet-5"},
			string(evidence.Spec):         {Model: "claude-opus-5"},
		},
		RiskPaths: gate.DefaultRiskPaths,
		Pass:      PassRule{MaxChangedLines: 200},
	}
}

// loadConfig reads the file over the defaults, field by field.
//
// Unmarshalling straight onto defaultConfig() would merge the maps key by key, so a
// `lanes:` block that deliberately dropped a lane would leave the default one still
// running — the file would say one thing and the gate do another. Where the file
// speaks it is authoritative; where it is silent the default stands.
func loadConfig(repoDir, path string) (Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(path)))
	if os.IsNotExist(err) {
		return cfg, nil // the defaults are the documented behaviour
	}
	if err != nil {
		return cfg, err
	}
	var file Config
	if err := yaml.Unmarshal(data, &file); err != nil {
		return cfg, err
	}
	if file.MaxDiffBytes != 0 {
		cfg.MaxDiffBytes = file.MaxDiffBytes
	}
	if file.OverrideLabel != "" {
		cfg.OverrideLabel = file.OverrideLabel
	}
	if file.Severities != nil {
		cfg.Severities = file.Severities
	}
	if file.Lanes != nil {
		cfg.Lanes = file.Lanes
	}
	if file.RiskPaths != nil {
		cfg.RiskPaths = file.RiskPaths
	}
	if file.Pass.MaxChangedLines != 0 {
		cfg.Pass.MaxChangedLines = file.Pass.MaxChangedLines
	}
	return cfg, nil
}

// severities converts the config's strings into the engine's type. An unknown severity
// is a config error, not a warning: the file is the one place the gate's temperament is
// set, and a misspelling there must fail the run rather than quietly loosen it.
func (c Config) severities() (scan.Severities, error) {
	out := scan.Severities{}
	for id, s := range c.Severities {
		sev, ok := gate.ParseSeverity(s)
		if !ok {
			return nil, fmt.Errorf("config: severity %q for rule %s is not blocker, warning, nit or off", s, id)
		}
		out[id] = sev
	}
	return out, nil
}

// lanes is the configured lanes, in the order they are reported.
//
// A name the gate does not have is an error, on the same argument as a misspelt
// severity: `blocks: true` under a misspelt lane would read as a granted lane and be a
// lane that never runs, and the run would pass with nothing to show it.
func (c Config) lanes() ([]evidence.Lane, error) {
	var unknown []string
	for name := range c.Lanes {
		if _, ok := evidence.Parse(name); !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("config: lanes names %s — there is no such lane, only %s",
			strings.Join(quote(unknown), ", "), strings.Join(quote(laneNames()), ", "))
	}
	var out []evidence.Lane
	for _, l := range evidence.Lanes {
		if _, ok := c.Lanes[string(l)]; ok {
			out = append(out, l)
		}
	}
	return out, nil
}

// lane is one lane's tuning, or the zero value — which is advisory, with no model
// named — for a lane the config does not mention.
func (c Config) lane(l evidence.Lane) LaneConfig { return c.Lanes[string(l)] }

// blocking is the set of lanes whose verdict can stop a merge, for the scorer.
func (c Config) blocking() map[string]bool {
	out := map[string]bool{}
	for name, l := range c.Lanes {
		if l.Blocks {
			out[name] = true
		}
	}
	return out
}

func laneNames() []string {
	out := make([]string, 0, len(evidence.Lanes))
	for _, l := range evidence.Lanes {
		out = append(out, string(l))
	}
	return out
}

func quote(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, fmt.Sprintf("%q", s))
	}
	return out
}
