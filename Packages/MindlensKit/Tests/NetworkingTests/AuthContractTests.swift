import Core
import Foundation
import Models
import TestSupport
import Testing

@testable import Networking

@Suite("Auth request bodies")
struct AuthRequestBodyTests {

    private func body(of endpoint: Endpoint<AuthSessionDTO>) throws -> [String: Any] {
        let data = try #require(endpoint.body)
        return try #require(try JSONSerialization.jsonObject(with: data) as? [String: Any])
    }

    /// The server runs `whitelist` + `forbidNonWhitelisted`, so an unexpected key is a 400
    /// rather than a field the server ignores. These tests are the guard on that.
    @Test("the Apple body sends exactly the documented fields")
    func appleBodyFields() throws {
        let endpoint = try Endpoint.appleSignIn(
            identityToken: "firebase-token",
            guid: "11111111-2222-3333-4444-555555555555",
            deviceModel: "iPhone17,1",
            timezone: "Europe/Kyiv",
            email: "someone@example.com"
        )

        let json = try body(of: endpoint)

        #expect(Set(json.keys) == ["idToken", "guid", "deviceModel", "timezone", "email"])
        #expect(json["idToken"] as? String == "firebase-token")
        #expect(json["timezone"] as? String == "Europe/Kyiv")
    }

    /// `email` is optional server-side, and `null` is not the same as absent to a validator.
    @Test("an absent email is omitted rather than sent as null")
    func appleBodyOmitsMissingEmail() throws {
        let endpoint = try Endpoint.appleSignIn(
            identityToken: "firebase-token",
            guid: "11111111-2222-3333-4444-555555555555",
            deviceModel: "iPhone17,1",
            timezone: "Europe/Kyiv",
            email: nil
        )

        let json = try body(of: endpoint)

        #expect(json.keys.contains("email") == false)
    }

    /// `/auth/google` has no `email` field at all, so sending one is a 400. This is why the two
    /// routes have two body types instead of one with an optional field.
    @Test("the Google body carries no email key")
    func googleBodyHasNoEmail() throws {
        let endpoint = try Endpoint.googleSignIn(
            identityToken: "firebase-token",
            guid: "11111111-2222-3333-4444-555555555555",
            deviceModel: "iPhone17,1",
            timezone: "Europe/Kyiv"
        )

        let json = try body(of: endpoint)

        #expect(Set(json.keys) == ["idToken", "guid", "deviceModel", "timezone"])
    }

    @Test("sign-in routes are public, and the rest are not")
    func authRequirements() throws {
        let apple = try Endpoint.appleSignIn(
            identityToken: "t", guid: "123456", deviceModel: "iPhone1", timezone: "UTC", email: nil
        )

        #expect(apple.requiresAuth == false)
        #expect(apple.path == "/auth/apple")
        #expect(Endpoint<NoContent>.logout.requiresAuth)
        #expect(Endpoint<NoContent>.logout.path == "/auth/logout")
        // No body at all — an empty JSON object is a different thing to a route with no `@Body`.
        #expect(Endpoint<NoContent>.logout.body == nil)
        #expect(Endpoint<User>.currentUser.path == "/users")
    }
}

@Suite("Auth response decoding")
struct AuthResponseDecodingTests {

    /// ⚠️ **Constructed, not captured.** Every field and spelling here comes from the server's
    /// `SocialLoginResponseDto` (`user: User`, so the whole Prisma row) plus `Tokens`, but a
    /// real `/auth/apple` 200 needs a live Firebase ID token, which needs the SDK wired. This
    /// asserts our decoding, **not** the server's shape — it is deliberately not in
    /// `Fixtures/`, because a file in there claims to be a capture. See `docs/STATE.md`.
    private static let sessionBody = Data(
        """
        {"data":{"user":{"id":42,"email":"someone@example.com","authProvider":"APPLE",
        "googleSocialId":null,"appleSocialId":"000123.abc.0100","timezone":"Europe/Kyiv",
        "isOnboardingComplete":false,"createdAt":"2026-09-10T18:22:41.512Z",
        "updatedAt":"2026-09-10T18:22:41.512Z"},
        "tokens":{"accessToken":"access-jwt","refreshToken":"refresh-jwt"}},
        "statusCode":200,"success":true,"timestamp":"2026-09-10T18:22:41.512Z"}
        """
        .utf8
    )

    @Test("a sign-in response unwraps to a user and a token pair")
    func decodesSession() throws {
        let session = try JSONDecoder.api
            .decode(
                APIEnvelope<AuthSessionDTO>.self, from: Self.sessionBody
            )
            .data

        #expect(session.user.id == 42)
        #expect(session.user.authProvider == .apple)
        #expect(session.user.isOnboardingComplete == false)
        #expect(session.tokens.pair == TokenPair(access: "access-jwt", refresh: "refresh-jwt"))
    }

    /// `data.user` is the whole Prisma row, so columns we do not model appear there and more
    /// will arrive without a server code change. Decoding must ignore them, not fail.
    @Test("columns we do not model are ignored rather than fatal")
    func ignoresUnmodelledColumns() throws {
        let session = try JSONDecoder.api
            .decode(
                APIEnvelope<AuthSessionDTO>.self, from: Self.sessionBody
            )
            .data

        #expect(session.user.email == "someone@example.com")
    }

    /// A real capture. The 422 error body spells its status **`status`**, not `statusCode`,
    /// and carries no `error` key — a third envelope shape, and the one sign-in hits.
    @Test("a rejected Firebase token decodes, despite spelling its status differently")
    func decodesInvalidTokenError() throws {
        let envelope = try JSONDecoder.api.decode(
            APIErrorEnvelope.self, from: try Fixture.data("error_invalid_token_422")
        )

        #expect(envelope.success == false)
        let error = try #require(envelope.error)
        #expect(error.messages == ["invalid token provided"])
        #expect(error.reason == nil)
        // The body's own status field is unreadable under this shape, which is exactly why
        // classification uses the HTTP status instead.
        #expect(error.statusCode == nil)
    }

    @Test("a 422 reaches the caller as a server error carrying the server's message")
    func clientSurfacesInvalidToken() async throws {
        let session = StubURLProtocol.session(
            status: 422, body: try Fixture.data("error_invalid_token_422")
        )
        let client = APIClient(
            baseURL: StubURLProtocol.url,
            session: session,
            refresher: TokenRefresher(
                transport: CountingRefreshTransport(), storage: InMemoryTokenStorage()
            )
        )

        await #expect(throws: AppError(kind: .server(status: 422), diagnostic: "invalid token provided")) {
            try await client.send(
                try .appleSignIn(
                    identityToken: "bad", guid: "123456", deviceModel: "iPhone1",
                    timezone: "UTC", email: nil
                )
            )
        }
    }
}
