import Core
import Foundation
import Models
import Synchronization
import TestSupport
import Testing

@testable import Networking

@Suite("API onboarding repository")
struct APIOnboardingRepositoryTests {

    /// One request as it arrived, with its body already read: `URLProtocol` moves a body to a stream.
    private struct Seen: Sendable {
        let method: String
        let path: String
        let body: Data?
    }

    /// A reference to the recorded requests, because `Mutex` is non-copyable and cannot be
    /// captured by the handler closure on its own.
    private final class Recorder: Sendable {
        private let storage = Mutex<[Seen]>([])

        func record(_ seen: Seen) { storage.withLock { $0.append(seen) } }
        var all: [Seen] { storage.withLock { $0 } }
    }

    /// Answers each route from a table, and records every request in order, so a test can assert
    /// what was *not* sent as well as what was. A route missing from the table is a 500.
    private final class Server: Sendable {
        typealias Route = (status: Int, body: Data)

        private let recorder = Recorder()
        let session: URLSession

        init(_ routes: [String: Route]) {
            let recorder = self.recorder
            session = StubURLProtocol.session { request in
                let seen = Seen(
                    method: request.httpMethod ?? "?",
                    path: request.url?.path ?? "",
                    body: request.httpBody ?? request.httpBodyStream.map(Server.drain)
                )
                recorder.record(seen)
                let route = routes["\(seen.method) \(seen.path)"] ?? (500, Data())
                return (StubURLProtocol.response(route.status), route.body)
            }
        }

        var writes: [String] {
            recorder.all.filter { $0.method == "POST" }.map(\.path)
        }

        /// The body of the POST to `path`. `GET /goals` and `POST /goals` share a path.
        func body(ofPostTo path: String) throws -> [String: Any] {
            let seen = try #require(recorder.all.first { $0.method == "POST" && $0.path == path })
            let data = try #require(seen.body)
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

    /// Every route a brand-new account meets: nothing exists yet, and every write succeeds.
    private static let newAccount: [String: Server.Route] = [
        "GET /baseline/latest": (404, Data()),
        "GET /goals": (200, ConstructedResponse.goals([])),
        "GET /users": (200, ConstructedResponse.currentUser(isOnboardingComplete: false)),
        "POST /baseline": (201, ConstructedResponse.baseline(statusCode: 201)),
        "POST /goals": (201, ConstructedResponse.goals(["fix_sleep"], statusCode: 201)),
        "POST /users/complete-onboarding": (201, ConstructedResponse.onboardingCompleted),
    ]

    private let answers = SurveyAnswers(goal: .fixSleep, feeling: 7, hurdle: .allOrNothing)

    private func repository(_ server: Server) -> APIOnboardingRepository {
        let refresher = TokenRefresher(
            transport: CountingRefreshTransport(),
            storage: InMemoryTokenStorage(initial: TokenPair(access: "access", refresh: "refresh"))
        )
        return APIOnboardingRepository(
            client: APIClient(baseURL: StubURLProtocol.url, session: server.session, refresher: refresher)
        )
    }

    @Test("a new account posts baseline, goals, then completion, in that order")
    func newAccountPostsAllThree() async throws {
        let server = Server(Self.newAccount)

        try await repository(server).submit(answers)

        #expect(server.writes == ["/baseline", "/goals", "/users/complete-onboarding"])
    }

    /// The server validates `motivationScore` as 1–10 and stores the hurdle as the English label;
    /// the goal goes as its id, which the server's label map is keyed on.
    @Test("the answers go on the wire as the server stores them")
    func wireValues() async throws {
        let server = Server(Self.newAccount)

        try await repository(server).submit(answers)

        let baseline = try server.body(ofPostTo: "/baseline")
        #expect(Set(baseline.keys) == ["motivationScore", "anticipatedHurdle"])
        #expect(baseline["motivationScore"] as? Int == 7)
        #expect(baseline["anticipatedHurdle"] as? String == "All-or-nothing mindset")
        #expect(try server.body(ofPostTo: "/goals")["titles"] as? [String] == ["fix_sleep"])
    }

    /// A second baseline is a 409 — `userId` is unique — so the one already there must be skipped.
    @Test("a baseline that already exists is not posted again")
    func existingBaselineIsSkipped() async throws {
        var routes = Self.newAccount
        routes["GET /baseline/latest"] = (200, ConstructedResponse.baseline())
        let server = Server(routes)

        try await repository(server).submit(answers)

        #expect(server.writes == ["/goals", "/users/complete-onboarding"])
    }

    @Test("goals that already exist are not posted again")
    func existingGoalsAreSkipped() async throws {
        var routes = Self.newAccount
        routes["GET /goals"] = (200, ConstructedResponse.goals(["stop_overthinking"]))
        let server = Server(routes)

        try await repository(server).submit(answers)

        #expect(server.writes == ["/baseline", "/users/complete-onboarding"])
    }

    /// Completion sets the flag, then queues a job; a failure between them is a 500 with the flag
    /// already set, and completing again is a 400. A retry must read the flag, not assume it.
    @Test(
        "a completion that already landed is not sent again",
        .bug("complete-onboarding is not atomic, and a second call is a 400")
    )
    func landedCompletionIsSkipped() async throws {
        var routes = Self.newAccount
        routes["GET /baseline/latest"] = (200, ConstructedResponse.baseline())
        routes["GET /goals"] = (200, ConstructedResponse.goals(["fix_sleep"]))
        routes["GET /users"] = (200, ConstructedResponse.currentUser(isOnboardingComplete: true))
        let server = Server(routes)

        try await repository(server).submit(answers)

        #expect(server.writes.isEmpty)
    }

    @Test("a failure reading the baseline posts nothing and throws")
    func failedReadPostsNothing() async throws {
        var routes = Self.newAccount
        routes["GET /baseline/latest"] = (503, Data())
        let server = Server(routes)

        await #expect(throws: AppError(kind: .server(status: 503))) {
            try await repository(server).submit(answers)
        }
        #expect(server.writes.isEmpty)
    }

    @Test("a failed goals post stops before completion, so the account is never completed without goals")
    func failedGoalsStopsCompletion() async throws {
        var routes = Self.newAccount
        routes["POST /goals"] = (500, Data())
        let server = Server(routes)

        await #expect(throws: AppError(kind: .server(status: 500))) {
            try await repository(server).submit(answers)
        }
        #expect(server.writes == ["/baseline", "/goals"])
    }
}
