import Core
import Foundation

/// Stands where the Firebase exchange will go.
///
/// The backend verifies **Firebase ID tokens** rather than Apple's or Google's own, so signing
/// in for real needs the Firebase Auth SDK plus a `GoogleService-Info.plist` — neither of which
/// is in this repository yet. See `docs/STATE.md`.
///
/// It exists rather than being left out so the boundary is real and the gap is visible: every
/// other part of sign-in is wired and tested against this, and swapping it for the SDK-backed
/// implementation touches this one file. ADR 0010.
struct UnavailableIdentityProvider: IdentityAuthenticating {
    func exchangeAppleCredential(_ request: AppleIdentityRequest) async throws -> IdentityCredential {
        throw Self.notWired
    }

    func signInWithGoogle() async throws -> IdentityCredential {
        throw Self.notWired
    }

    private static var notWired: AppError {
        AppError(
            kind: .unknown,
            diagnostic: "No identity provider is wired — the Firebase Auth SDK is not linked yet."
        )
    }
}
