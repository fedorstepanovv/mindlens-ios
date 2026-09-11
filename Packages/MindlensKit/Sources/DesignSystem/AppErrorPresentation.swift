import Core
import SwiftUI

/// Turns an `AppError` into words.
///
/// This is the only place user-facing error copy exists, and the only target with a
/// String Catalog — so `bundle: .module` resolves, which it does not from a target
/// without resources.
///
/// `@MainActor` because this target declares `defaultIsolation(MainActor.self)`, which
/// makes SPM's generated `Bundle.module` accessor main-actor isolated. That is the right
/// answer rather than a workaround: this is presentation copy and is only ever read
/// while building a view.
@MainActor
extension AppError {
    public var displayMessage: String {
        switch kind {
        case .offline:
            String(localized: "You appear to be offline.", bundle: .module)
        case .unauthenticated:
            String(localized: "Your session has ended. Please sign in again.", bundle: .module)
        case .throttled:
            String(localized: "Too many requests. Please wait a moment.", bundle: .module)
        case .server, .decoding, .unknown:
            String(localized: "Something went wrong. Please try again.", bundle: .module)
        }
    }

    /// Whether the UI should offer a retry affordance.
    public var isRecoverable: Bool { isRetryable }
}
