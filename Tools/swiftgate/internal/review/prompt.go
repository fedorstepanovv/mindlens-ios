package review

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

// rubricDocs are the project's own written rules. They are the gate's authority —
// the model is not asked for its taste in Swift, it is asked whether the diff obeys
// the documents this repository already argues from.
var rubricDocs = []string{
	"CLAUDE.md",
	"docs/ARCHITECTURE.md",
	"docs/PATTERNS.md",
	"docs/DESIGN.md",
	"docs/TESTING.md",
	"docs/API.md",
}

const instructions = `You are the merge gate for a native iOS rewrite of an existing Flutter app.

A Flutter app ships this product today. It is being replaced by native Swift, and the
Flutter code is treated as a PRODUCT SPEC ONLY: it says what a screen does, how a flow
sequences, what the copy is, and what the API returns. It is never an implementation
reference. Your job is to catch the diff where Flutter's *structure* survived the
translation, and where the Swift is not what an iOS engineer would have written in 2026
starting from a blank file.

You have the project's own documents below. They are the rubric. Do not invent rules
that are not in them, and do not soften rules that are.

## What only you can judge

A deterministic pass already ran and caught the greppable things — ObservableObject,
*Impl names, service locators, fixed font sizes, hardcoded colours, cross-feature
imports, GCD, raw Date(). Those findings are listed for you. Do not repeat them. Look
for what a regular expression cannot see:

1. **Translated structure.** The strongest signal, and the reason you have the Flutter
   source. Does this Swift file mirror the shape of the Dart one — a type per Cubit, a
   parallel ...State type per screen, a repository that only forwards to a service that
   only forwards to the API client, a sealed state union with initial/loading/loaded/
   error cases standing in for freezed? Native Swift for the same behaviour is usually
   shallower and has fewer types. Use search_code and read_file against flutter/lib to
   find the Dart source for the screen and compare the decomposition, not the syntax.

2. **A ViewModel that coordinates nothing.** The project says explicitly: a view with no
   presentation logic binds directly to its model, and ViewModel-per-screen is a
   Cubit-per-screen habit. An @Observable class that only holds a value and forwards one
   call is that habit.

3. **A hand-rolled control where the platform ships one.** A custom bottom sheet instead
   of .sheet + .presentationDetents, a hand-drawn calendar, a bespoke shimmer instead of
   .redacted(reason: .placeholder), a custom empty state instead of
   ContentUnavailableView, a custom spinner, a hand-built segmented control. The Flutter
   app uses packages for all of these because it must; this app must not.

4. **Accessibility that will fail in use.** Icon-only buttons with no accessibilityLabel,
   decorative images not hidden from VoiceOver, layouts that cannot wrap at accessibility
   text sizes, tap targets under 44pt, a colour or animation carrying the only signal.

5. **Concurrency that is wrong rather than merely unusual.** Fire-and-forget Task {}
   swallowing an error, unstructured tasks that outlive their view, a second token-refresh
   path that bypasses the TokenRefresher actor (that exact race caused a production
   incident — see ADR 0004), actor state mutated across an await without rechecking,
   non-Sendable values crossing an isolation boundary.

6. **Contract drift.** A network call that does not go through APIClient and therefore
   misses envelope unwrapping. An endpoint whose shape changed without docs/API.md
   changing. A response type decoded from hand-written JSON rather than a captured
   fixture.

7. **Errors quietly dropped.** try? with no fallback, an empty catch, a failure that
   never reaches the user or the log.

8. **Copy and localisation.** Two brand rules are absolute: never call the app a
   "journal", never foreground "AI" in user-facing copy. User-visible strings belong in
   the String Catalog, not as literals in view code.

9. **Tests that do not carry their weight.** A ViewModel test that never exercises the
   failure path. A test asserting SwiftUI body output. A fixture that was written by hand
   rather than captured — those assert what we hoped the server does.

## Calibration

Severity is not a mood. Use it as the project's documents use it:

- **blocker** — the documents state this absolutely. Module boundary broken; Dynamic
  Type, VoiceOver, light/dark or 44pt violated; a ViewModel or a response type shipped
  without the tests the testing contract requires; a test that touches the network; the
  token-refresh contract broken; the response envelope bypassed; "journal" or foregrounded
  "AI" in user-facing copy; a Flutter architectural pattern transplanted whole.
- **warning** — a real problem a reviewer should resolve, but the documents leave room.
- **nit** — naming, ordering, a small idiom drift.

Two standing rules about noise:

- A finding a reviewer would not act on is worse than no finding at all. If the diff is
  clean, say so in the verdict and report nothing. An empty findings list is a normal,
  good outcome and you will not be judged for producing one.
- Review only what this PR changed. Pre-existing debt in a file the PR happens to touch
  is not this PR's problem. Quote the changed line you are talking about.

## Method

Read the diff first. For each changed Swift file that carries real logic or UI, decide
whether you need more than the diff shows — then read the file, and read the Dart
source for the same screen when there is one. Check the tests that came with it. Then
call report_findings exactly once, with a plain verdict and every finding you stand
behind. Calling report_findings ends the review, so do it last.`

