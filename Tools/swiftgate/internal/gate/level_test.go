package gate

import (
	"strings"
	"testing"
)

// inputs is a pull request that would be assigned pass: small, off the risk class,
// no decision, no dependency. Each case below moves one thing.
func inputs(files []string, changed int) LevelInputs {
	return LevelInputs{
		Files:               files,
		Changed:             changed,
		RiskPaths:           DefaultRiskPaths,
		PassMaxChangedLines: 200,
		SizeCap:             1000,
	}
}

// ADR 0017's table, row by row. The level is what a human reads before merging, and it
// is assigned from the diff and nothing else — never from a severity a model chose on
// the day, which is why every row here is a fact about paths, counts and proofs.
func TestLevelTable(t *testing.T) {
	readme := []string{"README.md"}
	feature := []string{"Packages/MindlensKit/Sources/Features/Dashboard/MoodBadge.swift"}

	cases := []struct {
		name   string
		in     LevelInputs
		want   Level
		reason string
	}{
		{"small, off the risk class", inputs(readme, 12), LevelPass, "no risk path"},
		{"a feature target is not the risk class", inputs(feature, 180), LevelPass, ""},
		{"exactly at the pass ceiling", inputs(readme, 200), LevelPass, ""},
		{"one line over it", inputs(readme, 201), LevelBrief, "over the 200"},
		{"a new decision", inputs([]string{"docs/decisions/0024-a-choice.md"}, 40), LevelBrief, "adds a decision"},
		{"a dependency", inputs([]string{"Packages/MindlensKit/Package.swift"}, 4), LevelBrief, "changes a dependency"},

		{"the networking layer", inputs([]string{"Packages/MindlensKit/Sources/Networking/APIClient.swift"}, 9), LevelFull, "risk class"},
		{"persistence", inputs([]string{"Packages/MindlensKit/Sources/Persistence/Store.swift"}, 9), LevelFull, "risk class"},
		{"the gate itself", inputs([]string{"Tools/swiftgate/main.go"}, 9), LevelFull, "risk class"},
		{"a workflow", inputs([]string{".github/workflows/pr-gate.yml"}, 3), LevelFull, "risk class"},
		{"the build settings", inputs([]string{"Config/Base.xcconfig"}, 1), LevelFull, "risk class"},
		{"the instructions every agent reads", inputs([]string{"AGENTS.md"}, 2), LevelFull, "risk class"},
		{"a skill", inputs([]string{".claude/skills/pr/SKILL.md"}, 2), LevelFull, "risk class"},
		// The token code is named by file, not by directory, so it stays in the risk
		// class wherever it moves to.
		{"token code, wherever it lives", inputs([]string{"Packages/MindlensKit/Sources/Core/TokenRefresher.swift"}, 5), LevelFull, "risk class"},
		{"a keychain file", inputs([]string{"Packages/MindlensKit/Sources/Core/KeychainStore.swift"}, 5), LevelFull, "risk class"},
		{"an entitlement", inputs([]string{"mindlens/mindlens.entitlements"}, 1), LevelFull, "risk class"},
		{"the app's plist", inputs([]string{"mindlens/Info.plist"}, 1), LevelFull, "risk class"},
		// A path that merely resembles one on the list is not on it.
		{"a test of the networking layer is not the networking layer",
			inputs([]string{"Packages/MindlensKit/Tests/NetworkingTests/APIClientTests.swift"}, 30), LevelPass, ""},
		{"a doc about tools is not Tools/", inputs([]string{"docs/TESTING.md"}, 30), LevelPass, ""},
	}
	for _, c := range cases {
		got := Assess(c.in)
		if got.Level != c.want {
			t.Errorf("%s: level = %q, want %q (%v)", c.name, got.Level, c.want, got.Reasons)
		}
		if c.reason != "" && !strings.Contains(strings.Join(got.Reasons, "\n"), c.reason) {
			t.Errorf("%s: reasons %v should name %q", c.name, got.Reasons, c.reason)
		}
		if len(got.Reasons) == 0 {
			t.Errorf("%s: a level with no reason is a number nobody can check", c.name)
		}
	}
}

