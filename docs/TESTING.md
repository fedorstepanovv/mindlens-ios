# Testing

Swift Testing (`@Test`, `#expect`, `@Suite`) for everything except UI automation and
performance measurement, which still require XCTest (`XCTMetric`/`measure` have no Swift
Testing equivalent).

`swift test` runs from the CLI in about two seconds — the package declares a macOS
platform so the platform-agnostic targets build for the host. Use it for the fast loop;
use `xcodebuild -destination 'platform=iOS Simulator,…'` before committing.

## What must be tested

A feature is not done without these:

1. **Every ViewModel** — loading, success, failure, empty.
2. **Every API response type, against a real captured fixture.** There is no OpenAPI spec.
   These are the only contract guard. Refresh them with `Tools/capture-fixtures.sh`.
3. **The token refresher, under concurrency** — and not just the easy case. See below.
4. **Date and timezone logic** — local-day boundaries, a user whose timezone differs from
   the device's, DST transitions.
5. **Every bug fix**, tagged with `.bug(...)` and named for the failure.

## Tests run in parallel by default

Swift Testing parallelises **in-process, including within a suite**. XCTest does not, so
this catches people out.

Any shared mutable state across tests is a bug. This is not hypothetical — the first
version of `StubURLProtocol` in this repo kept one static handler, and parallel tests
overwrote each other's stub, producing failures that looked like product bugs. The fix
was to key handlers per session.

If a suite genuinely cannot run in parallel, say so explicitly with `.serialized` rather
than hoping.

## A suite testing a `@MainActor` type is itself `@MainActor`

The single most common Swift 6 testing stumble:

```swift
@Suite("Dashboard")
@MainActor                      // ← without this, three compile errors
struct DashboardModelTests {

    @Test("surfaces an error when loading fails")
    func showsError() async {
        let model = DashboardModel(moods: StubMoodRepository(error: AppError(kind: .offline)))
        await model.load()
        #expect(model.error?.kind == .offline)
    }
}
```

## Useful things the framework gives you

- `#require` — unwrap or fail the test, instead of `!`.
- `@Test(arguments:)` — one test, many cases. Right shape for a timezone or status-code matrix.
- `.timeLimit(.minutes(1))` — put this on concurrency tests. Without it a deadlock hangs CI.
- `.bug("…")` — the first-class form of "every bug fix ships with a regression test".
- `.serialized`, `.tags(…)`, `.enabled(if:)`, `withKnownIssue`, `Issue.record`.
- `confirmation()` — asserts an event fired exactly N times, without a shared counter.

## Concurrency tests must assert the hard case

A single-flight test that fires N callers simultaneously at a slow transport is nearly
tautological — it passes against implementations that are still broken, because perfectly
simultaneous arrival is the easy case.

The case that actually breaks in production is **staggered**: a request that was already
in flight when someone else refreshed, coming back 401 afterwards. See
`Packages/MindlensKit/Tests/NetworkingTests/TokenRefresherTests.swift` — specifically
`staleGenerationShortCircuits` and `signOutDuringRefreshDiscardsResult`, which are the
two tests that would have caught real bugs.

## No test touches the network

The mechanism is `StubURLProtocol` in `TestSupport`: a `URLProtocol` on an ephemeral
`URLSessionConfiguration`, so nothing leaks between tests and no request ever leaves the
process.

```swift
let session = StubURLProtocol.session(status: 401, counter: requests)
let client = APIClient(baseURL: url, session: session, refresher: refresher)
```

Fakes live in `TestSupport`, and the protocols they implement live in **shared** targets,
never in features — otherwise `TestSupport` would have to depend on a feature.

## What not to test

Don't test SwiftUI body output, Apple's frameworks, or trivial forwarding. Don't chase a
coverage number.

And beware tests that cannot fail. `#expect(SessionState.restoring != .signedOut)` compares
two distinct cases of a synthesised `Equatable` enum — it asserts nothing about behaviour.
If the property you care about is "launch does not flash the sign-in screen," that is a
view-level test.

## UI tests

XCUITest, deliberately small — cold launch to signed-in dashboard, logging a mood, and the
paywall appearing when it should. UI tests are flaky in proportion to their number.

## Accessibility

Mechanised, not reviewed by eye:

- `XCUIApplication().performAccessibilityAudit()` — catches contrast, hit-region,
  clipped-text-at-large-sizes and missing labels automatically.
- Launch under `-UIPreferredContentSizeCategoryName UICTContentSizeCategoryAccessibilityExtraExtraExtraLarge`
  to prove layouts survive the largest Dynamic Type sizes.
