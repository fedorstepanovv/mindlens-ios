import Foundation

public struct TokenPair: Sendable, Equatable {
    public let access: String
    public let refresh: String

    public init(access: String, refresh: String) {
        self.access = access
        self.refresh = refresh
    }
}

/// Where tokens live between launches.
///
/// Declared here rather than in `Networking` so that `Persistence` can implement it
/// without depending on the networking layer — storage is a foundational concern, and
/// the arrow between those two modules would have pointed the wrong way.
public protocol TokenStorage: Sendable {
    func load() async throws -> TokenPair?
    func save(_ tokens: TokenPair) async throws
    func clear() async throws
}
