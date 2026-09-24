import Foundation
import Security

/// Reaches a Keychain item beneath the type under test: to plant what the public API would
/// never write, and to read back the attributes it did write.
///
/// These tests live in the app-hosted bundle, not in the package, because a package test
/// bundle on the simulator has no Keychain — a write returns `-34018`
/// (`errSecMissingEntitlement`). Hosted by the signed app, they reach the same
/// data-protection Keychain a device uses (ADR 0026).
struct KeychainProbe: Sendable {
    let service: String
    let account: String

    /// A service no other test shares: Swift Testing runs tests in parallel, and two tests
    /// on one item would read each other's writes.
    static func unique(account: String) -> KeychainProbe {
        KeychainProbe(service: "test.\(UUID().uuidString)", account: account)
    }

    private var query: [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
    }

    func plant(_ data: Data) throws {
        var attributes = query
        attributes[kSecValueData as String] = data
        let status = SecItemAdd(attributes as CFDictionary, nil)
        guard status == errSecSuccess else { throw ProbeError(status: status) }
    }

    /// The item's attributes, or nil when there is no item.
    func attributes() throws -> [String: Any]? {
        var lookup = query
        lookup[kSecReturnAttributes as String] = true
        lookup[kSecMatchLimit as String] = kSecMatchLimitAll

        var result: CFTypeRef?
        let status = SecItemCopyMatching(lookup as CFDictionary, &result)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess, let items = result as? [[String: Any]] else {
            throw ProbeError(status: status)
        }
        // Two items under one service and account is itself the bug being looked for.
        guard items.count == 1 else { throw ProbeError(status: errSecDuplicateItem) }
        return items[0]
    }

    func remove() {
        SecItemDelete(query as CFDictionary)
    }

    struct ProbeError: Error {
        let status: OSStatus
    }
}
