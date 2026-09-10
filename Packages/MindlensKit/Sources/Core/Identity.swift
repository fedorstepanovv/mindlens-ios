import CryptoKit
import Foundation

/// Which sign-in button the user tapped.
///
/// Deliberately *not* `Models.AuthProvider`. That type is what the server says an
/// existing account was created with, and it carries an `unknown` case so a provider
/// added after this build ships still decodes. There is no unknown provider to sign in
/// *with*, and a value the UI offers should not have a case the UI cannot render.
public enum SignInProvider: String, Sendable, CaseIterable, Identifiable {
    case apple
    case google

    public var id: String { rawValue }
}

/// What the backend actually wants in exchange for a session: a **Firebase ID token**.
///
/// The server verifies Firebase tokens rather than raw Apple/Google credentials, so
/// signing in natively is a two-step exchange. See `docs/API.md` § Authentication.
public struct IdentityCredential: Sendable, Equatable {
    public let firebaseIDToken: String

    /// Apple returns an email address **only on the first authorization** for an app, and
    /// a private-relay address at that. The server needs one to create the account and
    /// answers 422 without it, so it travels beside the token instead of being assumed to
    /// be inside it.
    public let email: String?

    public init(firebaseIDToken: String, email: String?) {
        self.firebaseIDToken = firebaseIDToken
        self.email = email
    }
}

/// A single-use nonce binding one Apple authorization to one Firebase exchange.
public struct SignInNonce: Sendable, Equatable {
    /// Kept locally and handed to the identity exchange, never to Apple.
    public let raw: String

    public init(raw: String) {
        self.raw = raw
    }

    /// The SHA-256 of `raw`, which is what goes into the Apple request.
    ///
    /// Apple echoes this hash inside the identity token it returns; the identity provider
    /// re-hashes `raw` and compares. Sending `raw` to Apple instead would let anyone who
    /// captured the token replay it, which is the whole reason the nonce is hashed.
    public var hashed: String {
        SHA256.hash(data: Data(raw.utf8))
            .map { String(format: "%02x", $0) }
            .joined()
    }

    /// 32 bytes from the system CSPRNG, hex-encoded.
    public static func generate() -> SignInNonce {
        var bytes = [UInt8](repeating: 0, count: 32)
        var generator = SystemRandomNumberGenerator()
        for index in bytes.indices {
            bytes[index] = generator.next()
        }
        return SignInNonce(raw: bytes.map { String(format: "%02x", $0) }.joined())
    }
}

/// The result of a Sign in with Apple authorization, reduced to plain values.
///
/// `ASAuthorizationAppleIDCredential` does not appear here on purpose: the view runs the
/// system authorization, and everything past that point is provider work.
public struct AppleIdentityRequest: Sendable, Equatable {
    public let identityToken: String
    /// The **raw** nonce whose hash went into the Apple request.
    public let rawNonce: String
    public let authorizationCode: String?
    /// Apple's email, present only on first authorization.
    public let email: String?

    public init(identityToken: String, rawNonce: String, authorizationCode: String?, email: String?) {
        self.identityToken = identityToken
        self.rawNonce = rawNonce
        self.authorizationCode = authorizationCode
        self.email = email
    }
}

/// Exchanging a provider credential for an identity token, behind a protocol we own.
///
/// ADR 0005: the Firebase SDK is a dependency of the **app target only** and no vendor
/// type crosses this boundary, so the package builds and the whole suite runs with no
/// credentials configured at all.
///
/// The Apple and Google halves are shaped differently because the flows genuinely are:
/// SwiftUI's `SignInWithAppleButton` runs Apple's authorization itself — so the view
/// obtains a nonce, lets the system button do its work, and brings the credential back
/// here — whereas Google's SDK presents its own UI and needs no help from us.
///
/// **A user cancelling is not a failure.** Implementations throw `CancellationError` so
/// callers can return to idle without painting an error banner, which is the same rule
/// `docs/PATTERNS.md` applies to cancelled loads.
public protocol IdentityAuthenticating: Sendable {
    func exchangeAppleCredential(_ request: AppleIdentityRequest) async throws -> IdentityCredential
    func signInWithGoogle() async throws -> IdentityCredential
}
