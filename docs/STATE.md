# Current State

**What is true right now. Nothing else.**

This file is **rewritten in place, never appended to.** When something stops being
current it moves to `CHANGELOG.md`. History does not live here — that is how the
equivalent file in the server repo reached 2,000 lines and stopped being readable.

**Hard cap: 80 lines**, enforced by `Tools/check-doc-links.py`. If you are adding a line,
consider which one you are removing.

Last updated: 2026-09-10

---

## Where things stand

`MindlensKit` builds under Swift 6 language mode against iOS 18 with strict concurrency,
and so does the app target: `Config/Base.xcconfig` is the project's base configuration and
the only definition of either setting (ADR 0009). **28 tests across 7 suites pass**;
`swift test` runs from the CLI in ~2s. SwiftLint, swift-format and the doc-link check are
clean over `Sources`, `Tests` and `mindlens`. The API contract is verified against
production and the error fixtures are real captures.

Nothing reaches `main` except through the PR gate: build, test, lint, doc links, then
`Tools/swiftgate` — a deterministic rule catalogue plus an agentic review that blocks
diffs reading as translated Dart. See ADR 0006.

No app UI exists. The Xcode project links the package but is otherwise the stock template.

## Next action

Stage 1: authentication. Nothing is in the way — build settings, the network layer and
the merge gate are all in place.

## Blocked on

Nothing.

## Stages

| # | Feature | Status | Notes |
|---|---|---|---|
| 1 | Authentication | 🟡 | `TokenRefresher` + `APIClient` done and tested. No UI, no Firebase exchange. |
| 2 | Dashboard + Quick Log | 🟡 | `DashboardModel` tested against a stub. No views, no real repository. |
| 3 | Insights + Recaps | ⬜ | Swift Charts; polls for server-side generation. |
| 4 | Onboarding + Paywall | ⬜ | Survey polling, RevenueCat. |

Legend: ⬜ not started · 🟡 in progress · ✅ done · ⏸ deferred

## Known gaps

- **The local store is not observable.** `MoodRepository` returns a snapshot, so a
  background sync writes where the UI is not looking. Stage 2 work — see `docs/ARCHITECTURE.md`.
- **No persistence layer yet.** SwiftData models, the outbox, and ADR 0003's pre-ship
  validation gate are all unstarted.
- `mood_create_200.json` is the one hand-written fixture; it needs auth to capture for real.
- No `PrivacyInfo.xcprivacy`, entitlements, or usage descriptions. Required before any
  submission, and HealthKit alongside analytics needs a documented data boundary
  (App Store Guideline 5.1.3).

## Deliberately not doing

- **No feature parity with Flutter.** Health sync, Events, Tags, Reminders, Settings are
  out of scope. Add only by explicit decision.
- **No App Intents, Control, or widgets in v1** — ADR 0008. Highest-signal native feature,
  deferred knowingly.
- **No API versioning shim** — the backend has none; inventing one client-side would be
  honouring a contract the server does not.

## Open questions

- Is the Firebase indirection intentional? The server verifies **Firebase** ID tokens, so
  the iOS app must carry the Firebase SDK to sign in with Apple.
