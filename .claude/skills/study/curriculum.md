# Curriculum

Seventeen concepts, in prerequisite order, each pinned to the file in this repo that
teaches it best. Tiers are ordered; rows inside a tier are ordered. Flip ☐ to ☑ when a
row is done — this is the only file `/study` writes.

Paths: Swift files are under `Packages/MindlensKit/Sources/` unless they start with
`mindlens/` or `Tests/`. Flutter files are under `../app/mindlensapp/lib/src/`.

Each row: **Swift** (read this) · **Flutter** (the before picture) · **Rules** (the doc
that decided it) · **Idioms** (what the row teaches, in the order it should be met) ·
**Ask** (the interview question, then its follow-up) · **Wider iOS** (where this repo is
stricter than the field, so it is not over-generalised).

---

## Tier A — the language

### 1 · Value types and `Sendable` ☐
- Swift: `Core/Tokens.swift` (`TokenPair`), `Models/User.swift`, `Core/Identity.swift` (`SignInNonce`, `AppleIdentityRequest`)
- Flutter: `features/auth/models/tokens/tokens_model.dart` — a freezed data class
- Rules: `docs/PATTERNS.md` § Concurrency
- Idioms: `struct` copies vs `class` references · `let` stored properties · the memberwise init and why every public type writes its own `public init` · synthesized `Equatable`/`Hashable` · `Sendable` as "safe to cross an isolation boundary" · `final class APIClient: Sendable` as the deliberate exception
- Ask: "When does a struct stop being the right choice?" → "Why is `APIClient` a `final class` and not a struct or an actor?"
- Wider iOS: `Sendable` is everywhere under Swift 6; a Swift 5 codebase mostly omits it and gets warnings, not errors.

### 2 · Enums with payloads and exhaustive `switch` ☐
- Swift: `Models/SessionState.swift`, `Core/AppError.swift` (`Kind`), `Models/User.swift` (`AuthProvider.unknown`), `Core/Identity.swift` (`SignInProvider`)
- Flutter: `features/app/presentation/bloc/app_state.dart` (freezed union), `shared/enums/auth_status.dart`
- Rules: `docs/ARCHITECTURE.md` § Navigation
- Idioms: associated values · binding across cases with `case .onboarding(let user), .signedIn(let user)` · `switch` as an expression · raw-value enums with a custom `init(from:)` fallback · `CaseIterable`, `Identifiable` · why `SignInProvider` and `AuthProvider` are two enums
- Ask: "How do you model a screen's state without a Bloc state class?" → "A new case is added. What does the compiler do, and what did freezed do?"
- Wider iOS: universal — this is Swift, not house style.

### 3 · Protocols, `any` and `some` ☐
- Swift: `Models/AuthRepository.swift`, `Core/Tokens.swift` (`TokenStorage`), `Core/Identity.swift` (`IdentityAuthenticating`), `Analytics/AnalyticsRecording.swift` (`.noop` via `where Self ==`)
- Flutter: `features/auth/repositories/auth/auth_repository.dart` + `auth_repository_impl.dart`
- Rules: ADR 0005, ADR 0010, `docs/ARCHITECTURE.md` § Dependency rules
- Idioms: `protocol X: Sendable` · `any X` (existential, dynamic dispatch) vs `some X` (opaque, static) · `ExistentialAny` making `any` mandatory in `Package.swift` · a protocol extension with `where Self == NoopAnalytics` as the `.noop` default · conforming types named for what they are (`APIAuthRepository`, `KeychainTokenStorage`), never `*Impl`
- Ask: "`any` vs `some` — when each?" → "Why does `AuthRepository` live in `Models` and not in the Authentication feature?"
- Wider iOS: `any` was optional before Swift 6; older code writes the bare protocol name and means the same thing.

