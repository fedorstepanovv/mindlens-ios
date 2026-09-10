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
