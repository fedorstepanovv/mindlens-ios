# Testing

Swift Testing (`@Test`, `#expect`, `@Suite`) for everything except UI automation, which
still requires XCTest.

## What must be tested

Non-negotiable — a feature is not done without these:

1. **Every ViewModel.** Loading, success, failure, and empty. A ViewModel with untested
   error handling is a ViewModel with broken error handling.
2. **Every API response type, against a real captured JSON fixture.** There is no
   OpenAPI spec for this backend. These fixtures are the only thing standing between us
   and a silent contract break. Capture real responses; do not hand-write optimistic JSON.
3. **The token refresher, under concurrency.** Specifically: N simultaneous 401s produce
   exactly one refresh call, and a transient failure does not end the session.
4. **Date and timezone logic.** Local-day boundaries, DST transitions, a user whose
   timezone differs from the device.
5. **Every bug fix.** Named for the bug, with a comment linking the cause. This habit
   already exists in the Flutter repo and is the best testing practice in it.

## What not to test

Don't test SwiftUI body output, Apple's frameworks, or trivial forwarding. Don't chase a
coverage number — coverage as a target produces tests that assert nothing.

## Rules

- **No test touches the network.** Ever. Fakes live in `TestSupport`.
- **No test depends on the current date.** Inject `DateProvider`.
- Tests are parallel-safe by default. Shared mutable state is a bug, not a config problem.
- Fixtures live in `Packages/MindlensKit/Sources/TestSupport/Fixtures` as real `.json`
  files, with a README recording which are captured and which are still provisional.

## Shapes

```swift
@Suite("Dashboard")
struct DashboardModelTests {

    @Test("surfaces an error when the repository fails")
    func showsError() async {
        let model = DashboardModel(moods: FailingMoodRepository(), analytics: .noop)
        await model.loadSelectedDay()
        #expect(model.error != nil)
        #expect(model.days.isEmpty)
    }

    @Test("logs a mood optimistically before the network confirms")
    func optimisticLog() async throws { ... }
}
```

Concurrency, which is where the real bugs live:

```swift
@Test("concurrent 401s trigger exactly one refresh")
func singleFlight() async throws {
    let server = CountingRefreshServer()
    let refresher = TokenRefresher(server: server)

    await withTaskGroup(of: Void.self) { group in
        for _ in 0..<20 { group.addTask { _ = try? await refresher.refreshed(replacing: .stale) } }
    }

    #expect(server.refreshCallCount == 1)
}
```

Decoding, against a real response:

```swift
@Test("decodes the enveloped mood response")
func decodesMood() throws {
    let json = try Fixture.data("mood_create_200.json")
    let envelope = try JSONDecoder.api.decode(APIEnvelope<MoodRecordDTO>.self, from: json)
    #expect(envelope.data.mood == 3)
}
```

## UI tests

XCUITest, kept deliberately small — three flows, not thirty: cold launch to signed-in
dashboard, logging a mood end to end, and the paywall appearing when it should. UI tests
are slow and flaky in proportion to their number; the unit suite carries the load.

## Accessibility

Treated as a test surface, not a review checklist. Every screen is verified at the
largest Dynamic Type size and with VoiceOver labels present on interactive elements.