### 4 · Codable, by hand where it matters ☐
- Swift: `Core/CalendarDate.swift` (`CalendarDate`, `TimeOfDay`), `Networking/APIEnvelope.swift` (`APIErrorBody` — string-or-array), `Networking/Endpoint.swift` (`JSONDecoder.api`), `Networking/AuthEndpoints.swift` (`AuthSessionDTO` → `User`)
- Flutter: any `*.g.dart` from json_serializable; `features/auth/models/tokens/tokens_model.dart`
- Rules: `docs/API.md`, `docs/PATTERNS.md` § Dates and timezones
- Idioms: synthesized `Codable` · `CodingKeys` · `init(from:)` with `singleValueContainer()` and `container(keyedBy:)` · `decodeIfPresent` · `try?` to accept either shape · `DecodingError.dataCorrupted` · a custom `dateDecodingStrategy` · generic `APIEnvelope<Payload>` · a DTO kept `internal` to `Networking`
- Ask: "The server sends `message` as a string or an array. Decode it." → "Why three date shapes instead of one `dateDecodingStrategy`?"
- Wider iOS: universal, though many teams reach for `.convertFromSnakeCase` and never write `init(from:)` — being able to is the differentiator.

### 5 · Generics and a typed request ☐
- Swift: `Networking/Endpoint.swift`, `Networking/APIClient.swift` (`send<Response>`, `decode`), `Networking/AuthEndpoints.swift` (`extension Endpoint where Response == User`)
- Flutter: Dio's `get<T>` with a `fromJson` callback at every call site; `core/network/`
- Rules: ADR 0007, `docs/PATTERNS.md` § Networking
- Idioms: a generic struct whose `Response` is a phantom type · constrained extensions (`where Response == AuthSessionDTO`) · `some Decodable & Sendable` in parameter position · `NoContent() as? Response` for 204 · static factories on a generic type · `throws` from a factory (`.post` encodes)
- Ask: "Why attach the response type to the request?" → "What does `extension Endpoint where Response == User` buy over a plain function returning `Endpoint<User>`?"
- Wider iOS: a hand-written client is common; Alamofire and Moya are equally common, and Moya's `TargetType` is this idea with an enum.

### 6 · Errors: one type at the boundary ☐
- Swift: `Core/AppError.swift`, `Networking/APIClient.swift` (`failure(from:status:)`), `Features/Authentication/SessionModel.swift` (the `restore()` catch ladder), `DesignSystem/AppErrorPresentation.swift`, `Core/Log.swift`
- Flutter: `shared/exceptions/`, `DioException` handling in `core/network/auth_interceptor.dart`
- Rules: `docs/ARCHITECTURE.md` § Error handling, `docs/PATTERNS.md` § Errors, LESSONS row on `AppError.diagnostic`
- Idioms: untyped `throws` · a `do/catch` ladder with `catch let e as AppError where e.kind == .unauthenticated` · `catch is CancellationError` · `init(_ error: some Error)` as a mapping constructor · `Error` on a struct · `-> Never` · `Logger` with `privacy: .public` · why copy lives in `DesignSystem`, not `Core`
- Ask: "Walk an offline `URLError` from `URLSession` to the banner." → "Why is `.unauthenticated` the only kind that signs the user out?"
- Wider iOS: `Result` and `NSError` domains are still everywhere; typed throws (`throws(AppError)`) exists since Swift 6 and this repo chose not to use it — know why either way.

## Tier B — concurrency, the part that is not Dart

