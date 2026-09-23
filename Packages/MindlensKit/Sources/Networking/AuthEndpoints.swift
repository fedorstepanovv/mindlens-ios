import Core
import Foundation
import Models

/// `POST /auth/apple`. See `docs/API.md` § Authentication.
///
/// A typed struct rather than a dictionary because the server runs `whitelist` +
/// `forbidNonWhitelisted`: an extra or misspelled key is a 400, not a silently dropped
/// field. The compiler is a better place to catch that than production is.
struct AppleSignInBody: Encodable, Sendable {
    let idToken: String
    let guid: String
    let deviceModel: String
    let timezone: String
    let email: String?

    /// Spelled out because writing `encode(to:)` below suppresses the synthesized version.
    enum CodingKeys: String, CodingKey {
        case idToken, guid, deviceModel, timezone, email
    }

    /// `email` is **omitted** when absent rather than sent as `null`. The server declares
    /// it optional, so absence is the documented way to say "Apple gave us nothing".
    func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(idToken, forKey: .idToken)
        try container.encode(guid, forKey: .guid)
        try container.encode(deviceModel, forKey: .deviceModel)
        try container.encode(timezone, forKey: .timezone)
        try container.encodeIfPresent(email, forKey: .email)
    }
}

/// `POST /auth/google` — the same fields **without** `email`.
///
/// Not a shared body type with an unused field: `forbidNonWhitelisted` means sending
/// `email` to this route is a 400. Two routes with two shapes get two types.
struct GoogleSignInBody: Encodable, Sendable {
    let idToken: String
    let guid: String
    let deviceModel: String
    let timezone: String
}

/// The `data` of a successful sign-in.
struct AuthSessionDTO: Decodable, Sendable {
    let user: User
    let tokens: TokenPairDTO
}

/// `{accessToken, refreshToken}` — the wire spelling of `TokenPair`.
struct TokenPairDTO: Decodable, Sendable {
    let accessToken: String
    let refreshToken: String

    var pair: TokenPair { TokenPair(access: accessToken, refresh: refreshToken) }
}

extension Endpoint where Response == AuthSessionDTO {
    static func appleSignIn(
        identityToken: String,
        guid: String,
        deviceModel: String,
        timezone: String,
        email: String?
    ) throws -> Self {
        try .post(
            "/auth/apple",
            body: AppleSignInBody(
                idToken: identityToken,
                guid: guid,
                deviceModel: deviceModel,
                timezone: timezone,
                email: email
            ),
            requiresAuth: false
        )
    }

    static func googleSignIn(
        identityToken: String,
        guid: String,
        deviceModel: String,
        timezone: String
    ) throws -> Self {
        try .post(
            "/auth/google",
            body: GoogleSignInBody(
                idToken: identityToken,
                guid: guid,
                deviceModel: deviceModel,
                timezone: timezone
            ),
            requiresAuth: false
        )
    }
}

extension Endpoint where Response == User {
    /// `GET /users` — the current user row. Also how a stored session is checked at launch.
    static var currentUser: Self { .get("/users") }
}

extension Endpoint where Response == NoContent {
    /// `POST /auth/logout` — ends **this device's** session only, and answers 204.
    ///
    /// Built directly rather than through `.post(_:body:)` because it takes no body, and an
    /// empty JSON object is not the same thing as no body to a route with no `@Body`.
    static var logout: Self {
        Endpoint(method: .post, path: "/auth/logout", retriesAfterRefresh: false)
    }
}
