import Core
import Foundation

/// Everything the app can do to a session.
///
/// The protocol lives in `Models` rather than in the Authentication feature, so a feature
/// that needs to end a session — Settings, eventually — does not have to import
/// Authentication to do it. That is the rule that keeps features from needing each other.
public protocol AuthRepository: Sendable {
    /// Re-establishes a session from what is already on disk.
    ///
    /// - Returns: the signed-in user, or `nil` when there is nothing stored to restore.
    /// - Throws: `AppError` with kind `.unauthenticated` when the stored session is
    ///   genuinely over. **Anything else thrown is transient** — a cold launch with no
    ///   network must not be read as a sign-out.
    func restore() async throws -> User?

    func signInWithApple(_ request: AppleIdentityRequest) async throws -> User
    func signInWithGoogle() async throws -> User

    /// Ends the session locally whatever the server says. Never throws: a user who asked
    /// to sign out is signed out, and a failed round trip leaves a server-side session
    /// that expires on its own.
    func signOut() async
}