### 7 · `async`/`await`, `Task`, cancellation ☐
- Swift: `Features/Dashboard/DashboardModel.swift` (`load()`), `Features/Authentication/SessionModel.swift` (`signIn(with:_:)`), `mindlens/RootView.swift` (`.task { }`), `Features/Authentication/SignInView.swift` (`Task { }` inside a button)
- Flutter: `Future`, `async*`; `features/dashboard/presentation/view/dashboard/cubit/dashboard_cubit.dart`
- Rules: `docs/PATTERNS.md` § Loads are driven by the view · § Cancellation is not an error; SwiftLint `no_task_in_didset`
- Idioms: suspension points · `Task { }` (unstructured) vs `.task(id:)` (cancelled with the view) · `CancellationError` · `defer` across an `await` · `Task.sleep(for:)` · `try? await` as a deliberate discard · a `() async throws -> User` closure parameter
- Ask: "The user navigates away mid-load. What happens in Flutter, what happens here, and where is cancellation decided?" → "Why is `CancellationError` caught and ignored rather than shown?"
- Wider iOS: universal since iOS 15; codebases older than that wrap completion handlers with `withCheckedThrowingContinuation`, and you will meet that bridge.

### 8 · Actors: reentrancy and single-flight — the centrepiece ☐
- Swift: `Networking/TokenRefresher.swift` — `refreshed(after:)`, `performRefresh()`, the stamped `defer`, `handle(refreshFailure:)`; `Networking/APIClient.swift` (`send` — one retry)
- Flutter: `core/network/auth_interceptor.dart` — `QueuedInterceptor`, `_refreshCompleter`, `_bareRetryDio`
- Rules: ADR 0004, `docs/PATTERNS.md` § Token refresh, LESSONS rows on the generation counter and the raced test
- Idioms: `actor` — serialised access to mutable state · **reentrancy**: state can change across every `await` inside an actor · `Task<Credentials, any Error>` held as a join handle other callers `await` · generation counters over booleans · `&+=` · `some Error` · why nothing here is `nonisolated`
- Ask: "Two requests 401 at the same moment and refresh tokens are single-use. What goes wrong, and how do you make it *unrepresentable*?" → "A request was already in flight when someone else refreshed. It comes back 401. Does it refresh again?"
- Wider iOS: older code does this with a serial `DispatchQueue`, `NSLock`, or an `OperationQueue`; Alamofire's `RequestInterceptor` is the closest thing to the Dart interceptor. The actor is the modern form of the same idea, and the generation check is the part most codebases miss.

### 9 · Strict concurrency: isolation and `Sendable` checking ☐
- Swift: `Packages/MindlensKit/Package.swift` (`strict`, `uiFacing`, `.defaultIsolation(MainActor.self)`), `Networking/APIClient.swift` (`final class: Sendable`), `TestSupport/Fakes.swift` (`Mutex`, actors as fakes)
- Flutter: isolates — a weak analogy. Dart has no shared-memory data race to check for.
- Rules: ADR 0001, `docs/PATTERNS.md` § Concurrency, `docs/TESTING.md` § Tests run in parallel
- Idioms: `@MainActor` on a type vs on a function · default actor isolation per module (SE-0466) and why the app target and `uiFacing` targets agree · `@Sendable` closures · `Mutex` from `Synchronization` · why a stub is an `actor` · `MemberImportVisibility`
- Ask: "What does Swift 6 strict concurrency reject that Swift 5 allowed?" → "Why is `RecordingAnalytics` `Mutex`-backed while `StubAuthRepository` is an actor?"
- Wider iOS: most shipping apps are still in Swift 5 mode with warnings on. The *migration* story — what breaks first, what `@preconcurrency` buys — is the interview value.

## Tier C — UI and state

### 10 · `@Observable`, and where a ViewModel earns its keep ☐
- Swift: `Features/Authentication/SessionModel.swift`, `Features/Dashboard/DashboardModel.swift`, `mindlens/RootView.swift` (reads `session.state`)
- Flutter: `features/auth/presentation/login/bloc/login_bloc.dart`, `features/dashboard/presentation/view/dashboard/cubit/dashboard_cubit.dart`, `BlocBuilder`
- Rules: `docs/PATTERNS.md` § ViewModels · § When *not* to write a ViewModel · § One model per flow; ADR 0001
- Idioms: `@Observable` (a macro; property-level tracking) · `@MainActor final class` · `public private(set) var` · computed state (`isSigningIn`, `needsRestoreRetry`) instead of stored flags · one model per *flow*, not per screen · a plain `let model` in the view, no `@StateObject`
- Ask: "`@Observable` vs `ObservableObject` + `@Published` — what changes for the view?" → **Combine:** "Sketch `SessionModel.state` as a `@Published` consumed by a UIKit controller through `sink` and `AnyCancellable`."
- Wider iOS: `ObservableObject` / `@Published` / `@StateObject` is what most codebases — and the target job — actually have. Know both; explain why this repo picked one.

