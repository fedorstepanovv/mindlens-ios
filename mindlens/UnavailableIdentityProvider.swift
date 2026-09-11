import Core
import Foundation

/// What sign-in runs against when no `GoogleService-Info.plist` is bundled.
///
/// The backend verifies **Firebase ID tokens** rather than Apple's or Google's own, so a real
/// exchange needs `FirebaseIdentityProvider` and the plist that configures it. The plist is a
/// credential and is gitignored, so CI and a fresh clone have none — and rather than crash at
/// launch, the composition root wires this instead. See `docs/STATE.md`.
///
/// It exists rather than being left out so the boundary is real and the gap is visible: every
/// other part of sign-in is wired and tested against this, and a tap on either button reaches
/// exactly this far. ADR 0010.
struct UnavailableIdentityProvider: IdentityAuthenticating {
    func exchangeAppleCredential(_ request: AppleIdentityRequest) async throws -> IdentityCredential {
        throw Self.notConfigured
    }

    func signInWithGoogle() async throws -> IdentityCredential {
        throw Self.notConfigured
    }

    private static var notConfigured: AppError {
        AppError(
            kind: .unknown,
            diagnostic: "No identity provider — there is no GoogleService-Info.plist in the bundle."
        )
    }
}
