import Foundation
import Security

/// One generic-password Keychain item, addressed by service + account.
///
/// Extracted because two things now live in the Keychain — the token pair and the device
/// GUID — and a second hand-rolled copy of these four `SecItem` calls is a second place
/// for the accessibility attribute to be wrong. Getting that attribute wrong fails in the
/// worst way available: everything works while the device is unlocked.
struct KeychainItem: Sendable {
    let service: String
    let account: String

    private var query: [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
    }

    func read() throws -> Data? {
        var lookup = query
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

    /// `kSecAttrAccessibleAfterFirstUnlock` so a background token refresh works while the
    /// device is locked; `ThisDeviceOnly` so the item never travels to another device via
    /// backup or migration — a session and a device identity are both meaningless
    /// elsewhere.
    func write(_ data: Data) throws {
        var attributes = query
        attributes[kSecValueData as String] = data
        attributes[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly

        SecItemDelete(attributes as CFDictionary)
        let status = SecItemAdd(attributes as CFDictionary, nil)
        guard status == errSecSuccess else { throw KeychainError(status: status) }
    }

    func delete() {
        SecItemDelete(query as CFDictionary)
    }
}

public struct KeychainError: Error, Equatable {
    public let status: OSStatus

    public init(status: OSStatus) {
        self.status = status
    }
}
