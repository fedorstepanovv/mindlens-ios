# Mindlens iOS — Agent Guide

Native iOS rewrite of the Mindlens mood-tracking app. SwiftUI · Swift 6 · SPM · MVVM.

**Read `docs/STATE.md` first.** It is the only file that tells you what is currently
true. Everything else in `docs/` is stable reference and changes rarely.

---

## The one rule that matters most

The Flutter app at `../app/mindlensapp` is a **product spec, not an implementation
reference**. Read it to learn *what* a screen does, how a flow sequences, what the copy
says, what the API returns. Then close it and write idiomatic native iOS from first
principles.

Never port: `get_it` service location, Cubit/Bloc state machines, freezed sealed-union
state, mechanical repository→service→cubit layering, `go_router` global redirects,
barrel exports, `*Impl` type names, JSON codegen, or third-party widgets that replace
components UIKit/SwiftUI already ship.

Never imitate its UI. Flutter approximates iOS because it must. We *are* iOS.

## Architecture rules (enforced by the compiler)

- `Features/*` targets **may not import each other**. Cross-feature navigation goes
  through the app target via typed destinations. SPM enforces this — do not work around it.
- Features depend only on `Core`, `Models`, `Networking`, `Persistence`, `DesignSystem`,
  `Analytics`.
- The app target is thin: entry point, root scene, router, and the DI composition root.
  No business logic lives there.
- Dependencies are **constructor-injected**. There is no global container, no singleton
  registry, no `.shared` outside of Apple's own types.
- Every external service (analytics, purchases, push, crash reporting) sits behind a
  protocol defined in our code. SDK types never appear in feature code.

Full detail: `docs/ARCHITECTURE.md`. Code-level conventions: `docs/PATTERNS.md`.

## MVVM, applied honestly

A `@Observable @MainActor` ViewModel exists where there is real presentation state to
coordinate. A view with no presentation logic binds directly to its model — **do not
manufacture a ViewModel for every screen.** ViewModel-per-screen is a Cubit-per-screen
habit and it reads as translated Flutter.

## Concurrency

Swift 6 language mode, strict concurrency on. `async/await` is the default. Actors for
shared mutable state. Combine only where it is genuinely the right tool — debounced
input, `NotificationCenter` streams, multi-source merges — and never as the default way
to move data between layers.

## Design

Native components, always. `.sheet` + `.presentationDetents`, `List`/`Form`,
`ContentUnavailableView`, `.redacted(reason: .placeholder)`, SF Symbols, Swift Charts,
system materials and semantic colors.

Non-negotiable: **Dynamic Type throughout** (no fixed point sizes), VoiceOver labels on
every interactive element, light and dark both correct. Rules: `docs/DESIGN.md`.

## Testing

Swift Testing (`@Test`/`#expect`) for units. XCTest only where XCUITest requires it.

Required before a feature is done:
- Every ViewModel has tests.
- Every API response type has a decoding test against a real captured JSON fixture.
  There is no OpenAPI spec — these fixtures are the only contract guard we have.
- Every bug fix ships with a regression test named for the bug.

No test touches the network. Fakes live in `TestSupport`. Details: `docs/TESTING.md`.

## Backend

NestJS API, no versioning, no `/api` prefix, **no OpenAPI spec** — `docs/API.md` is the
only written contract. Keep it current or it becomes a lie.

Two things that will silently break if you forget them:
- Every response is enveloped: `{data, statusCode, success, timestamp}`. Errors:
  `{data: null, success: false, error, timestamp}`.
- Refresh tokens are **single-use and rotating**, access tokens last 15 minutes.
  Concurrent 401s must be single-flighted through the refresh actor or users get
  logged out. This is a real incident that happened in production. See ADR 0004.

## Finishing a feature

Run `/feature-done`. It builds, tests, lints, and walks the close-out checklist.

Non-negotiable on every feature:
1. Update `docs/STATE.md` — status, what works, what's deliberately deferred.
2. Write an ADR in `docs/decisions/` if you made a decision someone might later question.
3. Update `docs/API.md` if you touched an endpoint's shape.

**Do not create new top-level documents.** Features update the existing docs. Three
overlapping files generated in one session and never touched again is the exact failure
mode this structure exists to prevent.
