package scan

import "testing"

func TestRankPrefersKindThenNameThenNeighbourhood(t *testing.T) {
	target := describe("Packages/MindlensKit/Sources/Features/Insights/InsightsView.swift",
		"import SwiftUI\nstruct InsightsView: View { var body: some View { Text(\"\") } }\n")
	pool := []candidate{
		describe("Packages/MindlensKit/Sources/Features/Authentication/SignInView.swift", "struct SignInView: View { var body: some View { EmptyView() } }"),
		describe("Packages/MindlensKit/Sources/Features/Dashboard/DashboardModel.swift", "@Observable final class DashboardModel {}"),
		describe("mindlens/RootView.swift", "struct RootView: View { var body: some View { EmptyView() } }"),
		describe("Packages/MindlensKit/Sources/Networking/HTTPMethod.swift", "enum HTTPMethod: String { case get }"),
		describe("Packages/MindlensKit/Sources/Features/Insights/InsightsModel.swift", "@Observable final class InsightsModel {}"),
	}

	got := rank(target, pool, 3)
	if len(got) != 3 {
		t.Fatalf("want three exemplars, got %d", len(got))
	}
	// Two views tie on kind and suffix; the ordering between them is by path. Then the
	// model in the same directory beats the model in another feature.
	want := []string{
		"Packages/MindlensKit/Sources/Features/Authentication/SignInView.swift",
		"mindlens/RootView.swift",
		"Packages/MindlensKit/Sources/Features/Insights/InsightsModel.swift",
	}
	for i, w := range want {
		if got[i].path != w {
			t.Errorf("rank[%d] = %s, want %s", i, got[i].path, w)
		}
	}
}

func TestRankDropsFilesWithNoResemblance(t *testing.T) {
	target := describe("Packages/MindlensKit/Sources/Networking/HTTPMethod.swift", "enum HTTPMethod { case get }")
	pool := []candidate{describe("mindlens/RootView.swift", "struct RootView: View {}")}
	if got := rank(target, pool, 3); len(got) != 0 {
		t.Errorf("a view is no exemplar for a bare enum in another module, got %v", got)
	}
}

func TestDescribeReadsKindsModuleAndSuffix(t *testing.T) {
	c := describe("Packages/MindlensKit/Sources/Persistence/KeychainTokenStorage.swift",
		"public protocol TokenStorage {}\nactor KeychainTokenStorage { let item = SecItemCopyMatching }")
	if !c.kinds["protocol"] || !c.kinds["actor"] || !c.kinds["keychain"] || c.kinds["view"] {
		t.Errorf("kinds misread: %v", c.kinds)
	}
	if c.module != "Persistence" || c.suffix != "Storage" {
		t.Errorf("module %q suffix %q", c.module, c.suffix)
	}
	if describe("mindlens/AppContainer.swift", "").module != "app" {
		t.Error("the app target is its own neighbourhood")
	}
}
