# Authentication

Status: 🟡 · Stage 1 in `docs/STATE.md` · Decisions: ADR 0004, 0005, 0010

## What the user can do

Answer a four-question survey, then sign in with Apple or Google. The same screen serves
sign-up and sign-in. A new account keeps its answers; a returning one skips posting them.
Stay signed in across launches, and sign out.

**Done when:** a new account's survey answers reach the server and it lands past onboarding; a
real sign-in survives a relaunch and a 15-minute token expiry; the two auth fixtures are live captures.

## Behaviour

*Approved by Fedir, 2026-09-22 (ADR 0020).* Checked against the source app (`login_view.dart`,
`login_bloc.dart`). Where we differ on purpose, the line says so.

**Flows**
- *Survey.* Five pages in order, driven by buttons only (no swipe): intro → goal → feeling →
  hurdle → sign-in. Back goes one page; the intro has none. A progress indicator shows page n/5.
  There is no skip. Answers live in memory only, so relaunching starts over.
- *Sign in.* Tap Apple or Google → the system sheet → Firebase ID token → `POST /auth/apple` or
  `/auth/google`. If `isOnboardingComplete == false`, the answers are posted: create a baseline if
  none exists, create goals if none exist, then complete onboarding. Then the user is signed in.
  A returning account discards the answers.
- *Cancel.* Dismissing the sheet returns to idle with no message.
- *Launch.* A stored session skips the survey and shows progress while `GET /users` checks it.
  200 → signed in. A rejected session with a rejected refresh → signed out and cleared. Offline or
  5xx → a retry screen. We differ here: the source app goes in on a cached id, and we have no
  cached row until Stage 2.
- *Sign out.* `POST /auth/logout`, which may fail. Local state is cleared regardless; the device
  GUID stays. The "Sign out?" confirm belongs to Settings, which is out of scope.
- After sign-in, a new account lands on the onboarding stub. Stage 4 fills in the insight reveal
  and the paywall.

**Screens** — Survey pages 0–4 (`SignInView` is page 4): each page's idle and selected states;
Continue disabled on goal and hurdle until an answer is chosen. Sign-in page: idle · signing in
(per provider, the other button disabled) · error. `RootView`: restoring · restore failed with
retry · survey · onboarding stub · signed-in stub. Previews cover every state.

**Copy** (verbatim from the source app)
- Intro: "See what’s behind" / "your good and bad days" · "Continue"
- Goal: "What would you like to improve first?"; single choice: Recover from burnout · Clear brain
  fog · Stop overthinking · Build consistent routines · Fix my sleep · Reduce screen time.
- Feeling: "How do you feel lately?"; a whole-number slider from 1 to 10, starting at 5, labelled
  "Tough" … "Great", with the value shown.
- Hurdle: "What usually gets in the way?"; single choice: Constant exhaustion · Getting
  overwhelmed · Losing momentum · Overthinking · All-or-nothing mindset.
- Sign-in: "One last step" / "Sign in to turn your answers into your first lens." · "Continue with
  Apple" (system) · "Continue with Google" · "We respect your privacy. By continuing, you agree to
  our Terms of Use and Privacy Policy."
- Errors: 422 → "We couldn't verify that account. Try again, and choose Share My Email." Restore →
  "Can't reach Mindlens" / "Try Again". Everything else → the shared message. Provider errors
  never show raw text (the source app does).

**API calls** — `POST /auth/apple` {idToken, guid, deviceModel, timezone, email?} or `/auth/google`
(no email) → `data.user`, `data.tokens`. New account only: `GET /baseline/latest` and `GET /goals`
(404 or empty means none) → `POST /baseline` {motivationScore 1–10, anticipatedHurdle: the English
label} → `POST /goals` {titles: [goal id, e.g. `fix_sleep`]} → `POST /users/complete-onboarding`.
`GET /users` on restore. `POST /auth/refresh` takes the refresh token as bearer; its `data` is the
bare pair, single-use. `POST /auth/logout` → 204.

**Edge cases**
- Offline sign-in → the shared message, and the buttons come back.
- Several 401s at once → one refresh, and each request replays once. Refresh rejected → signed out
  silently. Refresh transient → the session stays and the request fails.
- No Firebase plist → sign-in reports unavailable.
- Posting the answers fails after the tokens are saved → stay on the sign-in page with the error.
  Retry re-posts the answers without reopening the provider sheet and posts nothing twice. (In the
  source app a relaunch here loses the answers and never completes onboarding.)
- No App Tracking Transparency prompt, since nothing tracks. It returns with AppsFlyer, where the
  source app asked for it on leaving the intro.

**Acceptance criteria**
- [ ] The survey sequence, copy and option lists match the Copy above; Continue is gated on goal and hurdle.
- [ ] A new account posts exactly baseline → goals → complete-onboarding with those wire values; a returning one posts none.
- [ ] A real Apple and Google sign-in each land signed in; relaunch restores without the survey.
- [ ] A session older than 15 minutes refreshes once on relaunch without signing out. Observed live.
- [ ] Concurrent 401s make exactly one `/auth/refresh` (a staggered test).
- [ ] The `/auth/apple` and `/auth/refresh` 200s are redacted live captures that the decoding tests read.
- [ ] No `swiftgate:allow` names a rule that no longer exists; Keychain paths run under a `Persistence` test target.

## Decisions — settled, do not reopen

