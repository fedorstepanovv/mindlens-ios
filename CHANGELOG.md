# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

This file is **append-only history**. `docs/STATE.md` holds only what is true *now* and is
rewritten in place — when something stops being current, it moves here rather than
accumulating there. That split is what stops the state file growing into the 2,000-line
progress log this project has produced before.

## [Unreleased]

### Added
- Documentation system: `CLAUDE.md` front door, `docs/` reference set, ADRs, and
  `docs/STATE.md` as a bounded current-state file.
- `MindlensKit` local SPM package — `Core`, `Models`, `Networking`, `Persistence`,
  `DesignSystem`, `Analytics`, `Dashboard`, `TestSupport`. Swift 6 language mode,
  iOS 18 floor, strict concurrency, `ExistentialAny` and `MemberImportVisibility` on.
- `TokenRefresher` — single-flight refresh with a generation guard and failure
  classification (ADR 0004).
- `APIClient`, typed `Endpoint`, envelope decoding, `KeychainTokenStorage`.
- `CalendarDate` and `TimeOfDay` for the API's non-timestamp date formats.
- `StubURLProtocol` in `TestSupport`, so no test touches the network.
- SwiftLint + swift-format, split so the formatter owns formatting and the linter owns
  correctness.
- `Tools/check-doc-links.py` — fails on any doc referencing a path that does not exist.
- `Tools/capture-fixtures.sh` — re-captures API fixtures from the live server.
- `Tools/swiftgate` and the PR gate workflow.
- `Config/Base.xcconfig` with the production API URL; secrets kept out of git.
- `Tools/check-build-settings.sh` — resolves the app's build settings and asserts them,
  so a claim about Swift version or deployment target cannot outlive the setting.
- ADR 0009 — build settings live in xcconfig, applied at the project level.
- **Stage 1, authentication.** The app has UI: the scene root is a `switch` on
  `SessionState`, and the signed-out branch is a sign-in screen built on the system
  `SignInWithAppleButton`, with the legal notice as inline markdown links.
- `SessionModel` — one `@Observable @MainActor` model for the whole session flow (restore,
  sign in, sign out) rather than one per screen.
- `APIAuthRepository` in `Networking`, so `AuthSessionDTO` never leaves that target;
  `Features/Authentication` links neither `Networking` nor `Persistence` (ADR 0010).
- `IdentityAuthenticating` in `Core` — the Firebase exchange behind a protocol we own, with
  only plain `Sendable` values crossing it. `UnavailableIdentityProvider` in the app target
  throws until the SDK is linked (ADR 0010).
- `AuthTokenRefreshTransport` — the live `POST /auth/refresh`, deliberately not routed
  through `APIClient`, which would recurse into refresh on a rejected refresh token.
  `TokenRefresher` had no concrete transport before this.
- `KeychainDeviceIdentity` — an install-persistent device GUID, an `actor` because
  get-or-create is a read-modify-write, and preserved across sign-out so the next sign-in
  reuses this device's session slot instead of evicting another device.
- `KeychainItem`, extracted so the token pair and the device GUID share one set of
  `SecItem` calls and one accessibility attribute.
- `SystemDeviceModel` — `utsname.machine` with the simulator case handled: it reports
  `arm64` there, one character under the API's six-character floor, which was a 400 on
  every sign-in on every developer's machine.
- `DateProvider.timeZone`, the seam that lets `TimeZone.current` be banned everywhere else.
- ADR 0010 — identity exchange behind a protocol, auth repository in `Networking`.
- The app's composition root (`AppContainer`) and session gate (`RootView`).

### Changed
- Split the memory system so no file grows without bound: `docs/STATE.md` is now
  current-state only (rewritten, never appended, capped at 80 lines), with history moving
  here and decisions to ADRs.
- Added `docs/LESSONS.md` as behavioural memory — each entry must name the mechanical
  guard that stops the mistake recurring, and is deleted once the guard makes it
  unrepeatable.
- Added a routing table to `CLAUDE.md` so a session reads ~100 lines to start, then only
  the documents its task actually touches.
- `Tools/check-doc-links.py` now enforces size caps as well as resolving references.
- SwiftLint custom rules turn three past mistakes into build failures:
  `localized_needs_bundle`, `no_task_in_didset`, `no_impl_suffix`.
