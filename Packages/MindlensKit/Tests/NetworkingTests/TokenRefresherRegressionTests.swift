import Core
import Foundation
import TestSupport
import Testing

@testable import Networking

/// The two interleavings that `TokenRefresherTests` claims to cover and does not.
///
/// Both are the ADR 0004 failure shape — a single-use refresh token spent twice, or a dead pair
/// written over a live one — and both are invisible to a timing-based test, because the ordering
/// they need is the one that does not happen when you simply fire N callers at once.
@Suite("Token refresh, hostile interleavings")
struct TokenRefresherRegressionTests {

    @Test(
        "a refresh still in flight at sign-out must not overwrite the next session",
        .bug("Stale refresh adopted over a fresh session"),
        .timeLimit(.minutes(1))
    )
    func staleRefreshDoesNotClobberNewSession() async throws {
        let transport = GatedRefreshTransport()
        let storage = InMemoryTokenStorage(initial: TokenPair(access: "old-a", refresh: "old-r"))
        let refresher = TokenRefresher(transport: transport, storage: storage)

        // A refresh goes out, and stops inside the transport.
        let inFlight = Task { try await refresher.refreshed(after: 0) }
        await transport.waitForEntry()

        // The user signs out and straight back in, while that refresh is still open.
        await refresher.signOut()
        try await refresher.adopt(TokenPair(access: "new-a", refresh: "new-r"))

        // Now the old refresh lands.
        await transport.release()
        _ = try? await inFlight.value

        let stored = try await storage.load()
        let credentials = try await refresher.credentials()

        #expect(stored == TokenPair(access: "new-a", refresh: "new-r"))
        #expect(credentials?.tokens == TokenPair(access: "new-a", refresh: "new-r"))
    }

    @Test(
        "one refresh finishing must not unregister a different one",
        .bug("defer cleared whichever task was current, not its own"),
        .timeLimit(.minutes(1))
    )
    func finishingRefreshDoesNotUnregisterAnother() async throws {
        // The first refresh fails transiently, so it leaves the cached pair and the generation
        // untouched — which is what lets a later caller read the very same refresh token.
        let transport = GatedRefreshTransport(failingCalls: [1])
        let storage = InMemoryTokenStorage(initial: TokenPair(access: "a", refresh: "r"))
        let refresher = TokenRefresher(transport: transport, storage: storage)

        // First refresh opens and parks inside the transport.
        let first = Task { try await refresher.refreshed(after: 0) }
        await transport.waitForEntry(count: 1)

        // Sign-out clears `inFlight` while that task keeps running; a new session begins and a
        // second refresh registers itself and parks too.
        await refresher.signOut()
        try await refresher.adopt(TokenPair(access: "new-a", refresh: "new-r"))

        let current = try #require(try await refresher.credentials()).generation
        let second = Task { try await refresher.refreshed(after: current) }
        await transport.waitForEntry(count: 2)

        // Only the first one lands. Its `defer` runs while the second is still in flight: if it
        // nils the current registration rather than its own, the second stops being joinable.
        await transport.release(count: 1)
        _ = try? await first.value

        // A third caller arrives. It must *join* the second refresh. If the second's
        // registration was lost, it instead starts one of its own — with the very token the
        // second is still holding, and that token is single-use.
        let third = Task { try await refresher.refreshed(after: current) }

        // A negative assertion, so it needs a window rather than an event: there is no third
        // entry to wait for when the code is correct. Without the fix the extra refresh starts
        // immediately, so this is not a close-run thing.
        try await Task.sleep(for: .milliseconds(200))
        let spent = await transport.tokensSeen

        await transport.release()
        _ = try? await second.value
        _ = try? await third.value

        #expect(
            spent.count == Set(spent).count,
            "The same single-use refresh token went out twice; the server kills the second: \(spent)"
        )
        #expect(spent.count == 2, "A third refresh started instead of joining the second: \(spent)")
    }
}
