#if DEBUG
import Core
import Foundation

public extension User {
    /// A user for previews. Next to the model rather than in `TestSupport`, because
    /// `TestSupport` is never linked into a feature or the app, and a preview needs one.
    static let preview = User(
        id: 1,
        email: "preview@example.com",
        authProvider: .apple,
        timezone: "Europe/Kyiv",
        isOnboardingComplete: true
    )
}

/// An `AuthRepository` for previews: scripted outcomes, no server.
///
/// `DEBUG`-only, so it does not ship. It lives beside the protocol for the same reason
/// `User.preview` does — the richer `StubAuthRepository` in `TestSupport` cannot be reached from
/// a preview, and every screen behind the session gate needs one of these to be previewable.
public struct PreviewAuthRepository: AuthRepository {
    public enum Outcome: Sendable {
        case succeeds(User)
        case fails(AppError)
        /// Never completes — the state a screen shows *while* something is happening.
        case hangs
    }

    public var restore: Outcome
    public var signIn: Outcome

    public init(
        restore: Outcome = .fails(AppError(kind: .unauthenticated)), signIn: Outcome = .succeeds(.preview)
    ) {
        self.restore = restore
        self.signIn = signIn
    }

    public func restore() async throws -> User? { try await resolve(restore) }
    public func signInWithApple(_ request: AppleIdentityRequest) async throws -> User {
        try await resolve(signIn)
    }
    public func signInWithGoogle() async throws -> User { try await resolve(signIn) }
    public func signOut() async {}

    private func resolve(_ outcome: Outcome) async throws -> User {
        switch outcome {
        case .succeeds(let user):
            return user
        case .fails(let error):
            throw error
        case .hangs:
            try await Task.sleep(for: .seconds(3600))
            throw CancellationError()
        }
    }
}
#endif
