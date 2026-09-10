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

### Fixed
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
