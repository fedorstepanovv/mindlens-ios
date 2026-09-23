import Core
import Foundation
import Synchronization
import TestSupport
import Testing

@testable import Networking

@Suite("Live token refresh")
struct AuthTokenRefreshTransportTests {

    private static let validPair = ConstructedResponse.tokenRefresh

    @Test("a successful rotation returns the new pair")
    func rotates() async throws {
        let transport = AuthTokenRefreshTransport(
            baseURL: StubURLProtocol.url,
            session: StubURLProtocol.session(status: 200, body: Self.validPair)
        )

        let pair = try await transport.refresh(using: "old-refresh")

        #expect(pair == TokenPair(access: "new-access", refresh: "new-refresh"))
    }

    @Test("the refresh token travels as the bearer, since there is no access token to use")
    func sendsRefreshTokenAsBearer() async throws {
        let seen = Mutex<String?>(nil)
        let session = StubURLProtocol.session { request in
            seen.withLock { $0 = request.value(forHTTPHeaderField: "Authorization") }
            return (StubURLProtocol.response(200), Self.validPair)
        }

        _ = try await AuthTokenRefreshTransport(baseURL: StubURLProtocol.url, session: session)
            .refresh(using: "the-refresh-token")

        #expect(seen.withLock { $0 } == "Bearer the-refresh-token")
    }

    /// The half of ADR 0004 that caused the production incident. **400/401/403 mean the
    /// session is over; everything else is transient** — and a refresh 400 is a rejected
    /// token, not a malformed request, so it must not arrive as something retryable.
    @Test(
        "failures classify so TokenRefresher can tell a dead session from a blip",
        arguments: [
            (status: 400, expected: AppError.Kind.server(status: 400)),
            (status: 401, expected: .unauthenticated),
            (status: 403, expected: .server(status: 403)),
            (status: 429, expected: .throttled(retryAfter: nil)),
            (status: 500, expected: .server(status: 500)),
            (status: 503, expected: .server(status: 503)),
        ]
    )
    func classifiesFailures(status: Int, expected: AppError.Kind) async {
        let transport = AuthTokenRefreshTransport(
            baseURL: StubURLProtocol.url,
            session: StubURLProtocol.session(status: status)
        )

        let error = await #expect(throws: AppError.self) {
            try await transport.refresh(using: "old-refresh")
        }

        #expect(error?.kind == expected)
    }

    /// A rotation that succeeded but did not decode is **not** a dead session. Reporting it as
    /// one would sign every user out over a renamed field.
    @Test("a body that does not match the contract is a decoding error, not a sign-out")
    func malformedSuccessIsNotUnauthenticated() async {
        let transport = AuthTokenRefreshTransport(
            baseURL: StubURLProtocol.url,
            session: StubURLProtocol.session(status: 200, body: Data(#"{"data":{"token":"x"}}"#.utf8))
        )

        let error = await #expect(throws: AppError.self) {
            try await transport.refresh(using: "old-refresh")
        }

        #expect(error?.kind == .decoding)
    }
}

/// The transport and the actor together, because each is only half of the behaviour ADR 0004
/// describes: the classification is worthless if the wiring drops it.
@Suite("Refresh classification end to end")
struct RefreshClassificationTests {

    @Test("a rejected refresh token ends the session exactly once", .timeLimit(.minutes(1)))
    func rejectedTokenEndsSession() async throws {
        let storage = InMemoryTokenStorage(initial: TokenPair(access: "a", refresh: "r"))
        let refresher = TokenRefresher(
            transport: AuthTokenRefreshTransport(
                baseURL: StubURLProtocol.url,
                session: StubURLProtocol.session(status: 400)
            ),
            storage: storage
        )

        await #expect(throws: AppError(kind: .unauthenticated)) {
            try await refresher.refreshed(after: 0)
        }
        // Bound rather than inlined: both of these throw, and `#expect` cannot add the `try`.
        let stored = try await storage.load()
        let credentials = try await refresher.credentials()
        #expect(stored == nil)
        #expect(credentials == nil)
    }

    @Test("a backend blip keeps the session and the tokens", .timeLimit(.minutes(1)))
    func transientFailureKeepsSession() async throws {
        let storage = InMemoryTokenStorage(initial: TokenPair(access: "a", refresh: "r"))
        let refresher = TokenRefresher(
            transport: AuthTokenRefreshTransport(
                baseURL: StubURLProtocol.url,
                session: StubURLProtocol.session(status: 503)
            ),
            storage: storage
        )

        await #expect(throws: AppError(kind: .server(status: 503))) {
            try await refresher.refreshed(after: 0)
        }
        let stored = try await storage.load()
        let credentials = try await refresher.credentials()
        #expect(stored != nil)
        #expect(credentials != nil)
    }
}
