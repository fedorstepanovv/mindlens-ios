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

`MindlensKit` and the app target both build under Swift 6 against iOS 18 with strict concurrency,
from `Config/Base.xcconfig`, the only definition of those settings (ADR 0009). **79 tests across 19
suites pass** in ~2s. SwiftLint, swift-format and the doc-link check are clean.

**The app has UI, verified by running it** — the scene root switches on `SessionState` and the
signed-out branch is a sign-in screen on the system Sign in with Apple button, checked in light and
dark at default and the largest accessibility size. Onboarding and signed-in are stubs.

Sign-in is wired end to end **except the credential exchange**: no Firebase SDK is linked, so
`UnavailableIdentityProvider` throws at that seam and a tap stops there (ADR 0010).

Nothing reaches `main` except the PR gate: test, build, launch, build settings (Debug only —
Release is unasserted), lint, doc links, and `Tools/swiftgate` alongside. See ADR 0006.

## Next action

Link the Firebase Auth SDK in the app target and replace `UnavailableIdentityProvider`. That one
file is all that stands between this build and a real session.

## Blocked on

A `GoogleService-Info.plist` from the Firebase project, and the Sign in with Apple capability.
Neither is in the repo. Everything else in Stage 1 is done and tested.

## Stages

| # | Feature | Status | Notes |
|---|---|---|---|
| 1 | Authentication | 🟡 | Screen, gate, repository and refresh transport tested (79 tests). Firebase exchange unlinked; `Persistence` has no test target, so the Keychain paths are unexercised. |
| 2 | Dashboard + Quick Log | 🟡 | `DashboardModel` tested against a stub. No views, no real repository. |
| 3 | Insights + Recaps | ⬜ | Swift Charts; polls for server-side generation. |
| 4 | Onboarding + Paywall | ⬜ | Survey polling, RevenueCat. The Flutter login screen bundles this survey ahead of sign-in. |

Legend: ⬜ not started · 🟡 in progress · ✅ done · ⏸ deferred

## Known gaps

- **No captured fixture for `/auth/apple` or `/auth/refresh` 200** — neither is scriptable, so
  decoding runs against inline bodies labelled as constructed and swiftgate's fixture rule is
  waived in `AuthEndpoints.swift`. Capture both by hand at the first real sign-in.
- **`Persistence` has no test target**, so `KeychainItem.write` has never run — not in a test, and
  not in the app, where sign-in throws at the identity provider before it asks for a GUID.
- **A transient restore failure parks in `restoring` with a retry**, correct only because no user
  row is cached; **the local store is not observable** either. SwiftData, the outbox and ADR 0003's
  validation gate are all unstarted. Stage 2.
- No `PrivacyInfo.xcprivacy`, entitlements or usage descriptions; HealthKit beside analytics needs
  a documented data boundary (Guideline 5.1.3). All required before submission.
- No brand colour or app icon — `AccentColor` is empty, so the app tints system blue. Analytics
  records events only. The Google button is unbranded but paired to Apple's per the HIG.

## Deliberately not doing

- **No feature parity with Flutter.** Health sync, Events, Tags, Reminders, Settings are out of
  scope. Add only by explicit decision.
- **No App Intents, Control, or widgets in v1** — ADR 0008.
- **No API versioning shim** — the backend has none.

## Open questions

- Is the Firebase indirection intentional? The server verifies **Firebase** ID tokens, so the app
  must carry the SDK to sign in with Apple at all. ADR 0010 makes either answer cheap to act on.
- What is Mindlens's native palette? Flutter's `#5A6DF0` fails WCAG AA on white, and it ships no
  dark mode, so both need deciding rather than copying.
