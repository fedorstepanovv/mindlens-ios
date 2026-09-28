import Foundation

/// Where a new account's survey answers go once it has a session.
///
/// Beside `AuthRepository` in `Models` for the same reason: the feature that posts the answers
/// and whatever later reads them do not have to import each other.
public protocol OnboardingRepository: Sendable {
    /// Posts the answers and completes onboarding.
    ///
    /// **Safe to call again after it throws.** Whatever already reached the server is not
    /// posted twice: a second baseline is a 409 and a second completion a 400, so each is
    /// checked for before it is sent.
    func submit(_ answers: SurveyAnswers) async throws
}
