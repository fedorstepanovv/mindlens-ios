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
3. 🟡 **Link Firebase Auth and replace the stand-in** — the SDK in the app target only, an
   `OAuthProvider` credential for `apple.com` from the identity token and raw nonce, signed
   in and exchanged for the ID token. A user cancelling throws `CancellationError`.
   entries: `IdentityAuthenticating` in `Core` (the protocol to implement) ·
   `UnavailableIdentityProvider` (what it replaces) · `AppContainer.init` (where it is
   wired) · `SignInView.handleAppleCompletion` (where Apple's credential arrives) ·
   `MindlensApp` (Firebase is configured at launch).
   files: `mindlens/AppContainer.swift`, `mindlens/mindlensApp.swift`, `mindlens.xcodeproj/project.pbxproj`,
   FirebaseIdentityProvider.swift *(new)*, mindlens.entitlements *(new)*, GoogleService-Info.plist *(new)*
   ready: a real Sign in with Apple lands on the signed-in stub, and relaunching skips the
   sign-in screen. `swift test` still runs with no credentials.
   blocked on: the plist and the Sign in with Apple capability — both from Fedir, neither in the repo.
4. ⬜ **Google Sign-In through the same seam** — GoogleSignIn presents, Firebase exchanges,
   the ID token comes back; cancel throws `CancellationError`; reversed client ID URL scheme.
   entries: `IdentityAuthenticating.signInWithGoogle()` · `SignInView.googleButton` ·
   the URL scheme in `mindlens/Info.plist`.
   files: FirebaseIdentityProvider.swift, `mindlens/Info.plist`, `mindlens.xcodeproj/project.pbxproj`
   ready: "Continue with Google" completes and lands signed in.
5. ⬜ **Capture the two auth fixtures and drop the waiver** — save the `/auth/apple` 200 and a
   `/auth/refresh` 200 during a real sign-in, point the decoding tests at them, delete the
   inline bodies.
   entries: the waiver in `Packages/MindlensKit/Sources/Networking/AuthEndpoints.swift` ·
   `AuthResponseDecodingTests` · `Tools/capture-fixtures.sh` (which cannot script these two).
   files: `Packages/MindlensKit/Sources/TestSupport/Fixtures/`, `Packages/MindlensKit/Tests/NetworkingTests/AuthContractTests.swift`
   ready: `Tools/swiftgate --static-only` passes with no waiver; the fixtures README lists both as live.
6. ⬜ `Persistence` test target for the Keychain paths. Shape it once 3 has run for real.

## Journal

- 2026-09-10 — Foundation: `TokenRefresher`, `APIClient`, Keychain token storage, `SessionState`.
- 2026-09-11 — Stage 1 built end to end behind the identity seam. A 422 is a third envelope
  shape — `status`, not `statusCode` — captured live.
- 2026-09-11 — Launch crash: `INFOPLIST_KEY_<custom>` is silently dropped by Xcode; a partial
  `Info.plist` instead. The guard now reads the built bundle, and the gate launches the app.
- 2026-09-11 — Running it broke the Apple button at accessibility sizes. It needs a width, an
  *exact* height inside 44–64, and a rebuild on a live text-size change.
- 2026-09-11 — Review: two `TokenRefresher` bugs (a stale refresh over a new session; `defer`
  unregistering the wrong task). The test that claimed to cover them never called the
  transport. Fixed, with tests that make the interleaving instead of racing for it.
- 2026-09-11 — Google button paired to Apple's per the HIG. Previews for every state.
