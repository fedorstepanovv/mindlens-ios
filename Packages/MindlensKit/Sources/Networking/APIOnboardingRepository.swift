import Core
import Foundation
import Models

/// The live `OnboardingRepository`: baseline, goals, then completion.
///
/// **Every write is checked for first.** The server makes none of the three safe to repeat: a
/// second baseline is a 409, and a second completion is a 400. Completion is also not atomic.
/// It sets the flag and then queues the job that generates the account's first survey, so a
/// failure between the two answers 500 with the flag already set. Reading state before each
/// write is what lets a retry after any partial failure post only what is missing.
public struct APIOnboardingRepository: OnboardingRepository {
    private let client: APIClient

    public init(client: APIClient) {
        self.client = client
    }

    public func submit(_ answers: SurveyAnswers) async throws {
        async let baseline = hasBaseline()
        async let goals = client.send(.goals)
        let (baselineExists, existingGoals) = try await (baseline, goals)

        if !baselineExists {
            _ = try await client.send(.createBaseline(answers))
        }
        if existingGoals.isEmpty {
            _ = try await client.send(.createGoals([answers.goal]))
        }
        if try await !client.send(.currentUser).isOnboardingComplete {
            _ = try await client.send(.completeOnboarding)
        }
    }

    /// A 404 is the answer "none yet", not a failure. Anything else still fails.
    private func hasBaseline() async throws -> Bool {
        do {
            _ = try await client.send(.latestBaseline)
            return true
        } catch let error as AppError where error.kind == .server(status: 404) {
            return false
        }
    }
}
