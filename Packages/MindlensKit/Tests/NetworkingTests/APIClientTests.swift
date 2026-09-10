import Core
import Foundation
import Models
import TestSupport
import Testing

@testable import Networking

@Suite("API client")
struct APIClientTests {

    private let baseURL = StubURLProtocol.url

    private func makeClient(
        session: URLSession,
        transport: CountingRefreshTransport? = nil
    ) -> (APIClient, TokenRefresher) {
        let refresher = TokenRefresher(
            transport: transport ?? CountingRefreshTransport(),
            storage: InMemoryTokenStorage(initial: TokenPair(access: "a", refresh: "r"))
        )
        return (APIClient(baseURL: baseURL, session: session, refresher: refresher), refresher)
    }

    @Test("unwraps the data envelope so call sites never see it")
    func unwrapsEnvelope() async throws {
        let body = try Fixture.data("mood_create_200")
        let (client, _) = makeClient(session: StubURLProtocol.session(status: 200, body: body))

        let mood: MoodRecordDTO = try await client.send(.get("/mood"))

        #expect(mood.mood == 3)
        #expect(mood.rating == .steady)
    }

    @Test("a 401 refreshes once and retries the request")
    func retriesOnceAfterRefresh() async throws {
        let requests = Counter()
        let body = try Fixture.data("mood_create_200")
        let transport = CountingRefreshTransport()

        let session = StubURLProtocol.session { request in
            requests.increment()
            // First attempt is unauthorised; the retry carries the refreshed token.
            let isRetry = request.value(forHTTPHeaderField: "Authorization") != "Bearer a"
            return (StubURLProtocol.response(isRetry ? 200 : 401), isRetry ? body : Data())
        }

        let (client, _) = makeClient(session: session, transport: transport)
        let mood: MoodRecordDTO = try await client.send(.get("/mood"))

        #expect(mood.mood == 3)
        #expect(requests.value == 2)
        #expect(await transport.callCount == 1)
    }

    @Test("does not retry more than once")
    func doesNotRetryForever() async throws {
        let requests = Counter()
        let session = StubURLProtocol.session(status: 401, counter: requests)
        let (client, _) = makeClient(session: session)

        await #expect(throws: AppError.self) {
            let _: MoodRecordDTO = try await client.send(.get("/mood"))
        }

        // One original + one retry. A loop here would hammer the API and burn tokens.
        #expect(requests.value == 2)
    }

    @Test(
        "maps status codes to error kinds",
        arguments: [
            (429, AppError.Kind.throttled(retryAfter: nil)),
            (500, AppError.Kind.server(status: 500)),
            (404, AppError.Kind.server(status: 404)),
        ])
    func mapsErrorKinds(status: Int, expected: AppError.Kind) async throws {
        let (client, _) = makeClient(session: StubURLProtocol.session(status: status))

        let error = await #expect(throws: AppError.self) {
            let _: MoodRecordDTO = try await client.send(.get("/mood", requiresAuth: false))
        }

        #expect(error?.kind == expected)
    }

    @Test("returns NoContent for a 204 rather than failing to decode")
    func handlesNoContent() async throws {
        let (client, _) = makeClient(session: StubURLProtocol.session(status: 204))
        let result: NoContent = try await client.send(.delete("/notes/1"))
        #expect(type(of: result) == NoContent.self)
    }

    @Test("a body that does not match the contract surfaces as a decoding error")
    func surfacesDecodingFailure() async throws {
        let body = Data(#"{"data":{"nope":1},"success":true}"#.utf8)
        let session = StubURLProtocol.session(status: 200, body: body)
        let (client, _) = makeClient(session: session)

        let error = await #expect(throws: AppError.self) {
            let _: MoodRecordDTO = try await client.send(.get("/mood"))
        }

        #expect(error?.kind == .decoding)
    }
}
