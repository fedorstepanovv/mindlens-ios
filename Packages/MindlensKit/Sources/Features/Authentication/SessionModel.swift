import Analytics
import Core
import Foundation
import Models
import Observation

/// The whole session lifecycle: restore at launch, sign in, sign out.
///
/// **One model, not two.** A separate `SignInModel` beside this would split a single flow —
/// tap, provider round trip, session established, gate changes — across two types that have
/// to stay in step, which is the ViewModel-per-screen habit arriving by a different route.
/// `pending` and `error` are this flow's presentation state and belong with the state they
/// describe.
@Observable
@MainActor
public final class SessionModel {
    /// What tree exists at the `Scene` root. A `switch`, not a redirect: the signed-in views
    /// are never built while signed out.
    public private(set) var state: SessionState = .restoring

    /// The provider whose flow is in flight, or `nil`. Drives per-button progress.
    public private(set) var pending: SignInProvider?

    public private(set) var error: AppError?

    /// Held between `appleRequestNonce()` and the credential coming back. Never leaves.
    private var appleNonce: SignInNonce?

    private let auth: any AuthRepository
    private let analytics: any AnalyticsRecording

    public init(auth: any AuthRepository, analytics: any AnalyticsRecording = .noop) {
        self.auth = auth
        self.analytics = analytics
    }

    public var isSigningIn: Bool { pending != nil }

    /// A launch restore that failed for a reason that is **not** "your session ended".
    ///
    /// Tokens are still good; the network was not. The root shows a retry rather than the
    /// sign-in screen, because dropping a user to sign-in over a dead tunnel is
    /// indistinguishable from being logged out, and they may not be able to sign back in.
    public var needsRestoreRetry: Bool { state == .restoring && error != nil }

    public func restore() async {
        error = nil

        do {
            if let user = try await auth.restore() {
                state = SessionState(authenticated: user)
            } else {
                state = .signedOut
            }
        } catch let appError as AppError where appError.kind == .unauthenticated {
            // The server rejected the stored session. This is the only error that signs out.
            state = .signedOut
        } catch is CancellationError {
            // The scene went away mid-restore.
        } catch {
            self.error = AppError(error)
        }
    }

    /// The **hashed** nonce for a Sign in with Apple request; the raw half stays here until
    /// the credential comes back, which is the point of the nonce.
    public func appleRequestNonce() -> String {
        let nonce = SignInNonce.generate()
        appleNonce = nonce
        return nonce.hashed
    }

    /// - Note: takes plain values rather than `ASAuthorizationAppleIDCredential` so this is
    ///   testable — that type has no public initializer.
    public func signInWithApple(
        identityToken: String,
        authorizationCode: String?,
        email: String?
    ) async {
        guard let nonce = appleNonce else {
            error = AppError(kind: .unknown, diagnostic: "Apple credential arrived with no nonce")
            return
        }
        appleNonce = nil  // Single use, whatever happens next.

        await signIn(with: .apple) {
            try await auth.signInWithApple(
                AppleIdentityRequest(
                    identityToken: identityToken,
                    rawNonce: nonce.raw,
                    authorizationCode: authorizationCode,
                    email: email
                )
            )
        }
    }

    public func signInWithGoogle() async {
        await signIn(with: .google) { try await auth.signInWithGoogle() }
    }

    /// The user dismissed the provider's sheet. Not a failure, and no banner.
    public func signInWasCancelled() {
        appleNonce = nil
        pending = nil
    }

    /// The provider's own flow failed before there was ever a credential to exchange.
    ///
    /// Separate from `signInWasCancelled()` because the two are told apart by a framework
    /// error code, which is the view's business — and because getting it wrong in either
    /// direction is visible: a banner for every dismissed sheet, or silence on a real failure.
    public func signInFailed(_ error: any Error) {
        appleNonce = nil
        pending = nil
        self.error = AppError(error)
    }

    public func signOut() async {
        await auth.signOut()
        state = .signedOut
        error = nil
        analytics.record(AnalyticsEvent("sign_out"))
    }

    // MARK: - Private

    private func signIn(with provider: SignInProvider, _ authenticate: () async throws -> User) async {
        pending = provider
        error = nil
        defer { pending = nil }

        do {
            let user = try await authenticate()
            state = SessionState(authenticated: user)

            // One path serves sign-up and sign-in, and the server returns no "is new" flag —
            // so an incomplete onboarding stands in for a fresh account. It over-counts a
            // user who abandons onboarding and returns; that is the same trade the existing
            // product makes, and it is a funnel signal rather than a reported metric.
            analytics.record(
                AnalyticsEvent(
                    "sign_in_completed",
                    properties: [
                        "provider": provider.rawValue,
                        "new_account": String(!user.isOnboardingComplete),
                    ]
                )
            )
        } catch is CancellationError {
            // Cancelling is not a failure — the same rule as a cancelled load.
        } catch {
            self.error = AppError(error)
        }
    }
}