### 11 · SwiftUI: ownership, environment, previews ☐
- Swift: `Features/Authentication/SignInView.swift`, `mindlens/mindlensApp.swift` (`@State private var container`), `mindlens/RootView.swift` (`@ViewBuilder`, the root `switch`), `Models/AuthRepository+Preview.swift`
- Flutter: `features/auth/presentation/login/view/login_view.dart`, `features/app/presentation/view/app_view.dart`, `features/app/router/router.dart` (the go_router redirect)
- Rules: `docs/DESIGN.md` § Non-negotiable, `docs/PATTERNS.md` § Previews, `docs/ARCHITECTURE.md` § Navigation
- Idioms: `some View` · `@State` (owned here) vs a plain `let` (handed in) · `@Environment(\.colorScheme)`, `@ScaledMetric` · `@ViewBuilder` · `.task` · `.frame(minHeight:)` vs `containerRelativeFrame` · `.id()` to force a fresh identity · `#Preview` with a scripted repository · `Text(_, bundle: .module)` · `ContentUnavailableView`
- Ask: "Where does `@State` belong, and why is the container `@State` in `App`?" → "Why a `switch` at the root instead of a router redirect?"
- Wider iOS: many apps are UIKit with SwiftUI islands, or use `NavigationStack` with a path enum. The root `switch` is standard; the Dynamic Type rigour in `SignInView` is not, and is worth showing off.

## Tier D — architecture

### 12 · Constructor injection and the composition root ☐
- Swift: `mindlens/AppContainer.swift`, `mindlens/mindlensApp.swift`, every `public init(...)` that takes an `any Protocol`
- Flutter: `locator.dart` — get_it
- Rules: `docs/ARCHITECTURE.md` § Dependency injection, ADR 0005, ADR 0010
- Idioms: `convenience init` vs designated · a default argument as the test seam (`analytics: any AnalyticsRecording = .noop`) · `#if DEBUG` / `preconditionFailure` for misconfiguration · a `@MainActor final class` graph held in `@State` for the app's lifetime
- Ask: "No DI framework. How does a deep type get its dependency, and how do you swap it in a test?" → "What would `get_it` have cost here?"
- Wider iOS: Swinject, Factory and Needle are common; `Environment`-based injection is the SwiftUI-native alternative. A pure composition root is the least common and the easiest to defend.

### 13 · Modules and access control ☐
- Swift: `Packages/MindlensKit/Package.swift`, `Networking/AuthEndpoints.swift` (internal DTOs), `Persistence/KeychainItem.swift` (internal), any `@testable import` in `Tests/`
- Flutter: `features/auth/auth.dart` and the other barrel files; `part` / `part of`
- Rules: ADR 0002, `docs/ARCHITECTURE.md` § Module graph
- Idioms: `public` / `internal` (the default) / `private` / `fileprivate` / `private(set)` · `@testable import` · a target per feature · `path:` and `resources:` on a target · `Bundle.module` · no umbrella product, on purpose
- Ask: "How do you stop feature A importing feature B?" → "What has to be `public` for a feature to use `AuthRepository`, and what should stay internal?"
- Wider iOS: many apps are a single target and a senior interview asks how you would split one. This repo is the answer, already built.

