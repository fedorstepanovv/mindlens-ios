import Core
import Foundation
import Models
import Synchronization
import TestSupport
import Testing

@testable import Networking

@Suite("API auth repository")
struct APIAuthRepositoryTests {

    /// Records every request and answers by path, so a test can assert what was *not* called
    /// as well as what was.
    /// A separate object because `Mutex` is non-copyable: it cannot be copied out of `Server`
    /// to be captured by the handler closure, so the shared state lives behind a reference both
    /// of them hold.
    private final class Recorder: Sendable {
        private let storage = Mutex<[URLRequest]>([])

        func record(_ request: URLRequest) { storage.withLock { $0.append(request) } }
        var all: [URLRequest] { storage.withLock { $0 } }
    }

    private final class Server: Sendable {
        let recorder = Recorder()
        let session: URLSession

        init(status: Int = 200, body: Data = APIAuthRepositoryTests.sessionBody) {
            let recorder = self.recorder
            session = StubURLProtocol.session { request in
                recorder.record(request)
                return (StubURLProtocol.response(status), body)
            }
        }

        var paths: [String] { recorder.all.compactMap(\.url?.path) }

        func body(at index: Int) throws -> [String: Any] {
            let request = try #require(recorder.all[index])
            // URLProtocol moves a body to the stream, so read it from there.
            let data = try #require(request.httpBody ?? request.httpBodyStream.map(Self.drain))
            return try #require(try JSONSerialization.jsonObject(with: data) as? [String: Any])
        }

        private static func drain(_ stream: InputStream) -> Data {
            stream.open()
            defer { stream.close() }

            var data = Data()
            var buffer = [UInt8](repeating: 0, count: 1024)
            while stream.hasBytesAvailable {
                let read = stream.read(&buffer, maxLength: buffer.count)
                if read <= 0 { break }
                data.append(buffer, count: read)
            }
            return data
        }
    }

    static let sessionBody = ConstructedResponse.signIn(isOnboardingComplete: true)

    private func makeRepository(
        server: Server,
        identity: any IdentityAuthenticating = StubIdentityProvider(),
        storage: InMemoryTokenStorage = InMemoryTokenStorage()
    ) -> (APIAuthRepository, TokenRefresher) {
        let refresher = TokenRefresher(transport: CountingRefreshTransport(), storage: storage)
        let repository = APIAuthRepository(
            client: APIClient(baseURL: StubURLProtocol.url, session: server.session, refresher: refresher),
            refresher: refresher,
            identity: identity,
            device: FixedDeviceIdentity(),
            dates: FixedDateProvider(.distantPast, timeZone: TimeZone(identifier: "Europe/Kyiv") ?? .gmt)
        )
        return (repository, refresher)
    }

    @Test("signing in sends the device identity and the device's timezone")
    func appleSignInBody() async throws {
        let server = Server()
        let (repository, _) = makeRepository(server: server)

        _ = try await repository.signInWithApple(
            AppleIdentityRequest(
                identityToken: "apple-token", rawNonce: "nonce", authorizationCode: nil, email: nil
            )
        )

        #expect(server.paths == ["/auth/apple"])
        let body = try server.body(at: 0)
        #expect(body["guid"] as? String == "11111111-2222-3333-4444-555555555555")
        #expect(body["deviceModel"] as? String == "iPhone17,1")
        #expect(body["timezone"] as? String == "Europe/Kyiv")
        // The identity provider's email, since Apple gave us none on this authorization.
        #expect(body["email"] as? String == "someone@example.com")
    }

    /// The session has to be usable the instant sign-in returns, or the first authenticated
    /// request after it fails.
    @Test("the returned tokens are adopted before sign-in returns")
    func adoptsTokens() async throws {
        let server = Server()
        let storage = InMemoryTokenStorage()
        let (repository, refresher) = makeRepository(server: server, storage: storage)

        let user = try await repository.signInWithGoogle()

        #expect(user.id == 42)
        let credentials = try await refresher.credentials()
        #expect(credentials?.tokens == TokenPair(access: "access-jwt", refresh: "refresh-jwt"))
        let stored = try await storage.load()
        #expect(stored == TokenPair(access: "access-jwt", refresh: "refresh-jwt"))
    }

    @Test("Apple's own email wins when Apple provides one")
    func prefersProviderEmail() async throws {
        let server = Server()
        let (repository, _) = makeRepository(
            server: server,
            identity: StubIdentityProvider(
                credential: IdentityCredential(firebaseIDToken: "token", email: nil)
            )
        )

        _ = try await repository.signInWithApple(
            AppleIdentityRequest(
                identityToken: "apple-token",
                rawNonce: "nonce",
                authorizationCode: nil,
                email: "relay@privaterelay.appleid.com"
            )
        )

        #expect(try server.body(at: 0)["email"] as? String == "relay@privaterelay.appleid.com")
    }

    @Test("a cancelled provider flow never reaches the network")
    func cancelledIdentityFlow() async {
        let server = Server()
        let (repository, _) = makeRepository(
            server: server,
            identity: StubIdentityProvider(error: CancellationError())
        )

        await #expect(throws: CancellationError.self) {
            try await repository.signInWithGoogle()
        }
        #expect(server.paths.isEmpty)
    }

    @Test("restoring with nothing stored asks the server nothing")
    func restoreWithoutTokens() async throws {
        let server = Server()
        let (repository, _) = makeRepository(server: server)

        let user = try await repository.restore()

        #expect(user == nil)
        #expect(server.paths.isEmpty)
    }

    @Test("restoring with stored tokens asks who the user is")
    func restoreWithTokens() async throws {
        let userBody = Data(
            """
            {"data":{"id":7,"email":"someone@example.com","authProvider":"GOOGLE",
            "timezone":"Europe/Kyiv","isOnboardingComplete":true},
            "statusCode":200,"success":true,"timestamp":"2026-09-10T18:22:41.512Z"}
            """
            .utf8
        )
        let server = Server(body: userBody)
        let (repository, _) = makeRepository(
            server: server,
            storage: InMemoryTokenStorage(initial: TokenPair(access: "a", refresh: "r"))
        )

        let user = try await repository.restore()

        #expect(user?.id == 7)
        #expect(server.paths == ["/users"])
    }

    /// Sign-out tells the server first, because that call needs the access token — and then
    /// clears locally regardless of how it went.
    @Test("signing out ends the remote session, then the local one")
    func signOut() async throws {
        let server = Server(status: 204, body: Data())
        let storage = InMemoryTokenStorage(initial: TokenPair(access: "a", refresh: "r"))
        let (repository, refresher) = makeRepository(server: server, storage: storage)

        await repository.signOut()

        #expect(server.paths == ["/auth/logout"])
        let stored = try await storage.load()
        let credentials = try await refresher.credentials()
        #expect(stored == nil)
        #expect(credentials == nil)
    }

    @Test("a failed logout call still signs the user out locally")
    func signOutSurvivesServerFailure() async throws {
        let server = Server(status: 500, body: Data())
        let storage = InMemoryTokenStorage(initial: TokenPair(access: "a", refresh: "r"))
        let (repository, _) = makeRepository(server: server, storage: storage)

        await repository.signOut()

        let stored = try await storage.load()
        #expect(stored == nil)
    }
}
