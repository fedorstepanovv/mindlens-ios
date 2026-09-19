# Authentication

Status: 🟡 · Stage 1 in `docs/STATE.md` · Decisions: ADR 0004, 0005, 0010

## What the user can do

Sign in with Apple or Google, stay signed in across launches, and sign out. A brand-new
account lands in onboarding; a returning one on the dashboard.

**Done when:** a real sign-in on a device produces a session that survives a relaunch and a
15-minute access-token expiry, and the two auth fixtures exist as live captures.

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
- Copy is the product's opening line ("See what’s behind / your good and bad days"). Flutter's
  "One last step" presumes the Stage 4 survey, which ships later.
- A transient restore failure parks in `restoring` with a retry. Only a server-rejected session
  signs out, and that also ends it locally. Sign-out is best-effort remote, unconditional local.
- The device GUID survives sign-out — the server keeps five sessions per user, LRU.
- 422 gets its own copy (Share My Email); every other error uses the shared message.
- Leading-aligned headline, no carousel, no glowing background: the survey and its
  choreography are Stage 4's, and a custom-drawn wash never adopts Liquid Glass.

## Screens

- `SignInView` (`Packages/MindlensKit/Sources/Features/Authentication/SignInView.swift`) —
  the signed-out branch. States: idle · signing in, per provider · error line above the buttons.
- `RootView` (`mindlens/RootView.swift`) — the gate. States: restoring · restore failed with
  retry · signed out → `SignInView` · onboarding *(stub)* · signed in *(stub, has Sign Out)*.
- Previews cover every state of both.

## Steps

1. ✅ Sign-in screen and session gate, verified in the simulator at both text sizes and appearances.
2. ✅ `APIAuthRepository`, `AuthTokenRefreshTransport`, `KeychainDeviceIdentity`; 79 tests; gate launches the app.
3. ✅ Firebase Auth linked (app target only); `FirebaseIdentityProvider` does the Apple exchange; entitlement
   and team asserted by the gate. A real sign-in lands signed in; relaunch restores. Plist: production project, gitignored.
4. ✅ Google Sign-In through the same seam: `FirebaseIdentityProvider.signInWithGoogle()` presents from
   the key window and exchanges through `GoogleAuthProvider`; the redirect scheme is `CFBundleURLTypes` in
   `mindlens/Info.plist`, asserted against the bundled plist by `Tools/check-build-settings.sh`. Ran for real.
5. ⬜ **Capture the two auth fixtures and drop the waiver** — save the `/auth/apple` 200 and a
   `/auth/refresh` 200 during a real sign-in, point the decoding tests at them, delete the
   inline bodies. Both are one-shot from the client (the refresh token rotates), so capture them
   off the app's own traffic through a proxy — never `curl /auth/refresh` beside a live session.
   entries: the waiver in `Packages/MindlensKit/Sources/Networking/AuthEndpoints.swift` ·
   `AuthResponseDecodingTests` · `Tools/capture-fixtures.sh` (which cannot script these two).
   files: `Packages/MindlensKit/Sources/TestSupport/Fixtures/`, `Packages/MindlensKit/Tests/NetworkingTests/AuthContractTests.swift`
   ready: `Tools/swiftgate --static-only` passes with no waiver; the fixtures README lists both as live.
6. ⬜ `Persistence` test target for the Keychain paths, which have now run for real but never
   under a test.

## Journal

- 2026-09-11 — Stage 1 built end to end behind the identity seam. A 422 is a third envelope
  shape — `status`, not `statusCode` — captured live.
- 2026-09-11 — Running it broke the Apple button at accessibility sizes. It needs a width, an
  *exact* height inside 44–64, and a rebuild on a live text-size change.
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
