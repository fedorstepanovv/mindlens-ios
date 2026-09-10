# Patterns

Canonical shapes for this codebase. New code matches these. Deviating is fine when the
situation warrants it — say why in the PR or an ADR.

## ViewModels

`@Observable`, `@MainActor`, plain properties. No `ObservableObject`, no `@Published`,
no sealed state unions.

```swift
@Observable
@MainActor
final class DashboardModel {
    private(set) var days: [DaySummary] = []
    private(set) var isLoading = false
    private(set) var error: AppError?

    var selectedDate: Date { didSet { Task { await loadSelectedDay() } } }

    private let moods: any MoodRepository
    private let analytics: any AnalyticsRecording

    init(moods: any MoodRepository, analytics: any AnalyticsRecording, today: Date = .now) {
        self.moods = moods
        self.analytics = analytics
        self.selectedDate = today
    }

    func loadSelectedDay() async {
        isLoading = true
        defer { isLoading = false }
        do {
            days = try await moods.summaries(around: selectedDate)
            error = nil
        } catch {
            self.error = AppError(error)
        }
    }
}
```

Owned by the view that creates it:

```swift
struct DashboardView: View {
    @State private var model: DashboardModel
    var body: some View { ... .task { await model.loadSelectedDay() } }
}
```

### When *not* to write a ViewModel

If a view has no state to coordinate, it takes its model directly. Manufacturing a
ViewModel for this is a Cubit-per-screen habit:

```swift
// Right.
struct MoodBadge: View {
    let mood: Mood
    var body: some View { Label(mood.title, systemImage: mood.symbolName) }
}
```

## Repositories

Protocol at the seam, concrete type behind it. Named for what it *is*, never `...Impl`.

```swift
public protocol MoodRepository: Sendable {
    func summaries(around date: Date) async throws -> [DaySummary]
    func log(_ mood: Mood, at date: Date) async throws
}

public struct LocalFirstMoodRepository: MoodRepository {
    private let store: MoodStore        // SwiftData — source of truth
    private let api: APIClient
    private let outbox: Outbox

    public func summaries(around date: Date) async throws -> [DaySummary] {
        let cached = try await store.summaries(around: date)
        Task { try? await sync(around: date) }   // revalidate, don't block the view
        return cached
    }

    public func log(_ mood: Mood, at date: Date) async throws {
        try await store.insert(mood, at: date)   // optimistic, immediately visible
        await outbox.enqueue(.logMood(mood, date))
    }
}
```

Repositories return domain models. DTOs never escape `Networking`.

## Networking

Endpoints are typed values. The response type is baked into the endpoint, so call sites
can't decode the wrong thing.

```swift
public struct Endpoint<Response: Decodable & Sendable>: Sendable {
    public var method: HTTPMethod
    public var path: String
    public var query: [URLQueryItem] = []
    public var body: Data?
    public var requiresAuth = true
}

extension Endpoint where Response == MoodRecordDTO {
    static func logMood(_ rate: Int) throws -> Self {
        try .post("/mood", body: ["moodRate": rate])
    }
}

let mood = try await api.send(.logMood(3))   // returns MoodRecordDTO
```

Every response is enveloped, and the client unwraps it in one place so no call site ever
sees `data`:

```swift
struct APIEnvelope<T: Decodable>: Decodable {
    let data: T
    let success: Bool
}
```

## Token refresh

The API rotates refresh tokens and invalidates the old one on use. Two requests hitting
401 at once will therefore race, and the loser's refresh token is already dead — that is
the production incident from 2026-07-12. An actor makes single-flighting structural:

```swift
public actor TokenRefresher {
    private var inFlight: Task<Tokens, Error>?

    public func refreshed(replacing stale: Tokens) async throws -> Tokens {
        if let inFlight { return try await inFlight.value }   // join the existing refresh

        let task = Task { try await self.performRefresh(using: stale.refresh) }
        inFlight = task
        defer { inFlight = nil }
        return try await task.value
    }
}
```

Classifying the failure is as important as single-flighting it. Treating every failed
refresh as "log the user out" is what caused the incident:

```swift
enum RefreshFailure: Error {
    case sessionEnded            // 400/401/403 — token genuinely dead. Sign out.
    case temporarilyUnavailable  // timeout, 5xx, decode failure. Keep the session.
}
```

## Combine, used deliberately

Combine earns its place where a stream needs operators. It does not carry data between
layers — that's `async/await`'s job.

```swift
// Justified: debounce + dedupe on user typing.
$query
    .debounce(for: .milliseconds(300), scheduler: DispatchQueue.main)
    .removeDuplicates()
    .sink { [weak self] in self?.search($0) }
    .store(in: &cancellables)
```

Not justified: wrapping a single network call in a `Future`, or turning a one-shot load
into a publisher chain. Use `async/await`.

## Dates and timezones

The API takes `date` + IANA `timezone` on most reads and stores UTC. "Today" is a
*local* day and the boundary is the user's timezone, not the device's UTC offset. All of
it goes through one helper in `Core` — never `Calendar.current` scattered through
features, and never a raw `Date()` in a type you want to test.

```swift
public protocol DateProvider: Sendable { var now: Date { get } }  // injected; tests control time
```
