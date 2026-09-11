import Core
import Foundation
import TestSupport
import Testing

@testable import Networking

@Suite("Token refresh")
struct TokenRefresherTests {

    private struct Harness {
        let refresher: TokenRefresher
        let transport: CountingRefreshTransport
        let storage: InMemoryTokenStorage
    }

    private func makeHarness(
        behaviour: CountingRefreshTransport.Behaviour = .succeed
    ) -> Harness {
        let transport = CountingRefreshTransport(behaviour: behaviour)
        let storage = InMemoryTokenStorage(
            initial: TokenPair(access: "stale", refresh: "stale-refresh")
        )
        return Harness(
            refresher: TokenRefresher(transport: transport, storage: storage),
            transport: transport,
            storage: storage
        )
    }

    @Test("many simultaneous 401s cause exactly one refresh")
    func singleFlight() async throws {
        let harness = makeHarness()
        let refresher = harness.refresher
        let credentials = try await refresher.credentials()
        let generation = try #require(credentials?.generation)

        await withTaskGroup(of: Void.self) { group in
            for _ in 0..<25 {
                group.addTask { _ = try? await refresher.refreshed(after: generation) }
            }
        }

        #expect(await harness.transport.callCount == 1)
    }

    @Test("a request that 401s after someone else refreshed does not burn another token")
    func staleGenerationShortCircuits() async throws {
        let harness = makeHarness()
        let refresher = harness.refresher
        let generation = try #require(try await refresher.credentials()?.generation)

        // First request refreshes.
        let fresh = try await refresher.refreshed(after: generation)

        // A request already in flight comes back 401 carrying the *old* generation.
        // Its token is stale by definition — it must receive the new pair, not refresh again.
        let second = try await refresher.refreshed(after: generation)

        #expect(await harness.transport.callCount == 1)
        #expect(second.tokens == fresh.tokens)
    }

    @Test("signing out during a refresh does not re-persist tokens")
    func signOutDuringRefreshDiscardsResult() async throws {
        // Regression: signOut() cleared storage but left the in-flight refresh running.
        // It completed afterwards and wrote live credentials back to the Keychain for a
        // user who had just signed out.
        let harness = makeHarness()
        let generation = try #require(try await harness.refresher.credentials()?.generation)

        async let refresh: Void = {
            _ = try? await harness.refresher.refreshed(after: generation)
        }()

        await harness.refresher.signOut()
        await refresh

        #expect(try await harness.storage.load() == nil)
        #expect(try await harness.refresher.credentials() == nil)
    }

    @Test("a transient failure keeps the session alive")
    func transientFailureKeepsSession() async throws {
        let harness = makeHarness(
            behaviour: .fail(AppError(kind: .server(status: 503)))
        )
        let (refresher, storage) = (harness.refresher, harness.storage)
        let generation = try #require(try await refresher.credentials()?.generation)

        await #expect(throws: AppError.self) {
            _ = try await refresher.refreshed(after: generation)
        }

        // The incident: treating a backend blip as a dead session signed users out.
        #expect(await storage.clearCount == 0)
        #expect(try await refresher.credentials() != nil)
    }

    @Test("the server rejecting the token ends the session", arguments: [400, 401, 403])
    func rejectedTokenEndsSession(status: Int) async throws {
        let harness = makeHarness(
            behaviour: .fail(AppError(kind: .server(status: status)))
        )
        let (refresher, storage) = (harness.refresher, harness.storage)
        let generation = try #require(try await refresher.credentials()?.generation)

        await #expect(throws: AppError.self) {
            _ = try await refresher.refreshed(after: generation)
        }

        #expect(await storage.clearCount == 1)
        #expect(try await refresher.credentials() == nil)
    }
}
