import Core
import Foundation
import Models

/// A scripted `OnboardingRepository`: each call takes the next outcome, and every call is recorded.
///
/// An `actor` for the same reason `StubAuthRepository` is one.
public actor StubOnboardingRepository: OnboardingRepository {
    public private(set) var submissions: [SurveyAnswers] = []

    private var outcomes: [AppError?]

    /// - Parameter failures: what each call throws, in order; `nil` succeeds. Calls past the end
    ///   succeed.
    public init(failures: [AppError?] = []) {
        outcomes = failures
    }

    public func submit(_ answers: SurveyAnswers) async throws {
        submissions.append(answers)
        if !outcomes.isEmpty, let error = outcomes.removeFirst() {
            throw error
        }
    }
}