### 14 · Keychain and C interop ☐
- Swift: `Persistence/KeychainItem.swift`, `Persistence/KeychainTokenStorage.swift`, `Persistence/KeychainDeviceIdentity.swift`, `Networking/TokenRefresher.swift` (`credentials()` — the propagated storage error)
- Flutter: `flutter_secure_storage` behind `core/storage/`
- Rules: `docs/PATTERNS.md` § Token refresh; STATE "Persistence has no test target"
- Idioms: `[String: Any]` bridged to `CFDictionary` · `kSec*` constants · `OSStatus` wrapped in an `Error` struct · `var item: CFTypeRef?` as an out-parameter with `&` · `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` and *why* · `read()` telling `nil` from `throw`
- Ask: "Which accessibility class, and what breaks with the wrong one?" → "`credentials()` propagates a storage error instead of `try?` — why?"
- Wider iOS: most teams wrap this in KeychainAccess or Valet; being able to read the raw `SecItem` calls is the interview value.

## Tier E — testing

### 15 · Swift Testing ☐
- Swift: `Tests/AuthenticationTests/SessionModelTests.swift`, `Tests/NetworkingTests/EnvelopeDecodingTests.swift`, `Tests/NetworkingTests/TokenRefresherRegressionTests.swift`, `TestSupport/Fixture.swift`
- Flutter: `test()`, `blocTest`, `group`
- Rules: `docs/TESTING.md`
- Idioms: `@Suite` · `@Test("name", arguments:)` · `#expect` · `try #require` · `.bug(...)`, `.timeLimit` · `@MainActor` on a suite · parallel by default · fixtures from `TestSupport/Fixtures` via `Bundle.module` · `swift test --filter`
- Ask: "How do you test a `@MainActor` model?" → "A parameterised test or four tests — when each?"
- Wider iOS: XCTest is still the majority; Swift Testing is the direction. Know `XCTAssertEqual` and `XCTestExpectation` too — the target codebase almost certainly has them.

### 16 · Test doubles that survive parallelism ☐
- Swift: `TestSupport/AuthFakes.swift` (`StubAuthRepository`, `GatedRefreshTransport`), `TestSupport/Fakes.swift` (`CountingRefreshTransport`, `RecordingAnalytics`), `TestSupport/StubURLProtocol.swift`
- Flutter: mocktail's `when(...).thenAnswer(...)`
- Rules: `docs/TESTING.md` § Concurrency tests must assert the hard case, LESSONS row on the raced test
- Idioms: hand-written fakes over a mocking library · `actor` fakes · `CheckedContinuation` as a gate (`waitForEntry`, `release(count:)`) · a `URLProtocol` subclass to stub `URLSession` · scripted outcomes as an `enum`
- Ask: "How do you make 'a refresh in flight during sign-out' deterministic?" → "Why is a timing-based concurrency test worse than no test?"
- Wider iOS: mocking libraries (Cuckoo, Mockolo, Sourcery-generated) are common; hand-written fakes are the more defensible choice and this repo shows why.

## Tier F — beyond this repo

### 17 · Combine, for the job that uses it ☐
- Swift: nothing here — that is the point. ADR 0001 is the argument, and every row in tiers B and C is its `@Observable`/`async` half.
- Flutter: `Stream`, `StreamController`, rxdart, `BlocListener`
- Rules: ADR 0001
- Idioms to write cold: `Publisher` / `Subscriber` · `@Published` · `AnyPublisher` · `sink` / `assign` · `AnyCancellable` and `Set<AnyCancellable>` · `map` / `flatMap` / `debounce` / `removeDuplicates` / `combineLatest` · `Future` · `PassthroughSubject` / `CurrentValueSubject` · `receive(on:)` · bridging to `async` with `.values`
- Ask: "Where would Combine be the *right* tool in Mindlens?" (debounced search, `NotificationCenter`, merging two sources) → "Rewrite `DashboardModel.load()` as a publisher chain with cancellation."
- Wider iOS: this is what the target codebase has. Drills on this row take a tier-B or tier-C row and ask for its Combine twin.
