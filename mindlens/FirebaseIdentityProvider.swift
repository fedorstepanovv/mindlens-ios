import Core
import FirebaseAuth
import FirebaseCore
import Foundation
import GoogleSignIn
import UIKit

/// The Firebase half of sign-in: a native provider's credential in, a Firebase ID token out.
///
/// The one file that names the Firebase and Google SDKs. It lives in the app target and nothing
/// but plain values crosses `IdentityAuthenticating`, so the package builds and every test runs
/// without an SDK or a credential in sight (ADR 0005, ADR 0010).
struct FirebaseIdentityProvider: IdentityAuthenticating {
    /// The OAuth client Google Sign-In presents as. `nil` when the plist has no `CLIENT_ID`,
    /// which is what the console emits before Google is enabled as a provider.
    private let googleClientID: String?
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
        // Google's SDK would otherwise look for `GIDClientID` in Info.plist — a second copy of a
        // value the plist already carries, and one that would go stale with it.
        if let clientID = options.clientID {
            GIDSignIn.sharedInstance.configuration = GIDConfiguration(clientID: clientID)
        }
        return FirebaseIdentityProvider(googleClientID: options.clientID)
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

    /// Google's SDK presents its own sheet — `ASWebAuthenticationSession` on our iOS floor — so
    /// unlike Apple's half there is no credential to bring in; it needs only a window to anchor
    /// to. The redirect comes back through the session, never through an app URL open, so no
    /// `.onOpenURL` handling exists for it.
    func signInWithGoogle() async throws -> IdentityCredential {
        guard let googleClientID else {
            throw AppError(
                kind: .unknown,
                diagnostic: "GoogleService-Info.plist has no CLIENT_ID; enable Google, re-download it."
            )
        }
        // The SDK checks this itself and raises an Objective-C exception when it is missing. The
        // same fact as a thrown error reaches the session log instead of crashing the app.
        let scheme = Self.reversedClientID(googleClientID)
        guard Self.registeredURLSchemes().contains(scheme) else {
            throw AppError(
                kind: .unknown,
                diagnostic: "URL scheme \(scheme) is not in Info.plist; Google cannot redirect back."
            )
        }
        guard let presenter = Self.presentingViewController() else {
            throw AppError(kind: .unknown, diagnostic: "No key window to present Google Sign-In from.")
        }

        do {
            let google = try await GIDSignIn.sharedInstance.signIn(withPresenting: presenter).user
            guard let idToken = google.idToken?.tokenString else {
                throw AppError(kind: .unknown, diagnostic: "Google signed in without an ID token.")
            }
            let credential = GoogleAuthProvider.credential(
                withIDToken: idToken, accessToken: google.accessToken.tokenString
            )
            let user = try await Auth.auth().signIn(with: credential).user
            let email = user.email
            let token = try await user.getIDToken()
            return IdentityCredential(firebaseIDToken: token, email: email)
        } catch let error as GIDSignInError where error.code == .canceled {
            // The user closed the sheet. `IdentityAuthenticating` promises `CancellationError`
            // for exactly this, so the model returns to idle without a banner.
            throw CancellationError()
        } catch {
            throw Self.appError(error)
        }
    }

    // MARK: - Google plumbing

    /// `123-abc.apps.googleusercontent.com` → `com.googleusercontent.apps.123-abc`, the custom
    /// scheme Google redirects to. It has to be declared statically in Info.plist; the value is
    /// the plist's `REVERSED_CLIENT_ID`.
    private static func reversedClientID(_ clientID: String) -> String {
        clientID.split(separator: ".").reversed().joined(separator: ".")
    }

    private static func registeredURLSchemes(in bundle: Bundle = .main) -> Set<String> {
        let types = bundle.object(forInfoDictionaryKey: "CFBundleURLTypes") as? [[String: Any]] ?? []
        return Set(types.flatMap { $0["CFBundleURLSchemes"] as? [String] ?? [] })
    }

    /// The key window's root, which `ASWebAuthenticationSession` uses as its presentation anchor.
    private static func presentingViewController() -> UIViewController? {
        UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap { $0.windows }
            .first { $0.isKeyWindow }?
            .rootViewController
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
