# Current State

**What is true right now. Nothing else.**

**Rewritten in place, never appended to** — what stops being current moves to `CHANGELOG.md`; the server repo's
equivalent reached 2,000 lines. **Hard cap: 80 lines**, enforced by `Tools/check-doc-links.py`: adding a line means removing one.

Last updated: 2026-09-22

---

## Where things stand

Swift 6, iOS 18, strict concurrency — all from `Config/Base.xcconfig`, the only definition of those
settings (ADR 0009). **79 tests across 19 suites pass** in ~2s. SwiftLint, swift-format and the doc-link check are clean.

**The app has UI, verified by running it** — the scene root switches on `SessionState`; signed-out
is the sign-in screen, checked light/dark at default and the largest text size. The rest is stubs.

**Sign in with Apple and Google work against production** — provider → Firebase → `POST /auth/apple` → signed-in stub,
and a relaunch restores the session from the Keychain. It needs a `GoogleService-Info.plist` from the production
Firebase project beside `mindlens/Info.plist`; the file is gitignored, so CI and a fresh clone get
`UnavailableIdentityProvider` instead, and a Release build without it fails at launch (ADR 0010,
`docs/features/auth.md`). Every sign-in is a real production account.

Nothing reaches `main` except a PR through the gate — test, build, launch, settings + team + entitlement (Debug only), lint, doc links, `Tools/swiftgate` static rules, branch name, ADR numbers, size (ADR 0006, 0012–0015). **Only that deterministic half blocks.** The three lanes (verification, idiom against this repo's exemplars, spec) run behind their evidence gates, report and record, and all three are `blocks: false`: a `BLOCK` or `CANNOT_EVALUATE` stops nothing until a reviewed PR cites the grant rule — ≥10 judged PRs, noise ≤30%, ≥1 finding `changed` — with the kill rule written down beside it (ADR 0021). A finding without a `proof` is dropped and counted. The gate assigns *pass*, *brief* or *full* from the diff and labels the PR; a human always merges (ADR 0017). Caps: 1,000 changed lines, 10 unreviewed commits.
`swiftgate metrics` sums the records (ADR 0016) — **no lane has judged a real diff yet**, so every cost and noise field
is empty. `main` is unprotected on this plan; `Tools/githooks` refuse the push and `Tools/setup.sh` installs them.

The pipeline is written down and built: `README.md` is the artifact — stages and owners (ADR 0019), the decisions by
area against September 2026 practice (ADR 0017–0023), an evidence section that stays empty until the records fill it —
and `AGENTS.md` is canonical, `CLAUDE.md` its import. Every mechanism those ADRs decided now exists in the gate. The
feature template has a human-approved `## Behaviour` section (ADR 0020); `docs/features/auth.md` gets its own at step 5.

## Next action

The load, starting with `feature/auth` — `docs/features/auth.md` step 5: write its `## Behaviour`, capture the
`/auth/apple` and `/auth/refresh` 200s off a proxied real sign-in, point the decoding tests at them, drop the dead
waivers. It is the first PR that changes Swift, so the first the lanes judge and the first with records to measure.
PR #1's two reviewer warnings (cancellation at the `APIClient` boundary, `RootView`'s placeholder copy) wait for a
`bugfix/` branch after it. Blocked on nothing.

## Stages

| # | Feature | Status | Notes |
|---|---|---|---|
| 1 | Authentication | 🟡 | `docs/features/auth.md`. Apple and Google sign-in and restore work live. Live fixtures, Keychain tests left. |
| 2 | Dashboard + Quick Log | 🟡 | `DashboardModel` tested against a stub. No views, no real repository. |
| 3 | Insights + Recaps | ⬜ | Swift Charts; polls for server-side generation. |
| 4 | Onboarding + Paywall | ⬜ | Survey polling, RevenueCat. The Flutter login screen bundles this survey ahead of sign-in. |

Legend: ⬜ not started · 🟡 in progress · ✅ done · ⏸ deferred

**The pipeline is done when** (`README.md` §4): twenty product PRs through the gate; every lane with a measured noise rate; one lane granted, refused or killed on data; one defect the gate caught that a full read would have missed, documented; behaviour-first intake on three features. The gate now changes only when the load says it should.

## Known gaps

- **No captured fixture for `/auth/apple` or `/auth/refresh` 200** — neither is scriptable, so decoding runs against
  inline bodies labelled as constructed, and dead `swiftgate:allow` comments in `AuthEndpoints.swift` and
  `DateProvider.swift` name rules that are gone. Capture both off a proxied sign-in (`auth.md` step 5).
- **The live refresh has never been observed** (restore only ran inside the 15-minute window), and **`Persistence`
  has no test target** — the verification lane reports that on any PR touching it. `auth.md` step 6.
- **Sign-out leaves the Firebase user signed in** — the seam has no sign-out. Fine until account deletion.
- **A transient restore failure parks in `restoring` with a retry** — correct only because no user row is cached — and
  **the local store is not observable**. SwiftData, the outbox, ADR 0003's gate: Stage 2.
- No `PrivacyInfo.xcprivacy`, usage descriptions, or a HealthKit data boundary (Guideline 5.1.3) — all before submission.
- No brand colour or app icon — `AccentColor` is empty, so the app tints system blue. Analytics records events only.

## Deliberately not doing

- **No feature parity with Flutter.** Health sync, Events, Tags, Reminders, Settings are out of scope; add only by decision.
- **No App Intents, Control, or widgets in v1** (ADR 0008); **no API versioning shim** — the backend has none.

## Open questions

- Is the Firebase indirection intentional? The server verifies **Firebase** ID tokens, so the app
  must carry the SDK to sign in with Apple at all. ADR 0010 makes either answer cheap to act on.
- What is Mindlens's native palette? Flutter's `#5A6DF0` fails WCAG AA on white and ships no dark mode.
