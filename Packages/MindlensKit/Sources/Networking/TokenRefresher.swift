import Core
import Foundation

/// The network call that exchanges a refresh token for a new pair.
/// Separated from `TokenRefresher` so tests can count invocations.
public protocol TokenRefreshing: Sendable {
    func refresh(using refreshToken: String) async throws -> TokenPair
}

/// Serialises token refresh.
///
/// The API issues 15-minute access tokens and **single-use, rotating** refresh tokens:
/// refreshing invalidates the token that was used. Two requests that 401 at the same
/// moment will therefore race, and the second one is holding a token the server has
/// already burned. On 2026-07-12 that force-logged-out production users.
///
/// Two mechanisms prevent it, and both matter:
///
/// 1. **Single-flight.** Concurrent callers join one in-flight refresh instead of each
///    starting their own. Actor isolation makes the race unrepresentable rather than
///    merely avoided.
/// 2. **Generation checks.** A request that was already in flight when someone else
///    refreshed will still come back 401. It must *not* refresh again — its token is
///    stale by definition. Comparing the generation it was issued under against the
///    current one tells us to hand over the fresh tokens instead of burning them.
///
/// Without (2), a single refresh still cascades into spurious sign-outs under load.
public actor TokenRefresher {

    /// Tokens plus the revision they belong to. Callers hand the generation back when
    /// reporting a 401 so we can tell "your token is old" from "the session is over".
    public struct Credentials: Sendable, Equatable {
        public let tokens: TokenPair
        public let generation: UInt64
    }

    private let transport: any TokenRefreshing
    private let storage: any TokenStorage

    private var cached: TokenPair?
    private var generation: UInt64 = 0
    private var inFlight: Task<Credentials, any Error>?
    private var inFlightStamp: UInt64 = 0
    private var sessionIsOver = false

    public init(transport: any TokenRefreshing, storage: any TokenStorage) {
        self.transport = transport
        self.storage = storage
    }

    /// Credentials to attach to an outgoing request, loading from storage on first use.
    public func credentials() async throws -> Credentials? {
        if sessionIsOver { return nil }
        if cached == nil {
            // Deliberately propagated, not `try?`. Storage answers `nil` for "no item"; it
            // *throws* for a Keychain that is locked, unreadable or holding a corrupt blob.
            // Swallowing that turns those into "no session", which signs the user out at launch
            // while perfectly good tokens sit on disk — and leaves them there, unexplained.
            cached = try await storage.load()
        }
        guard let cached else { return nil }
        return Credentials(tokens: cached, generation: generation)
    }

    /// Called when a request fails with 401.
    ///
    /// - Parameter seenGeneration: the generation the failing request was signed with.
    /// - Returns: credentials to retry with — freshly refreshed, or the current pair when
    ///   someone else already refreshed after this request went out.
    /// - Throws: `AppError` with kind `.unauthenticated` when the session is genuinely
    ///   over, or the underlying transport error when the failure was transient.
    public func refreshed(after seenGeneration: UInt64) async throws -> Credentials {
        if sessionIsOver {
            throw AppError(kind: .unauthenticated)
        }

        // Someone already refreshed after this request went out. Its 401 is stale news —
        // hand over the current tokens rather than burning them on another refresh.
        if generation > seenGeneration, let cached {
            return Credentials(tokens: cached, generation: generation)
        }

        if let inFlight {
            return try await inFlight.value
        }

        // The registration is stamped, and the `defer` only clears its own stamp. Clearing
        // `inFlight` unconditionally unregisters whoever happens to be current — which, after a
        // sign-out drops this task's registration and a new refresh takes its place, is somebody
        // else. The next caller then starts a *second* concurrent refresh on a single-use token.
        inFlightStamp &+= 1
        let stamp = inFlightStamp
        let task = Task<Credentials, any Error> { try await self.performRefresh() }
        inFlight = task
        defer { if inFlightStamp == stamp { inFlight = nil } }
        return try await task.value
    }

    /// Adopt a fresh pair after sign-in.
    public func adopt(_ tokens: TokenPair) async throws {
        cached = tokens
        generation &+= 1
        sessionIsOver = false
        try await storage.save(tokens)
    }

    public func signOut() async {
        // Cancel first. A refresh that completes after this point must not write tokens
        // back into storage for a user who has signed out.
        inFlight?.cancel()
        inFlight = nil
        cached = nil
        sessionIsOver = true
        generation &+= 1
        try? await storage.clear()
    }

    // MARK: - Private

    private func performRefresh() async throws -> Credentials {
        let existing = try await currentRefreshToken()
        let issuedUnder = generation

        let fresh: TokenPair
        do {
            fresh = try await transport.refresh(using: existing)
        } catch {
            try await handle(refreshFailure: error)
        }

        // The session may have moved while we awaited the network, and a boolean cannot see it:
        // `adopt()` sets `sessionIsOver` back to false, so after a sign-out *and a fresh sign-in*
        // this used to pass and write the dead session's rotated pair over the live one. The next
        // request then 401s, refreshes a token the server already deleted, and the user is signed
        // out — ADR 0004's incident, entered through the side door. The generation moves for both
        // sign-out and sign-in, so it sees both.
        //
        // Deliberately **outside** the `catch`: this is not a refresh failure. Routed through
        // `handle(refreshFailure:)` an `.unauthenticated` reads as "the server rejected the
        // token", and that tears down the session that just started — turning a stale result into
        // the very sign-out it exists to prevent.
        guard !sessionIsOver, generation == issuedUnder else {
            throw AppError(kind: .unauthenticated)
        }

        cached = fresh
        generation &+= 1
        try? await storage.save(fresh)
        return Credentials(tokens: fresh, generation: generation)
    }

    private func currentRefreshToken() async throws -> String {
        if cached == nil {
            cached = try? await storage.load()
        }
        guard let cached else {
            sessionIsOver = true
            throw AppError(kind: .unauthenticated)
        }
        return cached.refresh
    }

    /// Classifying the failure is the half that actually caused the incident. Treating
    /// every failed refresh as "sign the user out" turns a brief backend blip into mass
    /// sign-outs; only the server explicitly rejecting the token means the session is over.
    private func handle(refreshFailure error: some Error) async throws -> Never {
        let appError = AppError(error)

        let tokenRejected: Bool =
            switch appError.kind {
            case .server(let status): [400, 401, 403].contains(status)
            case .unauthenticated: true
            case .decoding, .offline, .throttled, .unknown: false
            }

        if tokenRejected {
            cached = nil
            sessionIsOver = true
            generation &+= 1
            try? await storage.clear()
            throw AppError(kind: .unauthenticated)
        }

        // Transient. Keep the session; the caller may retry.
        throw appError
    }
}
