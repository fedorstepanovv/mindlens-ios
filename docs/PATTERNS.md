# Patterns

Canonical shapes for this codebase.

**Rule for this file:** it holds *shapes and rules*. Where a pattern has a security- or
correctness-critical implementation, this file **points at the real file** instead of
copying it. A second copy of an algorithm in a doc is a copy that will drift — and when
it drifts, the doc quietly teaches the broken version.

## ViewModels

`@Observable`, `@MainActor`, plain properties. No `ObservableObject`, no `@Published`,
no sealed state unions.

Real example: `Packages/MindlensKit/Sources/Features/Dashboard/DashboardModel.swift`

```swift
@Observable
@MainActor
public final class DashboardModel {
    public private(set) var summaries: [DaySummary] = []
    public private(set) var isLoading = false
    public private(set) var error: AppError?

    public var selectedDate: Date

    private let moods: any MoodRepository

    public func load() async { ... }
}
```

### Loads are driven by the view, not by a property setter

```swift
// Right — SwiftUI cancels the previous load when the id changes.
.task(id: model.selectedDate) { await model.load() }
```

```swift
// Wrong — this is Cubit `emit`-on-set thinking.
var selectedDate: Date { didSet { Task { await load() } } }
```

The `didSet` version spawns an uncancelled task per change. Scrub a date picker across
five days and you get five overlapping loads: `summaries` ends up showing whichever
*finished* last rather than what the user selected, and the first task's
`defer { isLoading = false }` clears the spinner while four are still running.

`.task(id:)` gives cancellation, coalescing and lifecycle for free. Reach for it before
inventing anything.

### Cancellation is not an error

A cancelled load must not paint an error banner — that produces the classic flash-of-error
when navigating away.

```swift
} catch is CancellationError {
    // The view moved on.
} catch {
    self.error = AppError(error)
}
```

### When *not* to write a ViewModel

A view with no presentation state takes its model directly. Manufacturing a ViewModel for
every screen is a Cubit-per-screen habit:

```swift
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
    func log(_ mood: Mood, on date: Date) async throws
}
```

Repository protocols live in `Models`, **not** in a feature — that is what lets one
`TestSupport` serve every test target, and what stops two features needing each other.

Repositories return domain models. DTOs never escape `Networking`.

> ⚠️ **Open design point.** `ARCHITECTURE.md` says the local store is "the source of truth
> the UI observes," but this protocol returns a *snapshot*. Nothing observes anything yet,
> so a background sync writes to the store and the UI does not notice. Resolving this is
> Stage 2 work — see `docs/STATE.md`. Documented here rather than glossed over.

## Networking

Endpoints are typed values; the response type is baked in, so a call site cannot decode
the wrong thing.

```swift
public struct Endpoint<Response: Decodable & Sendable>: Sendable { ... }

extension Endpoint where Response == MoodRecordDTO {
    static func logMood(_ rating: Mood) throws -> Self {
        try .post("/mood", body: LogMoodRequest(moodRate: rating.rawValue))
    }
}
```

Request bodies are **typed structs, never dictionaries**. `["moodRate": rate]` is a
`Map<String, dynamic>` habit, and the server rejects unknown fields outright — so the
compiler should be the thing that catches a typo, not a 400 in production.

Envelope unwrapping happens once, in `APIClient`, so no call site sees `data`.

`APIClient` is a `Sendable final class`, **not** an actor — all its properties are `let`,
so an actor would add an executor hop per request and protect nothing. Actors are for
shared mutable state; `TokenRefresher` has some, `APIClient` does not.

## Token refresh

→ **`Packages/MindlensKit/Sources/Networking/TokenRefresher.swift`**

Read the real file. It is deliberately not reproduced here.

The two properties that matter, both tested in `TokenRefresherTests`:

1. **Single-flight** — concurrent callers join one in-flight refresh.
2. **Generation guard** — a request that was *already in flight* when someone else
   refreshed comes back 401 later. It must not refresh again: its token is stale by
   definition, and the API rotates refresh tokens single-use, so refreshing again burns a
   live token and signs the user out.

Single-flighting alone is not enough, and getting only half of it is what caused the
2026-07-12 production incident. Failure classification is the other half: **400/401/403**
means the session is over, everything else is transient and must keep the session.

## Dates and timezones

Three wire formats, so three types — one global `dateDecodingStrategy` cannot serve all of
them (`Packages/MindlensKit/Sources/Core/CalendarDate.swift`):

| Wire | Type |
|---|---|
| `2026-09-10T18:22:41.512Z` | `Date`, via an explicit fractional-seconds strategy |
| `2026-09-10` | `CalendarDate` |
| `09:05` | `TimeOfDay` |

"Today" is a **local** day, and which day an instant belongs to depends on the user's
timezone — so `CalendarDate(instant, in:)` requires a zone and never defaults one. Never
scatter `Calendar.current` through features, and never put a bare `Date()` in a type you
want to test — inject `DateProvider`.

## Concurrency

`async/await` by default. Actors for shared mutable state. `Mutex` (SE-0433) for a simple
counter or flag — available on our iOS 18 floor and cheaper than an actor.

**Combine** is used where a stream genuinely needs operators over time, and it does not
carry data between layers. Note that its two classic justifications have both moved:

- Debounced input → `.task(id:)`, which debounces *and* cancels, with no `cancellables` bag.
- `NotificationCenter` → already an `AsyncSequence` via `.notifications(named:)`.

Also note `@Observable` has **no** `$property` projected value, so `$query.debounce(...)`
does not compile in this codebase. The honest remaining cases for Combine are third-party
SDKs that only vend publishers, and interop with legacy `@Published` code. If you reach
for it outside those, justify it in the PR.

## Errors

One `AppError` in `Core`, carrying `kind` + a diagnostic string for logs. It deliberately
holds **no user-facing copy** — words are presentation and live in `DesignSystem`, the
only target with a String Catalog.

Any `String(localized:)` inside a package target **must** pass `bundle: .module`, or it
resolves against `Bundle.main` and silently never localizes.
