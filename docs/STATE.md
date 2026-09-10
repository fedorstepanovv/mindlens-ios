# Mindlens iOS — Current State

**This is the living file. Every session updates it before ending.** The other docs in
`docs/` are stable reference and change rarely; this one changes constantly.

Last updated: 2026-09-10 · Foundation green, linting wired, API contract verified live

Legend: ⬜ not started · 🟡 in progress · ✅ done · ⏸ deferred (with reason) · 📦 shipped

---

## Where things stand

The documentation system and the shared module layer exist and build. `MindlensKit`
compiles under Swift 6 language mode against iOS 18, and **16 tests across 5 suites
pass** — including the concurrency tests for token refresh. SwiftLint and swift-format
are configured and clean.

The API contract in `docs/API.md` has been **verified against production**, and the
error-envelope fixtures are real captures rather than hand-written guesses.

No app UI exists yet: the Xcode project is still the stock template and does not yet
reference the package.

**Next action:** wire `Packages/MindlensKit` into `mindlens.xcodeproj`, drop the
deployment target from 26.2 to 18.0, and set Swift 6 language mode on the app target.
Verify the app still builds before writing any feature code.

**Blocked on:** nothing.

## Foundation

| Item | Status | Notes |
|---|---|---|
| Repo scaffolding | ✅ | `docs/`, `.claude/`, `Packages/`, `Tools/` |
| CLAUDE.md front door | ✅ | |
| Architecture rules | ✅ | `docs/ARCHITECTURE.md` |
| Code patterns | ✅ | `docs/PATTERNS.md` |
| Design contract | ✅ | `docs/DESIGN.md` |
| Testing contract | ✅ | `docs/TESTING.md` |
| API contract | ✅ | `docs/API.md` — the only spec that exists |
| SPM package skeleton | ✅ | `Packages/MindlensKit` |
| Xcode project wiring | ⬜ | **Next action.** iOS 18 target, Swift 6 mode, package reference |
| SwiftLint + swift-format | ✅ | `.swiftlint.yml` + `.swift-format`; formatter owns formatting, linter owns correctness |
| CI (GitHub Actions) | ⬜ | build · test · lint · doc-link check |
| `/feature-done` command | ✅ | |

## Features

Sequenced deliberately — each stage must be complete before the next begins.

| Stage | Feature | Status | Notes |
|---|---|---|---|
| 1 | Authentication | 🟡 | `TokenRefresher` + `APIClient` built and tested. Sign-in UI and Firebase exchange not started. |
| 2 | Dashboard + Quick Log | 🟡 | `DashboardModel` built and tested against a stub repository. No views, no real repository. |
| 3 | Insights + Recaps | ⬜ | Swift Charts. Polls for server-side AI generation. |
| 4 | Onboarding + Paywall | ⬜ | Profile, goals, Lens picker, survey polling, RevenueCat. |

## Decisions made

See `docs/decisions/` for the full records. Summary:

| ADR | Decision |
|---|---|
| 0001 | Swift Concurrency + Observation as the primary stack; Combine used surgically |
| 0002 | Local SPM package with per-feature targets; features cannot import features |
| 0003 | SwiftData for local persistence, behind repository protocols |
| 0004 | Token refresh as an actor — single-flight, rotating-token aware |
| 0005 | Third-party SDKs behind protocols; stub implementations are the default |

## Deliberately not doing

Recorded so nobody re-litigates these or mistakes them for oversights:

- **No feature parity with Flutter.** Health sync, Events, Tag management, Reminders and
  Settings are out of the current scope. Add them only by explicit decision.
- **No API versioning shim.** The backend has none; adding client-side version
  negotiation would be inventing a contract the server doesn't honor.
- **No custom design system beyond tokens.** Native components carry the design. Building
  a bespoke component library would work against the goal of idiomatic iOS.

## Known gaps

- `mood_create_200.json` is the one **hand-written** fixture — it cannot be captured until
  auth works. Replace it during Stage 1. See the Fixtures README.
- The app target is still the Xcode template, including the lowercase `mindlensApp` type
  name that SwiftLint flags. Fix during project wiring.

## Open questions

- Whether Firebase is required at all, or whether the backend could accept Apple/Google
  identity tokens directly. Currently the server verifies **Firebase** ID tokens, so the
  iOS app must carry the Firebase Auth SDK. Worth confirming this is intentional.
