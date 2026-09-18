# Current State

**What is true right now. Nothing else.**

This file is **rewritten in place, never appended to.** When something stops being
current it moves to `CHANGELOG.md`. History does not live here — that is how the
equivalent file in the server repo reached 2,000 lines and stopped being readable.

**Hard cap: 80 lines**, enforced by `Tools/check-doc-links.py`. If you are adding a line,
consider which one you are removing.

Last updated: 2026-09-18

---

## Where things stand

Swift 6, iOS 18, strict concurrency — all from `Config/Base.xcconfig`, the only definition of those
settings (ADR 0009). **79 tests across 19 suites pass** in ~2s. SwiftLint, swift-format and the doc-link check are clean.

**The app has UI, verified by running it** — the scene root switches on `SessionState`; signed-out
is the sign-in screen, checked light/dark at default and the largest text size. The rest is stubs.

**Sign in with Apple works against production** — Apple → Firebase → `POST /auth/apple` → signed-in
stub, and a relaunch restores the session from the Keychain. It needs a `GoogleService-Info.plist`
from the production Firebase project beside `mindlens/Info.plist`; the file is gitignored, so CI
and a fresh clone get `UnavailableIdentityProvider` instead, and a Release build without it fails at
launch (ADR 0010, `docs/features/auth.md`). Every sign-in is a real production account.

Nothing reaches `main` except a PR through the gate — test, build, launch, settings + team + entitlement (Debug
only), lint, doc links, `Tools/swiftgate` static rules and judge lanes behind evidence gates with a deterministic
scorer (only `idiom` scores yet), branch name, ADR numbers and size (ADR 0006, 0012–0014). `main` is unprotected on this plan; `Tools/githooks` refuse the push.

## Next action

`feature/auth` for `docs/features/auth.md` step 4: the `REVERSED_CLIENT_ID` URL scheme into
`mindlens/Info.plist`, then a real "Continue with Google" — wired, never run. Beside it, a `fix/` branch for the
reviewer's two warnings on PR #1: cancellation at the `APIClient` boundary, and `RootView`'s placeholder copy.

## Blocked on

Nothing.

## Stages

| # | Feature | Status | Notes |
|---|---|---|---|
| 1 | Authentication | 🟡 | `docs/features/auth.md`. Apple sign-in and restore work live. Google, live fixtures, Keychain tests left. |
| 2 | Dashboard + Quick Log | 🟡 | `DashboardModel` tested against a stub. No views, no real repository. |
| 3 | Insights + Recaps | ⬜ | Swift Charts; polls for server-side generation. |
| 4 | Onboarding + Paywall | ⬜ | Survey polling, RevenueCat. The Flutter login screen bundles this survey ahead of sign-in. |

Legend: ⬜ not started · 🟡 in progress · ✅ done · ⏸ deferred

## Known gaps

- **No captured fixture for `/auth/apple` or `/auth/refresh` 200** — neither is scriptable, so
  decoding runs against inline bodies labelled as constructed and swiftgate's fixture rule is
  waived in `AuthEndpoints.swift`. Capture both off a proxied real sign-in (`auth.md` step 5).
- **The live refresh has never been observed** (restore only ran inside the 15-minute window), and
  **`Persistence` has no test target** — `KeychainItem` has run for real in the app, never under a test.
- **Sign-out leaves the Firebase user signed in** — the seam has no sign-out. Fine until account deletion.
- **A transient restore failure parks in `restoring` with a retry**, correct only because no user
  row is cached; **the local store is not observable** either. SwiftData, the outbox, ADR 0003's gate: Stage 2.
- No `PrivacyInfo.xcprivacy` or usage descriptions; HealthKit beside analytics needs a documented
  data boundary (Guideline 5.1.3). All required before submission.
- No brand colour or app icon — `AccentColor` is empty, so the app tints system blue. Analytics
  records events only. The Google button is unbranded but paired to Apple's per the HIG.

## Deliberately not doing

- **No feature parity with Flutter.** Health sync, Events, Tags, Reminders, Settings are out of scope; add only by decision.
- **No App Intents, Control, or widgets in v1** — ADR 0008.
- **No API versioning shim** — the backend has none.

## Open questions

- Is the Firebase indirection intentional? The server verifies **Firebase** ID tokens, so the app
  must carry the SDK to sign in with Apple at all. ADR 0010 makes either answer cheap to act on.
- What is Mindlens's native palette? Flutter's `#5A6DF0` fails WCAG AA on white and ships no dark mode.
