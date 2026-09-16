import Core
import Foundation
import Models
import Networking

public extension User {
    /// A user for tests to hand around. `onboarded` is the interesting axis: the same sign-in
    /// path serves a fresh account and a returning one, and the gate sends them to different
    /// places.
    static func fake(id: Int = 1, email: String = "someone@example.com", onboarded: Bool = true) -> User {
        User(
            id: id,
            email: email,
            authProvider: .apple,
            timezone: "Europe/Kyiv",
            isOnboardingComplete: onboarded
        )
    }
}

/// A scripted `AuthRepository`.
///
/// An `actor` rather than a struct with `var`s so that recording calls is safe under Swift
/// Testing's in-process parallelism — the mistake `docs/TESTING.md` describes.
public actor StubAuthRepository: AuthRepository {
    public enum Outcome: Sendable {
        case user(User)
        case none
        case failure(AppError)
        case cancelled
    }

    private let restoreOutcome: Outcome
    private let signInOutcome: Outcome

    public private(set) var signOutCount = 0
    public private(set) var appleRequests: [AppleIdentityRequest] = []
    public private(set) var googleSignInCount = 0

    public init(restore: Outcome = .none, signIn: Outcome = .user(.fake())) {
        restoreOutcome = restore
        signInOutcome = signIn
    }

    public func restore() async throws -> User? {
        switch restoreOutcome {
        case .user(let user): return user
        case .none: return nil
        case .failure(let error): throw error
        case .cancelled: throw CancellationError()
        }
    }

    public func signInWithApple(_ request: AppleIdentityRequest) async throws -> User {
        appleRequests.append(request)
        return try resolveSignIn()
    }

    public func signInWithGoogle() async throws -> User {
        googleSignInCount += 1
        return try resolveSignIn()
    }

    public func signOut() async {
        signOutCount += 1
    }

    private func resolveSignIn() throws -> User {
        switch signInOutcome {
        case .user(let user): return user
        case .none: throw AppError(kind: .unknown, diagnostic: "Stub had no user to return")
        case .failure(let error): throw error
        case .cancelled: throw CancellationError()
        }
    }
}

/// Stands in for the Firebase exchange. No credentials, no network, no SDK — which is the
/// entire point of `IdentityAuthenticating` existing (ADR 0005).
public struct StubIdentityProvider: IdentityAuthenticating {
    private let credential: IdentityCredential?
    private let error: (any Error)?

    public init(
        credential: IdentityCredential? = IdentityCredential(
            firebaseIDToken: "firebase-id-token",
            email: "someone@example.com"
        ),
        error: (any Error)? = nil
    ) {
        self.credential = credential
        self.error = error
    }

    public func exchangeAppleCredential(_ request: AppleIdentityRequest) async throws -> IdentityCredential {
        try resolve()
    }

    public func signInWithGoogle() async throws -> IdentityCredential {
        try resolve()
    }

    private func resolve() throws -> IdentityCredential {
        if let error { throw error }
        guard let credential else { throw AppError(kind: .unknown) }
        return credential
    }
}

public struct FixedDeviceIdentity: DeviceIdentifying {
    private let value: String
    public let model: String

    public init(guid: String = "11111111-2222-3333-4444-555555555555", model: String = "iPhone17,1") {
        value = guid
        self.model = model
    }

    public func guid() async -> String { value }
}

/// A refresh transport the test controls precisely: it reports when a call has entered, and
/// blocks there until released.
///
/// Timing-based tests of `TokenRefresher` are worse than useless — they pass against broken
/// implementations because the interleaving they need never happens. This makes the dangerous
/// interleaving deterministic: hold a refresh open, do something to the actor, then let it land.
///
/// Deliberately **not** cancellation-aware. A real refresh whose response already arrived is not
/// stopped by `Task.cancel()` either, and that is the window these tests exist to cover.
public actor GatedRefreshTransport: TokenRefreshing {
    /// Every refresh token the transport was called with, in order.
    public private(set) var tokensSeen: [String] = []

    private var blocked: [CheckedContinuation<Void, Never>] = []
    private var awaitingEntry: [(count: Int, continuation: CheckedContinuation<Void, Never>)] = []
    private var isOpen = false
    private let failingCalls: Set<Int>

    /// - Parameter failingCalls: 1-based call numbers that should throw a **transient** error
    ///   rather than rotate. A transient failure leaves the generation and the cached pair
    ///   untouched, which is the state the nastiest interleavings need.
    public init(failingCalls: Set<Int> = []) {
        self.failingCalls = failingCalls
    }

    public func refresh(using refreshToken: String) async throws -> TokenPair {
        tokensSeen.append(refreshToken)

        let reached = awaitingEntry.filter { $0.count <= tokensSeen.count }
        awaitingEntry.removeAll { $0.count <= tokensSeen.count }
        for waiter in reached { waiter.continuation.resume() }

        let call = tokensSeen.count

        if !isOpen {
            await withCheckedContinuation { blocked.append($0) }
        }

        if failingCalls.contains(call) {
            throw AppError(kind: .server(status: 503))
        }

        return TokenPair(access: "rotated-access-\(call)", refresh: "rotated-refresh-\(call)")
    }

    /// Suspends until at least `count` refreshes have started.
    public func waitForEntry(count: Int = 1) async {
        if tokensSeen.count >= count { return }
        await withCheckedContinuation { awaitingEntry.append((count, $0)) }
    }

    /// Lets every blocked refresh, and every later one, return.
    public func release() {
        isOpen = true
        let waiting = blocked
        blocked = []
        for continuation in waiting { continuation.resume() }
    }

    /// Releases only the first `count` blocked refreshes, leaving the rest parked and the gate
    /// shut. This is what makes "one refresh finishes while another is still open" expressible.
    public func release(count: Int) {
        let releasing = blocked.prefix(count)
        blocked.removeFirst(min(count, blocked.count))
        for continuation in releasing { continuation.resume() }
    }
}
