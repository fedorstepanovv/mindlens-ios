import Foundation
import Models

/// `POST /baseline`. Both fields are validated server-side: `motivationScore` an integer in 1–10,
/// `anticipatedHurdle` a non-empty string of at most 500 characters.
struct BaselineBody: Encodable, Sendable {
    let motivationScore: Int
    let anticipatedHurdle: String
}

/// `POST /goals`. The server adds these titles to the account's goals and skips any it already has.
struct GoalsBody: Encodable, Sendable {
    let titles: [String]
}

/// A baseline row. Only its identity is read: the app needs to know whether one exists.
struct BaselineDTO: Decodable, Sendable {
    let id: String
}

/// A goal row. Only its identity is read, for the same reason.
struct GoalDTO: Decodable, Sendable {
    let id: String
}

extension Endpoint where Response == BaselineDTO {
    /// `GET /baseline/latest` — 404 when the account has none.
    static var latestBaseline: Self { .get("/baseline/latest") }

    /// `POST /baseline` — 201. A second one for the same account is a 409: `userId` is unique.
    static func createBaseline(_ answers: SurveyAnswers) throws -> Self {
        try .post(
            "/baseline",
            body: BaselineBody(motivationScore: answers.feeling, anticipatedHurdle: answers.hurdle.rawValue)
        )
    }
}

extension Endpoint where Response == [GoalDTO] {
    /// `GET /goals` — an empty array when there are none, never a 404.
    static var goals: Self { .get("/goals") }

    /// `POST /goals` — 201, with only the rows it inserted.
    static func createGoals(_ goals: [SurveyGoal]) throws -> Self {
        try .post("/goals", body: GoalsBody(titles: goals.map(\.rawValue)))
    }
}

extension Endpoint where Response == NoContent {
    /// `POST /users/complete-onboarding` — 201 with no `data` key at all, since the handler
    /// returns nothing. A 400 if onboarding is already complete, or if a baseline or a goal is missing.
    ///
    /// No body, as with logout: the route declares none.
    static var completeOnboarding: Self {
        Endpoint(method: .post, path: "/users/complete-onboarding")
    }
}