- `Config/Base.xcconfig` is now the Xcode project's base configuration for Debug and
  Release. The `SWIFT_VERSION` and `IPHONEOS_DEPLOYMENT_TARGET` entries were deleted from
  all six target configurations, where they had been overriding it (ADR 0009).
- The PR gate lints `Packages/MindlensKit/Tests` alongside `Sources`, and verifies the
  resolved build settings before linting.
- `Tools/check-doc-links.py` now also fails on a document living outside its one home.
- Per-user Xcode scheme state is no longer tracked; `.gitignore` already listed it.
- `docs/API.md` documents a **third** error-envelope shape: a 422 spells its status
  `status`, not `statusCode`, and carries no `error` key. Captured live as
  `error_invalid_token_422.json`. `data.user` is documented as the whole Prisma row.
- `AppConfiguration` reads the API origin from the app's Info.plist, which
  `mindlens/Info.plist` fills in from `API_BASE_URL` — one source of truth.
  `Tools/check-build-settings.sh` asserts the key in the **built** bundle.
- `SwiftLint custom_rules.text_needs_bundle` — `SwiftUI.Text("…")` in a package target has
  the same silent non-localization hole as `String(localized:)`.
- `Tools/check-doc-links.py` skips `worktrees`. A git worktree is a different checkout, so
  `.claude/worktrees/gate/docs` was being reported as a duplicate `docs/` tree — the check
  failed for everyone whenever a gate run was in flight.

### Fixed
Stage 1 follow-up, 2026-09-11 — all three found by running the app rather than building it:
- **The app crashed on launch.** `INFOPLIST_KEY_MindlensAPIBaseURL` resolved correctly in the
  build settings and never reached the bundle: Xcode's generated Info.plist forwards only the
  `INFOPLIST_KEY_` names on its own allowlist and drops the rest without a word. Replaced with
  a partial `mindlens/Info.plist` that Xcode merges its generated keys into, and a
  `PBXFileSystemSynchronizedBuildFileExceptionSet` so the synchronized folder does not also
  copy that file as a resource and collide with the processed one.
- `Tools/check-build-settings.sh` asserted the build setting, which was correct, rather than
  the built `Info.plist`, which was missing the key. It now reads the bundle.
- The sign-in screen broke at accessibility text sizes: the bottom panel could not scroll, and
  `SignInWithAppleButton` lost its capsule and fell under the 44pt tap target. The screen is
  one scroll view now, and the button's height is clamped to the 30–64pt range Apple documents
  and rebuilt on a text-size change, which it does not survive on its own.

Findings from an adversarial architecture review, 2026-09-10:
- Keychain wrote the token pair as two items; a tear left a new access token beside an
  already-consumed refresh token. Now one atomic item.
- `signOut()` left the in-flight refresh running, which completed afterwards and
  re-persisted live tokens for a signed-out user.
- `APIClient` was an `actor` with only `let` properties — an executor hop per request
  protecting nothing. Now a `Sendable final class`.
- All seven `String(localized:)` calls omitted `bundle: .module`, so no package string
  would ever localize. User-facing copy moved to `DesignSystem`.
- `APIClient` had no tests at all. Now covered for envelope unwrapping, the 401 retry
  path, error mapping, 204, and decoding failure.
- `PATTERNS.md` shipped a broken copy of `TokenRefresher` beside the correct code, and
  three doc snippets did not compile.
- A single global `dateDecodingStrategy` could not serve three wire formats.
- `Package.swift` exported an umbrella library — a barrel export, which `CLAUDE.md` bans.
- Cancelled loads rendered as user-facing errors.
- `SessionState` lived in a feature target that another feature would need.

Finishing the base setup, 2026-09-10:
- The app target built at `SWIFT_VERSION = 5.0` and `IPHONEOS_DEPLOYMENT_TARGET = 26.2`
  while every document claimed Swift 6 and iOS 18. `Config/Base.xcconfig` existed with the
  right values and was referenced by nothing.
- `API_BASE_URL` resolved to `https:mindlens-api-production…` — an xcconfig treats `//` as
  a comment inside values, so the variable meant to hold the scheme separator was empty.
- `StubURLProtocol`'s force-unwraps were suppressed for SwiftLint but not swift-format, so
  a full-tree format lint failed. Tests now share one `StubURLProtocol.url` instead of
  each building their own.
- Removed a duplicate `docs/` and `Config/` tree under `mindlens/`. The docs copy was a
  pre-trim snapshot and was staged for commit.
