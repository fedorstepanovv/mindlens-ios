import Core
import Foundation

/// Tokens live in the Keychain, not `UserDefaults`.
public struct KeychainTokenStorage: TokenStorage {
    private let item: KeychainItem

    public init(service: String = "care.mindlens.tokens") {
        item = KeychainItem(service: service, account: "tokens")
    }

    /// Both tokens live in **one** Keychain item.
    ///
    /// Writing them as two items can tear: killed between the writes, the Keychain holds
    /// a new access token beside the *old* refresh token, which the server has already
    /// consumed. The next refresh then fails permanently and signs the user out — the
    /// exact incident ADR 0004 exists to prevent, arriving through the storage layer.
    public func load() async throws -> TokenPair? {
        guard let data = try item.read() else { return nil }
        return try JSONDecoder().decode(StoredTokens.self, from: data).pair
    }

    public func save(_ tokens: TokenPair) async throws {
        try item.write(try JSONEncoder().encode(StoredTokens(pair: tokens)))
    }

    public func clear() async throws {
        item.delete()
    }

    private struct StoredTokens: Codable {
        let access: String
        let refresh: String

        init(pair: TokenPair) {
            access = pair.access
            refresh = pair.refresh
        }

        var pair: TokenPair { TokenPair(access: access, refresh: refresh) }
    }
}
