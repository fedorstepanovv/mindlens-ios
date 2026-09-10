import Core
import Foundation
import Models
import Networking

public actor InMemoryTokenStorage: TokenStorage {
    private var tokens: TokenPair?
    public private(set) var clearCount = 0

    public init(initial: TokenPair? = nil) { tokens = initial }

    public func load() async throws -> TokenPair? { tokens }
    public func save(_ tokens: TokenPair) async throws { self.tokens = tokens }
    public func clear() async throws {
        tokens = nil
        clearCount += 1
    }
}

/// Counts refreshes so single-flighting can be asserted, and can be told to fail.
public actor CountingRefreshTransport: TokenRefreshing {
    public enum Behaviour: Sendable {
        case succeed
        case fail(AppError)
    }

    public private(set) var callCount = 0
    private let behaviour: Behaviour
    private let delay: Duration

    public init(behaviour: Behaviour = .succeed, delay: Duration = .milliseconds(20)) {
        self.behaviour = behaviour
        self.delay = delay
    }

    public func refresh(using refreshToken: String) async throws -> TokenPair {
        callCount += 1
        try? await Task.sleep(for: delay)

        switch behaviour {
        case .succeed:
            return TokenPair(access: "access-\(callCount)", refresh: "refresh-\(callCount)")
        case .fail(let error):
            throw error
        }
    }
}

public struct StubMoodRepository: MoodRepository {
    private let summariesToReturn: [DaySummary]
    private let error: AppError?

    public init(summaries: [DaySummary] = [], error: AppError? = nil) {
        summariesToReturn = summaries
        self.error = error
    }

    public func summaries(around date: Date) async throws -> [DaySummary] {
        if let error { throw error }
        return summariesToReturn
    }

    public func log(_ mood: Mood, on date: Date) async throws {
        if let error { throw error }
    }
}

public struct FixedDateProvider: DateProvider {
    public let now: Date
    public init(_ now: Date) { self.now = now }
}
