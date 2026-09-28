#if DEBUG
import Core
import Foundation

public extension SurveyAnswers {
    /// Answers for previews, with the slider where it starts.
    static let preview = SurveyAnswers(goal: .fixSleep, feeling: 5, hurdle: .overthinking)
}

/// An `OnboardingRepository` for previews. `DEBUG`-only, beside the protocol, for the reasons
/// `PreviewAuthRepository` gives.
public struct PreviewOnboardingRepository: OnboardingRepository {
    public var error: AppError?

    public init(error: AppError? = nil) {
        self.error = error
    }

    public func submit(_ answers: SurveyAnswers) async throws {
        if let error { throw error }
    }
}
#endif