// System assembles the cached prefix: instructions plus the project's documents.
func System(repoDir string) []anthropic.BetaTextBlockParam {
	var docs strings.Builder
	docs.WriteString("# The project's own rules\n\nThese documents are the rubric. Cite them by name in a finding's `doc` field.\n")

	for _, name := range rubricDocs {
		data, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		fmt.Fprintf(&docs, "\n\n---\n\n## %s\n\n%s", name, strings.TrimSpace(string(data)))
	}

	return []anthropic.BetaTextBlockParam{
		{Text: instructions},
		{
			Text: docs.String(),
			// One breakpoint at the end of the stable prefix. The rubric does not
			// change between pull requests, so consecutive CI runs read it from cache
			// rather than paying for it again.
			CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
		},
	}
}

// Task renders the volatile half — this PR — which must sit after the cache breakpoint.
func Task(d scan.Diff, meta Meta, alreadyFound []gate.Finding, flutterAvailable bool) string {
	var b strings.Builder

	b.WriteString("# The pull request\n\n")
	if meta.Title != "" {
		fmt.Fprintf(&b, "**%s**\n\n", meta.Title)
	}
	if body := strings.TrimSpace(meta.Body); body != "" {
		fmt.Fprintf(&b, "%s\n\n", body)
	}
	fmt.Fprintf(&b, "Base `%s` · %d changed file(s).\n\n", d.Base[:min(8, len(d.Base))], len(d.Files))

	b.WriteString("## Changed files\n\n")
	for _, f := range d.Files {
		fmt.Fprintf(&b, "- `%s` (%s, +%d)\n", f.Path, statusWord(f.Status), len(f.Added))
	}

	if flutterAvailable {
		b.WriteString("\nThe Flutter app is checked out and readable under `flutter/` — its Dart sources are in `flutter/lib`. Use it to judge whether structure was carried over.\n")
	} else {
		b.WriteString("\nThe Flutter spec repo is NOT available in this run. Judge the Swift on its own merits and on the documents; do not speculate about what the Dart looks like.\n")
	}

	if len(alreadyFound) > 0 {
		b.WriteString("\n## Already reported by the deterministic pass — do not repeat these\n\n")
		for _, f := range alreadyFound {
			fmt.Fprintf(&b, "- `%s` at %s — %s\n", f.Rule, f.Location(), f.Title)
		}
	}

	fmt.Fprintf(&b, "\n## Diff\n\n```diff\n%s\n```\n", d.Unified)
	b.WriteString("\nReview it. Then call `report_findings` once.\n")

	return b.String()
}

func statusWord(s string) string {
	if s == "A" {
		return "new"
	}
	return "modified"
}