- Sign in with Apple is the **system button**: black in light, white in dark, never restyled.
- The Google button is a plain `.bordered` button **paired to Apple's per the HIG** — same
  height via `.controlSize(.large)`, same 6pt radius, label range capped at `.xxxLarge` as
  Apple's own is. Unbranded until Google's mark is an asset; never `.borderedProminent`.
- **One `SessionModel`** for restore, sign-in and sign-out. No `SignInModel` beside it.
- The Firebase exchange sits behind `IdentityAuthenticating` in `Core`; the SDK is
  app-target only. `APIAuthRepository` lives in `Networking` so the DTO never leaves it (ADR 0010).
- **The plist decides, not the build.** `GoogleService-Info.plist` is gitignored, so CI and a
  fresh clone launch without it: the composition root configures Firebase only when the file is
  bundled and keeps `UnavailableIdentityProvider` otherwise. Debug only — a Release build with
  no plist is misconfigured and hits `preconditionFailure`, as a missing API URL does.
- Firebase's own error is classified in the provider, once: `.networkError` → `.offline`,
  everything else `.unknown` with the code in the diagnostic. Feature code never sees `NSError`.
- **The survey belongs to auth** (Fedir, 2026-09-22). The sign-in screen is the survey's last page
  for sign-up and sign-in alike. This supersedes "the survey is Stage 4's".
- A transient restore failure parks in `restoring` with a retry. Only a server-rejected session
  signs out, and that also ends it locally. Sign-out is best-effort remote, unconditional local.
- The device GUID survives sign-out — the server keeps five sessions per user, LRU.
- 422 gets its own copy (Share My Email); every other error uses the shared message.
- Leading-aligned headline, no glowing background: a custom-drawn wash never adopts Liquid Glass.

## Steps

1. ✅ Sign-in screen and session gate, verified in the simulator at both text sizes and appearances.
2. ✅ `APIAuthRepository`, `AuthTokenRefreshTransport`, `KeychainDeviceIdentity`; 79 tests; gate launches the app.
3. ✅ Firebase Auth (app target only) and Sign in with Apple, run for real; entitlement and team asserted by the gate.
4. ✅ Google Sign-In through the same seam, run for real; the redirect scheme asserted against the bundled plist.
5. ⬜ **Capture the two auth fixtures and drop the waiver** — save the `/auth/apple` 200 and a
   `/auth/refresh` 200 during a real sign-in, point the decoding tests at them, delete the
   inline bodies. Both are one-shot from the client (the refresh token rotates), so capture them
   off the app's own traffic through a proxy — never `curl /auth/refresh` beside a live session.
   entries: the waiver in `Packages/MindlensKit/Sources/Networking/AuthEndpoints.swift` and the dead one
   in `Packages/MindlensKit/Sources/Core/DateProvider.swift` · `AuthResponseDecodingTests` and the
   inline bodies in `APIAuthRepositoryTests` and `AuthTokenRefreshTransportTests` ·
   `Tools/capture-fixtures.sh` (which cannot script these two).
   files: `Packages/MindlensKit/Sources/TestSupport/Fixtures/`, `Packages/MindlensKit/Tests/NetworkingTests/AuthContractTests.swift`
   ready: no `swiftgate:allow` names a missing rule; the fixtures README lists both as live and redacted.
6. ⬜ `Persistence` test target for the Keychain paths, which have run for real but never under a test.
7. ⬜ **The survey** — pages 0–3 ahead of `SignInView` and a new account's answers posted after
   sign-in, per Behaviour. Shaped in full when it opens.

## Journal

- 2026-09-11 — Review: two `TokenRefresher` bugs (a stale refresh over a new session; `defer`
  unregistering the wrong task). The test that claimed to cover them never called the
  transport. Fixed, with tests that make the interleaving instead of racing for it.
- 2026-09-11 — Google button paired to Apple's per the HIG. Previews for every state.
- 2026-09-11 — Firebase Auth linked, `FirebaseIdentityProvider` written, entitlement signed;
  the stand-in stays as the no-plist fallback since CI never has one. `codesign` reads an
  *empty* set off a simulator build — the truth is `__TEXT,__entitlements`, which the gate now
  decodes. Open: sign-out leaves the Firebase user signed in (no sign-out on the seam; account
  deletion will want it, plus the authorization code). SPM fetches every Firebase binary, ~1 GB.
- 2026-09-11 — First real run. `-7022` from AuthKit: no `DEVELOPMENT_TEAM`, so Xcode guessed a
  team that did not own the App ID. Then a 422 nothing logged — `AppError.diagnostic` had never
  been read; `Logger(category:)` in `Core` and `SessionModel.failed()` fix that. The 422 was a
  plist from a second Firebase project; the API verifies only production's.
- 2026-09-11 — Native bundle ID registered as a second iOS app in the production project. First
  real sign-in landed on the signed-in stub; relaunch restored the session (Keychain →
  `GET /users`). Step 3 ticked. Sign-ins hit production accounts — there is no other backend.
- 2026-09-11 — Google wired (step 4), unrun: `GoogleSignIn-iOS` 10.0.0 fits Firebase 12.19's
  graph (GTMSessionFetcher `3.3..<6`). The SDK asserts the redirect scheme with an ObjC
  exception at the tap; the provider derives it from the client ID and throws first.
- 2026-09-19 — `Tools/check-build-settings.sh` asserts the bundled plist's `REVERSED_CLIENT_ID` is a declared
  URL scheme in the built Info.plist; shown red before the scheme exists. The value is still pasted by hand.
- 2026-09-19 — First real Google sign-in landed on the signed-in stub. Step 4 ticked.