// A proven lane finding at warning or above sends a human to the diff. One at nit does
// not — the level is about what is worth reading, not about tidiness.
func TestALaneFindingAtWarningOrAboveForcesFull(t *testing.T) {
	small := inputs([]string{"README.md"}, 10)

	for _, sev := range []Severity{Blocker, Warning} {
		in := small
		in.Findings = []Finding{{Severity: sev, Title: "a real one", File: "A.swift", Line: 3, Proof: "p"}}
		if got := Assess(in); got.Level != LevelFull {
			t.Errorf("a %s finding must force full, got %q (%v)", sev, got.Level, got.Reasons)
		}
	}

	in := small
	in.Findings = []Finding{{Severity: Nit, Title: "naming", File: "A.swift", Proof: "p"}}
	if got := Assess(in); got.Level != LevelPass {
		t.Errorf("a nit alone must not force full, got %q (%v)", got.Level, got.Reasons)
	}
}

// Over the cap is not a level by itself — the conventions job blocks it. It becomes a
// level when a human lifts the cap, and then the whole diff is read.
func TestOverTheCapOnAnOverrideIsFull(t *testing.T) {
	in := inputs([]string{"README.md"}, 1400)
	if got := Assess(in); got.Level != LevelBrief {
		t.Errorf("over the cap with no override is the conventions job's business, got %q", got.Level)
	}
	in.SizeOverridden = true
	got := Assess(in)
	if got.Level != LevelFull || !strings.Contains(strings.Join(got.Reasons, "\n"), "size override") {
		t.Errorf("over the cap on an override must be full and say so, got %q (%v)", got.Level, got.Reasons)
	}
}

func TestMatchPath(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"Packages/*/Sources/Networking/**", "Packages/MindlensKit/Sources/Networking/APIClient.swift", true},
		{"Packages/*/Sources/Networking/**", "Packages/MindlensKit/Sources/Networking/Auth/Endpoints.swift", true},
		{"Packages/*/Sources/Networking/**", "Packages/MindlensKit/Sources/Models/User.swift", false},
		// `**` matches one or more segments, not zero: the directory itself is not a file.
		{"Config/**", "Config/Base.xcconfig", true},
		{"Config/**", "Config", false},
		{"Tools/**", "Tools/githooks/pre-commit", true},
		{"AGENTS.md", "AGENTS.md", true},
		{"AGENTS.md", "docs/AGENTS.md", true}, // a slashless pattern is a name, at any depth
		{"Auth*", "Packages/MindlensKit/Sources/Networking/AuthEndpoints.swift", true},
		{"Auth*", "Packages/MindlensKit/Sources/Core/SessionModel.swift", false},
		{"*.entitlements", "mindlens/mindlens.entitlements", true},
		{"docs/decisions/[0-9][0-9][0-9][0-9]-*.md", "docs/decisions/0024-x.md", true},
		{"docs/decisions/[0-9][0-9][0-9][0-9]-*.md", "docs/decisions/0000-template.md", true},
		{"docs/decisions/[0-9][0-9][0-9][0-9]-*.md", "docs/decisions/README.md", false},
	}
	for _, c := range cases {
		if got := MatchPath(c.pattern, c.path); got != c.want {
			t.Errorf("MatchPath(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

// The sentence is what a human actually reads at the top of the comment.
func TestSentenceSaysWhatToReadAndWhy(t *testing.T) {
	r := Assess(inputs([]string{"Tools/swiftgate/main.go"}, 9))
	s := r.Sentence()
	for _, want := range []string{"Read: full", "the diff", "Tools/swiftgate/main.go"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if r.Label() != "review:full" {
		t.Errorf("label = %q", r.Label())
	}
}
