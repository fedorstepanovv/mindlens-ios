import Core
import FirebaseAuth
import FirebaseCore
import Foundation

/// The Firebase half of sign-in: a native provider's credential in, a Firebase ID token out.
///
/// The one file that names the Firebase SDK. It lives in the app target and nothing but plain
/// values crosses `IdentityAuthenticating`, so the package builds and every test runs without
/// the SDK or a credential in sight (ADR 0005, ADR 0010).
struct FirebaseIdentityProvider: IdentityAuthenticating {
    /// Configures Firebase from the bundled `GoogleService-Info.plist` and returns a provider
    /// bound to it — or `nil` when there is no plist, in which case Firebase is not touched.
    ///
    /// The plist is a credential and is gitignored, so CI and a fresh clone launch without one.
    /// `FirebaseApp.configure()` with no plist raises an Objective-C exception, and it would do
    /// so on the first line of the composition root: a crash the gate's launch test would catch,
    /// but a crash all the same. No plist, no Firebase — the caller keeps
    /// `UnavailableIdentityProvider`, and a tap on Sign in stops where `docs/STATE.md` says.
    ///
    /// Call it **once**, from the composition root. Configuring twice raises, and there is no way
    /// to ask first: every public "is it configured" accessor — `FirebaseApp.app()`,
    /// `FirebaseApp.allApps` — logs a `[FirebaseCore] not yet configured` *error* when the
    /// answer is no, which reads as a misconfiguration in the console of a build that is fine.
    static func configuringFirebase(from bundle: Bundle = .main) -> FirebaseIdentityProvider? {
        guard let path = bundle.path(forResource: "GoogleService-Info", ofType: "plist"),
            let options = FirebaseOptions(contentsOfFile: path)
        else {
            return nil
        }
        FirebaseApp.configure(options: options)
        return FirebaseIdentityProvider()
    }

    func exchangeAppleCredential(_ request: AppleIdentityRequest) async throws -> IdentityCredential {
        // Firebase re-hashes the raw nonce and compares it with the hash Apple signed into the
        // identity token. `fullName` would only set a display name on the Firebase user, which
        // nothing reads: the server takes a token and an email.
        let credential = OAuthProvider.appleCredential(
            withIDToken: request.identityToken, rawNonce: request.rawNonce, fullName: nil
        )
        do {
            let user = try await Auth.auth().signIn(with: credential).user
            // Apple sends the address once, on first authorization, and Firebase keeps it on the
            // user record — so a returning user still has one to give the server. The repository
            // falls back to Apple's own when even this is empty.
            let email = user.email
            let token = try await user.getIDToken()
            return IdentityCredential(firebaseIDToken: token, email: email)
        } catch {
            throw Self.appError(error)
        }
    }

    func signInWithGoogle() async throws -> IdentityCredential {
        throw AppError(
            kind: .unknown,
            diagnostic: "Google Sign-In is not wired — docs/features/auth.md, step 4."
        )
    }

    /// The vendor error is classified here, at the boundary, because `AppError(_:)` cannot know
    /// Firebase's domain. Only one distinction matters to the screen: its network error is
    /// `.offline`, which is retryable and gets its own copy. Everything else is `.unknown`, with
    /// the code kept for the log.
    private static func appError(_ error: any Error) -> AppError {
        let nsError = error as NSError
        guard nsError.domain == AuthErrors.domain else {
            return AppError(error)
        }
        let isNetwork = AuthErrorCode(rawValue: nsError.code) == .networkError
        return AppError(
            kind: isNetwork ? .offline : .unknown,
            diagnostic: "Firebase Auth \(nsError.code): \(nsError.localizedDescription)"
        )
    }
}
