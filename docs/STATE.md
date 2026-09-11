# Current State

**What is true right now. Nothing else.**

This file is **rewritten in place, never appended to.** When something stops being
current it moves to `CHANGELOG.md`. History does not live here — that is how the
equivalent file in the server repo reached 2,000 lines and stopped being readable.

**Hard cap: 80 lines**, enforced by `Tools/check-doc-links.py`. If you are adding a line,
consider which one you are removing.

Last updated: 2026-09-11

---

## Where things stand

`MindlensKit` builds under Swift 6 language mode against iOS 18 with strict concurrency, and so
does the app target: `Config/Base.xcconfig` is the project's base configuration and the only
definition of those settings (ADR 0009). **75 tests across 17 suites pass** in ~2s from the CLI.
SwiftLint, swift-format and the doc-link check are clean over `Sources`, `Tests` and `mindlens`.

**The app has UI, verified by running it** — the scene root switches on `SessionState`, and the
signed-out branch is a sign-in screen on the system Sign in with Apple button, checked in light
and dark at default and the largest accessibility text size. Onboarding and signed-in are stubs.

Sign-in is wired end to end **except the credential exchange**: the server verifies Firebase ID
tokens, and no Firebase SDK is linked. `UnavailableIdentityProvider` throws at that seam, so
tapping a button reaches the boundary and stops there (ADR 0010).

Nothing reaches `main` except through the PR gate: build, test, launch, lint, doc links, build
settings, then `Tools/swiftgate`. See ADR 0006.

## Next action

Link the Firebase Auth SDK in the app target and replace `UnavailableIdentityProvider`. That one
file is all that stands between this build and a real session.

## Blocked on

A `GoogleService-Info.plist` from the Firebase project, and the Sign in with Apple capability.
Neither is in the repo. Everything else in Stage 1 is done and tested.

## Stages

| # | Feature | Status | Notes |
|---|---|---|---|
| 1 | Authentication | 🟡 | Screen, gate, repository, refresh transport, Keychain device GUID — all tested. Firebase exchange unlinked. |
| 2 | Dashboard + Quick Log | 🟡 | `DashboardModel` tested against a stub. No views, no real repository. |
| 3 | Insights + Recaps | ⬜ | Swift Charts; polls for server-side generation. |
| 4 | Onboarding + Paywall | ⬜ | Survey polling, RevenueCat. The Flutter login screen bundles this survey ahead of sign-in. |

Legend: ⬜ not started · 🟡 in progress · ✅ done · ⏸ deferred

## Known gaps

- **No captured fixture for `/auth/apple` or `/auth/refresh` 200** — both need a live Firebase
  token, so decoding is tested against inline bodies labelled as constructed, and swiftgate's
  fixture rule is waived in `AuthEndpoints.swift`. Same for `mood_create_200.json`. Re-capture.
- **A restore that fails transiently parks in `restoring` with a retry.** Correct, but only
  because no user row is cached — with persistence, a cold launch should paint from disk.
- **The local store is not observable.** `MoodRepository` returns a snapshot. Stage 2.
- **No persistence layer yet.** SwiftData models, the outbox, and ADR 0003's pre-ship validation
  gate are unstarted. Keychain is the only storage in use.
- No `PrivacyInfo.xcprivacy`, entitlements, or usage descriptions. Required before submission,
  and HealthKit alongside analytics needs a documented data boundary (Guideline 5.1.3).
- Analytics records events only — no `identify`, so no user is named to a vendor. Revisit with a
  real SDK. The Google button is unbranded: no system button exists and their mark is not an asset.

## Deliberately not doing

- **No feature parity with Flutter.** Health sync, Events, Tags, Reminders, Settings are out of
  scope. Add only by explicit decision.
- **No App Intents, Control, or widgets in v1** — ADR 0008.
- **No API versioning shim** — the backend has none.

## Open questions

- Is the Firebase indirection intentional? The server verifies **Firebase** ID tokens, so the
  app must carry the SDK to sign in with Apple. ADR 0010 makes either answer cheap.
