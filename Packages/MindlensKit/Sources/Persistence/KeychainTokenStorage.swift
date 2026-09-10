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

    public func load() async throws -> TokenPair? {
        guard let access = try read(account: "access"), let refresh = try read(account: "refresh") else {
            return nil
        }
        return TokenPair(access: access, refresh: refresh)
    }

    public func save(_ tokens: TokenPair) async throws {
        try write(tokens.access, account: "access")
        try write(tokens.refresh, account: "refresh")
    }

    public func clear() async throws {
        for account in ["access", "refresh"] {
            SecItemDelete(query(account: account) as CFDictionary)
        }
    }

    // MARK: - Private

    private func query(account: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
    }

    private func read(account: String) throws -> String? {
        var lookup = query(account: account)
        lookup[kSecReturnData as String] = true
        lookup[kSecMatchLimit as String] = kSecMatchLimitOne

        var item: CFTypeRef?
        let status = SecItemCopyMatching(lookup as CFDictionary, &item)

        switch status {
        case errSecSuccess:
            guard let data = item as? Data else { return nil }
            return String(data: data, encoding: .utf8)
        case errSecItemNotFound:
            return nil
        default:
            throw KeychainError(status: status)
        }
    }

    private func write(_ value: String, account: String) throws {
        let data = Data(value.utf8)
        var attributes = query(account: account)
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
