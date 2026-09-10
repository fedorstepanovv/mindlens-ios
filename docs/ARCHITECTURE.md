# Architecture

Stable reference. Changes only by ADR.

## Module graph

One local SPM package, many targets. SPM enforces the dependency rules at compile time,
which is the entire reason for the structure — conventions erode, compilers don't.

```
mindlens (app target)
│  App entry · root Scene · Router · DI composition root · asset catalog
│  Thin by design. No business logic.
│
└── Packages/MindlensKit
    ├── Core           Foundation-level. DateProvider, AppError, TokenPair, TokenStorage.
    │                  Depends on nothing.
    ├── Models         Domain entities and repository protocols. Value types. Depends on Core.
    ├── Networking     APIClient, Endpoint, envelope decoding, TokenRefresher actor.
    │                  Depends on Core, Models.
    ├── Persistence    SwiftData stores, Keychain, the write outbox.
    │                  Depends on Core, Models.
    ├── DesignSystem   Tokens, shared components, previews. Depends on Core.
    ├── Analytics      Protocols + stub implementations. Depends on Core, Models.
    │
    ├── Features/      Added as they are built — see docs/STATE.md for what exists.
    │   ├── Authentication
    │   └── Dashboard
    │       Each depends on: Core, Models, Networking, Persistence,
    │       DesignSystem, Analytics — and never on another Feature.
    │
    └── TestSupport    Fakes, fixtures, builders. Depends on everything.
                       Never linked into the app.
```

## Dependency rules

1. **A feature may not import another feature.** If two features need the same thing, it
   moves down into a shared target. If a feature needs to navigate to another, it emits a
   typed destination that the app target's router resolves.
2. **Nothing imports the app target.** It sits at the top and is the only place that knows
   the whole graph.
3. **No target imports a third-party SDK except the app target.** Feature code talks to
   our own protocols. The composition root binds them to real SDKs or stubs.
4. **Core depends on nothing.** If something in Core needs Models, it belongs in Models.

## Layers inside a feature

Deliberately shallower than the Flutter app's `repository → service → cubit`. That
three-layer split produced files that only forwarded calls.

```
View            SwiftUI. Dumb. Binds to a ViewModel or straight to a model.
ViewModel       @Observable @MainActor. Presentation state + user intent.
                Exists only where there is real state to coordinate.
Repository      Protocol. Owns the local store as source of truth; syncs from network.
                Returns domain models, never DTOs.
```

A service layer is added **only** when logic is genuinely shared across features, and
then it lives in a shared target, not in a feature.

## Data flow

The local store is the source of truth the UI observes. The network syncs into it.

```
View ──observes──> ViewModel ──> Repository ──> SwiftData store (source of truth)
                                      │                 ▲
                                      └──> APIClient ───┘  sync in
                                      └──> Outbox ─────────> retries writes
```

Views never wait on the network to render. A cold launch paints from disk immediately,
then revalidates. Writes to the core logging path (mood, tags, notes) go through the
outbox so they survive being offline.

**Not yet true.** Today `MoodRepository` returns a snapshot array, so a background sync
writes to the store and the UI does not see it. Making the store genuinely observable is
Stage 2 work and has two honest options — `@Query` in the view (live, but `@Model` types
leak into features) or the store vending an `AsyncSequence` of domain values (layering
intact, more work on SwiftData). Tracked in `docs/STATE.md`; do not let this paragraph
read as done.

A background revalidation must not swallow its error with `try?` — that contradicts the
error rule below. Log it even when there is nothing to show.

## Dependency injection

Constructor injection. The composition root in the app target builds the object graph
once at launch and hands dependencies down.

```swift
// App target — the only place that knows about concrete types.
let container = AppContainer(environment: .live)

// Features receive exactly what they need, nothing more.
DashboardView(model: DashboardModel(
    moods: container.moodRepository,
    analytics: container.analytics
))
```

`@Environment` carries dependencies that genuinely belong to the view tree (theme,
routing). It is not a back door for services — if a ViewModel needs it, it comes through
`init`.

No global container. No service locator. No `.shared` outside Apple's own types. A
singleton is a dependency you can't fake, and a dependency you can't fake is a test you
can't write.

## Navigation

Each tab owns a `NavigationStack` with a typed path. The router lives in the app target.

The session gate is a `switch` at the `Scene` root, not a redirect callback — the
unauthenticated tree does not exist while signed in, and vice versa. This is structurally
different from `go_router`'s global `redirect`, and it means an unauthenticated view
cannot be reached by accident.

Note that `TokenStorage` lives in `Core`, not `Networking`. `Persistence` implements it,
and having persistence depend on networking would point the arrow the wrong way — the
compiler caught this the first time the package was built.

## Error handling

One `AppError` in Core, surfaced to users through a presentation-layer mapping. Network
errors carry their HTTP status; the envelope's `error` payload is decoded and preserved.
No error is ever swallowed silently — if it can't be shown, it's logged.
