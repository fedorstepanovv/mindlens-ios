import Foundation

/// The two response bodies no one can capture — **constructed, not captured** (ADR 0024).
///
/// `POST /auth/apple` needs a live Firebase ID token and `POST /auth/refresh` consumes the
/// session's single-use refresh token, so neither can be fetched beside a running app. Each body
/// carries exactly the keys two independent readers of production agree on: the server's Prisma
/// `User` row and `Tokens`, and the source app's `UserModel` and `TokensModel`, which decode these
/// responses in production today. A key either one requires is here; nothing here is a guess.
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
}
