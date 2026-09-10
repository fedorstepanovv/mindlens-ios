import Foundation

/// Drives the gate at the `Scene` root.
///
/// A `switch` on this decides which tree exists at all — unlike a router redirect, an
/// authenticated view is not merely unreachable while signed out, it is never built.
public enum SessionState: Sendable, Equatable {
    case restoring
    case signedOut
    case onboarding(User)
    case signedIn(User)
}

public extension SessionState {
    /// Having a session is not the same as being finished with onboarding.
    ///
    /// The server tracks completion on the user row, and the same sign-in path serves a
    /// brand-new account and a returning one — so an account that abandoned onboarding
    /// resumes there rather than landing on a dashboard it has no data for.
    init(authenticated user: User) {
        self = user.isOnboardingComplete ? .signedIn(user) : .onboarding(user)
    }

    /// The user, whichever authenticated case this is.
    var user: User? {
        switch self {
        case .onboarding(let user), .signedIn(let user): user
        case .restoring, .signedOut: nil
        }
    }
}
