package scan

import (
	"os"
	"strings"
	"testing"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
)

// file builds a ChangedFile whose added lines are the given source, starting at line 1.
func file(path string, src string) ChangedFile {
	f := ChangedFile{Path: path, Status: "A"}
	for i, line := range strings.Split(strings.TrimPrefix(src, "\n"), "\n") {
		f.Added = append(f.Added, AddedLine{Number: i + 1, Text: line})
	}
	return f
}

func rules(t *testing.T, f ChangedFile) map[string]gate.Finding {
	t.Helper()
	s := Static{RepoDir: "../../../..", Severities: Severities{}}
	out := map[string]gate.Finding{}
	for _, finding := range s.Run(Diff{Files: []ChangedFile{f}}) {
		if _, seen := out[finding.Rule]; !seen { // keep the first hit, so line assertions are stable
			out[finding.Rule] = finding
		}
	}
	return out
}

func TestFlagsObservableObject(t *testing.T) {
	got := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/DashboardModel.swift", `
final class DashboardModel: ObservableObject {
    @Published var isLoading = false
}
`))
	f, ok := got["flutter/observable-object"]
	if !ok {
		t.Fatalf("expected ObservableObject to be flagged, got %v", keys(got))
	}
	if f.Severity != gate.Blocker {
		t.Errorf("ObservableObject should block, got %s", f.Severity)
	}
	if f.Line != 1 {
		t.Errorf("expected line 1, got %d", f.Line)
	}
}

func TestFlagsFeatureImportingFeature(t *testing.T) {
	got := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/DashboardView.swift", `
import Authentication
import Models
`))
	if _, ok := got["arch/feature-imports-feature"]; !ok {
		t.Fatalf("expected cross-feature import to be flagged, got %v", keys(got))
	}
}

func TestAllowsFeatureImportingSharedTarget(t *testing.T) {
	got := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/DashboardView.swift", `
import Core
import Models
import DesignSystem
`))
	if _, ok := got["arch/feature-imports-feature"]; ok {
		t.Fatalf("shared targets must not be flagged as cross-feature imports")
	}
}

func TestFlagsThirdPartySDKInsidePackage(t *testing.T) {
	got := rules(t, file("Packages/MindlensKit/Sources/Features/Authentication/SignIn.swift", `
import FirebaseAuth
`))
	if _, ok := got["arch/third-party-import"]; !ok {
		t.Fatalf("expected Firebase import to be flagged, got %v", keys(got))
	}
}

func TestFlagsFixedFontSizeAndHardcodedColour(t *testing.T) {
	got := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/MoodRow.swift", `
Text("Hello").font(.system(size: 17))
    .foregroundStyle(Color(red: 0.1, green: 0.2, blue: 0.3))
`))
	for _, want := range []string{"design/fixed-font-size", "design/hardcoded-color"} {
		if _, ok := got[want]; !ok {
			t.Errorf("expected %s, got %v", want, keys(got))
		}
	}
}

func TestFlagsRawDateInFeatureButNotInCore(t *testing.T) {
	inFeature := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/DashboardModel.swift", `
    self.selectedDate = Date()
`))
	if _, ok := inFeature["core/raw-date-now"]; !ok {
		t.Errorf("expected raw Date() in a feature to be flagged, got %v", keys(inFeature))
	}

	inCore := rules(t, file("Packages/MindlensKit/Sources/Core/DateProvider.swift", `
    public var now: Date { Date() }
`))
	if _, ok := inCore["core/raw-date-now"]; ok {
		t.Error("Core is where Date() is allowed to live; it must not be flagged there")
	}
}

func TestIgnoresPatternsInsideComments(t *testing.T) {
	got := rules(t, file("Packages/MindlensKit/Sources/Core/Notes.swift", `
/// Deliberately shallower than the Flutter app's Cubit layering.
// We do not use ObservableObject or DispatchSemaphore anywhere.
`))
	if len(got) != 0 {
		t.Fatalf("prose mentioning a forbidden pattern must not fire: %v", keys(got))
	}
}

func TestWaiverSilencesNamedRuleOnly(t *testing.T) {
	got := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/Legacy.swift", `
let queue = DispatchQueue.main // swiftgate:allow swift/gcd-queue — UIKit interop, removed with the bridge
print("loaded")
`))
	if _, ok := got["swift/gcd-queue"]; ok {
		t.Error("a waiver naming the rule should silence it")
	}
	if _, ok := got["design/print-logging"]; !ok {
		t.Error("a waiver must not silence rules it does not name")
	}
}

func TestFlagsDartFileNaming(t *testing.T) {
	got := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/dashboard_cubit.swift", "\nimport Foundation\n"))
	if _, ok := got["flutter/dart-file-naming"]; !ok {
		t.Fatalf("expected Dart file naming to be flagged, got %v", keys(got))
	}
}

func TestFlagsServiceLocator(t *testing.T) {
	got := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/DashboardModel.swift", `
    let repository = DIContainer.resolve(MoodRepository.self)
`))
	f, ok := got["flutter/service-locator"]
	if !ok {
		t.Fatalf("expected the service locator to be flagged, got %v", keys(got))
	}
	if f.Severity != gate.Blocker {
		t.Errorf("a service locator is get_it in Swift clothing and must block, got %s", f.Severity)
	}
}

func TestFlagsOurSingletonButNotApples(t *testing.T) {
	ours := rules(t, file("Packages/MindlensKit/Sources/Core/Analytics.swift", `
public final class Analytics {
    public static let shared = Analytics()
}
`))
	f, ok := ours["arch/our-singleton"]
	if !ok {
		t.Fatalf("expected a declared .shared to be flagged, got %v", keys(ours))
	}
	if f.Line != 2 {
		t.Errorf("the finding should sit on the declaration, got line %d", f.Line)
	}

	apples := rules(t, file("Packages/MindlensKit/Sources/Networking/Transport.swift", `
    let session = URLSession.shared
    let center = NotificationCenter.default
`))
	if _, ok := apples["arch/our-singleton"]; ok {
		t.Error("using Apple's .shared is allowed; only declaring one is not")
	}
}

func TestFlagsFlutterTypeVocabulary(t *testing.T) {
	for _, src := range []string{
		"struct MoodBadgeWidget: View {}",
		"final class DashboardCubit {}",
		"class MoodBloc {}",
		"final class SettingsStateNotifier {}",
	} {
		got := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/A.swift", "\n"+src+"\n"))
		if _, ok := got["flutter/widget-suffix"]; !ok {
			t.Errorf("expected %q to be flagged as Flutter vocabulary, got %v", src, keys(got))
		}
	}

	native := rules(t, file("Packages/MindlensKit/Sources/Features/Dashboard/MoodBadge.swift", `
struct MoodBadge: View {}
@Observable final class DashboardModel {}
`))
	if _, ok := native["flutter/widget-suffix"]; ok {
		t.Error("a type named the SwiftUI way must not be flagged")
	}
}

// Every rule the deterministic pass can emit has a test in this file that names it.
// Nine rules once shipped with none and were deleted rather than tested; this is the
// guard that stops the next one from shipping on faith.
func TestEveryRuleHasATest(t *testing.T) {
	src, err := os.ReadFile("static_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range RuleIDs() {
		if !strings.Contains(string(src), `"`+id+`"`) {
			t.Errorf("rule %s has no test naming it: test it or delete it", id)
		}
	}
}

func TestParseHunkStart(t *testing.T) {
	for header, want := range map[string]int{
		"@@ -0,0 +1,25 @@":                 1,
		"@@ -12,3 +34,7 @@ func thing() {": 34,
		"@@ -5 +9 @@":                      9,
		"garbage":                          0,
	} {
		if got := parseHunkStart(header); got != want {
			t.Errorf("parseHunkStart(%q) = %d, want %d", header, got, want)
		}
	}
}

func TestParseUnifiedAssignsLineNumbers(t *testing.T) {
	raw := `diff --git a/A.swift b/A.swift
new file mode 100644
--- /dev/null
+++ b/A.swift
@@ -0,0 +1,2 @@
+import Foundation
+let x = 1
diff --git a/B.swift b/B.swift
--- a/B.swift
+++ b/B.swift
@@ -10,0 +11,1 @@
+let y = 2
`
	files := parseUnified(raw)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Status != "A" {
		t.Errorf("A.swift should be reported as added, got %q", files[0].Status)
	}
	if len(files[0].Added) != 2 || files[0].Added[1].Number != 2 {
		t.Errorf("unexpected added lines for A.swift: %+v", files[0].Added)
	}
	if len(files[1].Added) != 1 || files[1].Added[0].Number != 11 {
		t.Errorf("expected B.swift line 11, got %+v", files[1].Added)
	}
}

func keys(m map[string]gate.Finding) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
