package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

func touch(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func swiftDiff(paths ...string) scan.Diff {
	d := scan.Diff{Unified: "+let x = 1\n"}
	for _, p := range paths {
		d.Files = append(d.Files, scan.ChangedFile{Path: p, Status: "M",
			Added: []scan.AddedLine{{Number: 1, Text: "let x = 1"}}})
	}
	return d
}

// viewDiff is a diff the idiom lane applies to: it adds a type conforming to View.
func viewDiff(paths ...string) scan.Diff {
	d := swiftDiff(paths...)
	d.Files[0].Added = append(d.Files[0].Added, scan.AddedLine{Number: 2, Text: "struct MoodBadge: View {"})
	return d
}

const featureFile = "Packages/MindlensKit/Sources/Features/Dashboard/DashboardModel.swift"

// The bug from PR #1: the judge ran with nothing to hold the change against and the
// gate passed. Today the standard is an exemplar; none retrieved is missing evidence.
func TestIdiomLaneNeedsAnExemplar(t *testing.T) {
	in := Inputs{RepoDir: t.TempDir(), Diff: viewDiff(featureFile)}
	got := Check(Idiom, in)
	if got.OK() || !strings.Contains(strings.Join(got.Missing, ""), "exemplar") {
		t.Fatalf("no exemplar must be missing evidence, got %+v", got)
	}
	in.Exemplars = 1
	if got := Check(Idiom, in); !got.OK() {
		t.Errorf("one exemplar is evidence, got %+v", got)
	}
	in.Diff.Unified = ""
	if got := Check(Idiom, in); got.OK() {
		t.Error("an empty diff is missing evidence too")
	}
}

func TestIdiomLaneSkipsWhenNoSwiftChanged(t *testing.T) {
	got := Check(Idiom, Inputs{RepoDir: t.TempDir(), Diff: swiftDiff("docs/STATE.md")})
	if got.Applies() {
		t.Errorf("a docs-only PR has nothing for the idiom lane, got %+v", got)
	}
}

// The trigger (ADR 0021): the lane runs where structural translation happens — a View,
// an @Observable type, or a whole new file under a feature target — and skips the rest
// with the reason recorded, so `swiftgate metrics` can still count how often it was
// eligible. It is the lane expected to fail the grant rule; this is what bounds its cost.
func TestIdiomLaneAppliesOnlyWhereStructureIsAdded(t *testing.T) {
	line := func(text string) scan.Diff {
		d := swiftDiff(featureFile)
		d.Files[0].Added = []scan.AddedLine{{Number: 1, Text: text}}
		return d
	}
	newFile := func(path string) scan.Diff {
		d := swiftDiff(path)
		d.Files[0].Status = "A"
		return d
	}

	cases := []struct {
		name    string
		diff    scan.Diff
		applies bool
	}{
		{"adds a View", line("struct MoodBadge: View {"), true},
		{"adds a View with other conformances", line("struct MoodBadge: Identifiable, View {"), true},
		{"adds an @Observable type", line("@Observable"), true},
		{"adds an indented @Observable", line("    @Observable @MainActor"), true},
		{"a new file under a feature target", newFile("Packages/MindlensKit/Sources/Features/Dashboard/Row.swift"), true},

		{"a repository change adds neither", line("    func load() async throws -> [Entry] {"), false},
		{"a word that merely contains View", line("    let previewMode = true"), false},
		{"a View mentioned but not declared", line("    // returns some View"), false},
		{"a new file outside a feature target", newFile("Packages/MindlensKit/Sources/Networking/Endpoint.swift"), false},
		// A test file that happens to declare a view is not where translation lands,
		// and judging test scaffolding against production exemplars is noise.
		{"a View added in a test", func() scan.Diff {
			d := swiftDiff("Packages/MindlensKit/Tests/DashboardTests/HostTests.swift")
			d.Files[0].Added = []scan.AddedLine{{Number: 1, Text: "struct Host: View {"}}
			return d
		}(), false},
	}
	for _, c := range cases {
		got := Check(Idiom, Inputs{RepoDir: t.TempDir(), Diff: c.diff, Exemplars: 1})
		if got.Applies() != c.applies {
			t.Errorf("%s: applies = %v, want %v (%+v)", c.name, got.Applies(), c.applies, got)
		}
		if !c.applies && !strings.Contains(got.Skipped, "no view or model added") {
			t.Errorf("%s: a skip must say why, so metrics can count it; got %q", c.name, got.Skipped)
		}
	}
}

func TestSpecLaneReadsTheFeatureFileOffTheBranchName(t *testing.T) {
	repo := t.TempDir()
	in := Inputs{RepoDir: repo, Diff: swiftDiff(featureFile), Branch: "feature/auth"}

	got := Check(Spec, in)
	if got.Applies() || !strings.Contains(got.Skipped, "docs/features/auth.md") {
		t.Fatalf("a feature branch with no feature file is tooling: skipped, naming the file, got %+v", got)
	}

	if err := os.MkdirAll(filepath.Join(repo, "docs/features"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(repo, "docs/features/auth.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("## Steps\n\n1. ✅ done\n2. ✅ also done\n")
	if got := Check(Spec, in); got.OK() {
		t.Error("every step ticked leaves the spec lane nothing to judge against")
	}
	write("## Steps\n\n1. ✅ done\n2. 🟡 **In progress**\n")
	if got := Check(Spec, in); !got.OK() {
		t.Errorf("an open step is the evidence, got %+v", got)
	}

	for _, branch := range []string{"bugfix/x", "hotfix/x", "docs/x", "", "feature/"} {
		if got := Check(Spec, Inputs{RepoDir: repo, Branch: branch}); got.Applies() {
			t.Errorf("%q is not a feature branch; the spec lane should skip, got %+v", branch, got)
		}
	}
}

func TestVerificationLaneNeedsATestTargetPerTouchedModule(t *testing.T) {
	repo := t.TempDir()
	in := Inputs{RepoDir: repo, Diff: swiftDiff(featureFile, "Packages/MindlensKit/Sources/Networking/APIClient.swift")}

	touch(t, repo, "Packages/MindlensKit/Tests/DashboardTests/DashboardModelTests.swift")
	got := Check(Verification, in)
	if got.OK() || len(got.Missing) != 1 || !strings.Contains(got.Missing[0], "NetworkingTests") {
		t.Fatalf("Networking has no test target and that must be named, got %+v", got)
	}
	if len(got.Present) != 1 || !strings.Contains(got.Present[0], "DashboardTests") {
		t.Errorf("the target that is there should be listed as present, got %+v", got)
	}

	touch(t, repo, "Packages/MindlensKit/Tests/NetworkingTests/APIClientTests.swift")
	if got := Check(Verification, in); !got.OK() {
		t.Errorf("both targets present, got %+v", got)
	}
}

// The Keychain has no home in a package test bundle on the simulator, so Persistence is
// tested hosted by the app (ADR 0026). A PersistenceTests directory would never exist.
func TestVerificationLaneLooksForPersistenceInTheAppHostedBundle(t *testing.T) {
	repo := t.TempDir()
	in := Inputs{RepoDir: repo, Diff: swiftDiff("Packages/MindlensKit/Sources/Persistence/KeychainItem.swift")}

	got := Check(Verification, in)
	if got.OK() || len(got.Missing) != 1 || !strings.Contains(got.Missing[0], "mindlensTests") {
		t.Fatalf("with mindlensTests empty, that is what is missing, got %+v", got)
	}

	touch(t, repo, "mindlensTests/KeychainTokenStorageTests.swift")
	if got := Check(Verification, in); !got.OK() {
		t.Errorf("mindlensTests holds Persistence's tests, got %+v", got)
	}
}

func TestVerificationLaneSkipsTestOnlyAndAppTargetIsUITests(t *testing.T) {
	repo := t.TempDir()
	testsOnly := swiftDiff("Packages/MindlensKit/Tests/CoreTests/AppErrorTests.swift", "Packages/MindlensKit/Sources/TestSupport/Fakes.swift")
	if got := Check(Verification, Inputs{RepoDir: repo, Diff: testsOnly}); got.Applies() {
		t.Errorf("a tests-only change has no production path to verify, got %+v", got)
	}

	app := swiftDiff("mindlens/RootView.swift")
	if got := Check(Verification, Inputs{RepoDir: repo, Diff: app}); got.OK() {
		t.Error("the app target's tests live in mindlensUITests; absent, that is missing")
	}
	touch(t, repo, "mindlensUITests/LaunchSmokeTests.swift")
	if got := Check(Verification, Inputs{RepoDir: repo, Diff: app}); !got.OK() {
		t.Errorf("mindlensUITests present, got %+v", got)
	}
}

func TestParseAndAll(t *testing.T) {
	if l, ok := Parse(" Idiom "); !ok || l != Idiom {
		t.Errorf("Parse should be forgiving about case and space, got %q %v", l, ok)
	}
	if _, ok := Parse("style"); ok {
		t.Error("an unknown lane must not parse")
	}
	all := All(Inputs{RepoDir: t.TempDir()})
	if len(all) != len(Lanes) {
		t.Errorf("All checks every lane, got %d", len(all))
	}
	if got := Check("style", Inputs{}); got.OK() {
		t.Error("an unknown lane has missing evidence by definition")
	}
}
