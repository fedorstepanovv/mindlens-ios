import Foundation

/// Response bodies **constructed, not captured** (ADR 0024, 0027).
///
/// Each carries exactly the keys two independent readers of production agree on: the server's
/// Prisma model, and the source app's model that decodes the response in production today. A key
/// either one requires is here; nothing here is a guess. Each body's comment names both.
///
/// `POST /auth/apple` and `POST /auth/refresh` could not be captured at all (ADR 0024). The rest
/// could, but only with a production account's token, and some only through a write that happens
/// once per account (ADR 0027).
///
/// Swift rather than a file under `Fixtures/`, because a file there claims to be a capture.
public enum ConstructedResponse {

    /// A `POST /auth/apple` or `/auth/google` 200: `data` is `{user, tokens}`, and `user` is the
    /// whole Prisma row. The pair is `access-jwt` / `refresh-jwt`.
    public static func signIn(isOnboardingComplete: Bool = false) -> Data {
        Data(
            """
            {"data":{"user":{"id":42,"email":"someone@example.com","authProvider":"APPLE",
            "googleSocialId":null,"appleSocialId":"000123.abc.0100","timezone":"Europe/Kyiv",
            "isOnboardingComplete":\(isOnboardingComplete),"createdAt":"2026-09-10T18:22:41.512Z",
            "updatedAt":"2026-09-10T18:22:41.512Z"},
            "tokens":{"accessToken":"access-jwt","refreshToken":"refresh-jwt"}},
            "statusCode":200,"success":true,"timestamp":"2026-09-10T18:22:41.512Z"}
            """
            .utf8
        )
    }

    /// A `POST /auth/refresh` 200: `data` is the bare pair, `new-access` / `new-refresh`.
    public static let tokenRefresh = Data(
        """
        {"data":{"accessToken":"new-access","refreshToken":"new-refresh"},
        "statusCode":200,"success":true,"timestamp":"2026-09-10T18:22:41.512Z"}
        """
        .utf8
    )

    /// A `GET /users` 200: `data` is the whole Prisma `User` row. Checked against `User` in
    /// `prisma/schema.prisma` and the source app's `UserModel`
    /// (`features/users/models/user/user_model.dart`).
    public static func currentUser(isOnboardingComplete: Bool) -> Data {
        Data(
            """
            {"data":{"id":42,"email":"someone@example.com","authProvider":"APPLE",
            "googleSocialId":null,"appleSocialId":"000123.abc.0100","timezone":"Europe/Kyiv",
            "isOnboardingComplete":\(isOnboardingComplete),"createdAt":"2026-09-10T18:22:41.512Z",
            "updatedAt":"2026-09-10T18:22:41.512Z"},
            "statusCode":200,"success":true,"timestamp":"2026-09-10T18:22:41.512Z"}
            """
            .utf8
        )
    }

    /// A `GET /baseline/latest` 200, or a `POST /baseline` 201 with `statusCode` 201: the Prisma
    /// `UserBaselineRecord` row, unserialized. Checked against `UserBaselineRecord` in
    /// `prisma/schema.prisma` and the source app's `BaselineModel`
    /// (`features/baseline/models/baseline_model.dart`), whose `anticipatedHurdle` is the one nullable key.
    public static func baseline(statusCode: Int = 200) -> Data {
        Data(
            """
            {"data":{"id":"6f1c2a9e-0b7d-4c1e-9a53-2f8e5d7b4c10","userId":42,"motivationScore":5,
            "anticipatedHurdle":"Overthinking","isInitial":true,"createdAt":"2026-09-10T18:22:41.512Z",
            "updatedAt":"2026-09-10T18:22:41.512Z"},
            "statusCode":\(statusCode),"success":true,"timestamp":"2026-09-10T18:22:41.512Z"}
            """
            .utf8
        )
    }

    /// A `GET /goals` 200, or a `POST /goals` 201 with only the inserted rows: an array of Prisma
    /// `Goal` rows, in no order. Checked against `Goal` in `prisma/schema.prisma` and the source
    /// app's `GoalModel` (`features/goals/models/goal_model.dart`). `titles` are the rows' titles.
    public static func goals(_ titles: [String], statusCode: Int = 200) -> Data {
        let rows = titles.enumerated().map(goalRow)
        return Data(
            """
            {"data":[\(rows.joined(separator: ","))],"statusCode":\(statusCode),"success":true,
            "timestamp":"2026-09-10T18:22:41.512Z"}
            """
            .utf8
        )
    }

    private static func goalRow(_ index: Int, _ title: String) -> String {
        """
        {"id":"0d4b8e2a-7c61-4f3b-8e9d-1a2b3c4d5e6\(index)","title":"\(title)","userId":42,
        "isActive":true,"createdAt":"2026-09-10T18:22:41.512Z","updatedAt":"2026-09-10T18:22:41.512Z"}
        """
    }

    /// A `POST /users/complete-onboarding` 201. The handler returns nothing, so the response
    /// interceptor's `data` is `undefined` and the key is **absent**, not `null`. Checked against
    /// `users.controller.ts` and `response.interceptor.ts`, and the source app's `ApiClient`
    /// (`core/network/api_client.dart`), which requires only a JSON object and reads nothing from it.
    public static let onboardingCompleted = Data(
        """
        {"statusCode":201,"success":true,"timestamp":"2026-09-10T18:22:41.512Z"}
        """
        .utf8
    )
}
