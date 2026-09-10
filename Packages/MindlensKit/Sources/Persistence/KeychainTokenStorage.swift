import Core
import Foundation
import Security

/// Tokens live in the Keychain, not `UserDefaults`.
///
/// `kSecAttrAccessibleAfterFirstUnlock` so a background refresh works when the device is
/// locked, but the item still never leaves the device (`ThisDeviceOnly`).
public struct KeychainTokenStorage: TokenStorage {
    private let service: String

    public init(service: String = "care.mindlens.tokens") {
        self.service = service
    }

    /// Both tokens live in **one** Keychain item.
    ///
    /// Writing them as two items can tear: killed between the writes, the Keychain holds
    /// a new access token beside the *old* refresh token, which the server has already
    /// consumed. The next refresh then fails permanently and signs the user out — the
    /// exact incident ADR 0004 exists to prevent, arriving through the storage layer.
    public func load() async throws -> TokenPair? {
        guard let data = try read() else { return nil }
        return try JSONDecoder().decode(StoredTokens.self, from: data).pair
    }

    public func save(_ tokens: TokenPair) async throws {
        try write(try JSONEncoder().encode(StoredTokens(pair: tokens)))
    }

    public func clear() async throws {
        SecItemDelete(query() as CFDictionary)
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

    // MARK: - Private

    private func query() -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: "tokens",
        ]
    }

    private func read() throws -> Data? {
        var lookup = query()
        lookup[kSecReturnData as String] = true
        lookup[kSecMatchLimit as String] = kSecMatchLimitOne

        var item: CFTypeRef?
        let status = SecItemCopyMatching(lookup as CFDictionary, &item)

        switch status {
        case errSecSuccess:
            return item as? Data
        case errSecItemNotFound:
            return nil
        default:
            throw KeychainError(status: status)
        }
    }

    private func write(_ data: Data) throws {
        var attributes = query()
        attributes[kSecValueData as String] = data
        attributes[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly

        SecItemDelete(attributes as CFDictionary)
        let status = SecItemAdd(attributes as CFDictionary, nil)
        guard status == errSecSuccess else { throw KeychainError(status: status) }
    }
}

public struct KeychainError: Error, Equatable {
    public let status: OSStatus
}
