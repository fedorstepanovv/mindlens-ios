# ADR 0010 — Identity exchange behind a protocol, and the auth repository in Networking

Date: 2026-09-11 · Status: Accepted

## Context
The backend verifies **Firebase ID tokens**, not Apple's or Google's own credentials. Signing
in is therefore two exchanges: native provider → Firebase ID token → `POST /auth/apple`.

That has two consequences the client cannot avoid. Signing in for real needs the Firebase Auth
SDK and a `GoogleService-Info.plist`, neither of which is in this repository. And every layer
of sign-in above that exchange — device GUID, request shape, token adoption, the session gate —
is testable without it.

Separately: `AuthRepository` needs `APIClient`, `TokenRefresher`, `TokenStorage` and the
identity exchange. Putting the concrete repository in the Authentication feature would make
`AuthSessionDTO` public so the feature could name it, which is the coupling the repository
layer exists to remove.

## Decision
Two boundaries.

**`IdentityAuthenticating` in `Core`.** Only plain `Sendable` values cross it —
`AppleIdentityRequest` in, `IdentityCredential` out. No vendor type appears in a signature, and
the Firebase SDK will be a dependency of the **app target only**, bound at the composition
root, exactly as ADR 0005 requires of every other SDK. Until it is linked,
`UnavailableIdentityProvider` in the app target throws.

**`APIAuthRepository` in `Networking`.** Its collaborators are all `Core` protocols plus the
client next door, so it needs nothing from a feature, and `AuthSessionDTO` stays internal to
`Networking`. `Features/Authentication` depends on neither `Networking` nor `Persistence` — it
takes `any AuthRepository` from `Models`.

The Apple half of the protocol is split from the Google half because the flows differ:
SwiftUI's `SignInWithAppleButton` runs the authorization itself, so the view obtains a nonce
and brings the credential back, while Google's SDK presents its own UI.

## Consequences
The whole of Stage 1 is built and tested with no credentials, no SDK and no network — 75 tests,
about two seconds. Wiring Firebase later touches one file in the app target.

The feature target links four modules instead of six, so it does not rebuild when the
networking layer changes.

The cost is honest: **sign-in cannot actually complete in this build.** Everything up to the
token exchange runs; the exchange throws. That is a visible gap rather than a hidden one, and
`docs/STATE.md` records it as the next action.

A second cost: `AppleIdentityRequest` is built by the view, from
`ASAuthorizationAppleIDCredential`. That type has no public initializer, so the model takes
plain values and the extraction is the one piece of this flow no unit test covers.

## What would change this
The server accepting Apple and Google tokens directly, which would delete the Firebase
dependency and leave this protocol with one implementation and no reason to exist. Whether that
indirection is intentional is still an open question in `docs/STATE.md` — this boundary is what
makes answering it cheap either way.
