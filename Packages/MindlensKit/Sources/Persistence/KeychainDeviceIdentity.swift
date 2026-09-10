import Core
import Foundation

/// The device GUID, created once and kept for the life of the install.
///
/// An `actor` because "read it, and create it if absent" is a read-modify-write. Two
/// concurrent sign-in taps racing through that produce two different GUIDs, and the server
/// reads a GUID it has never seen as a new device — quietly spending one of the user's five
/// session slots and evicting the oldest.
public actor KeychainDeviceIdentity: DeviceIdentifying {
    nonisolated public let model: String

    private let item: KeychainItem
    private var cached: String?

    public init(service: String = "care.mindlens.device", model: String = SystemDeviceModel().value) {
        item = KeychainItem(service: service, account: "guid")
        self.model = model
    }

    public func guid() async -> String {
        if let cached { return cached }

        // A read failure is not a reason to refuse the read: an item written under a
        // different accessibility class, or a Keychain that is simply unhappy, both surface
        // here and neither means "no GUID exists".
        if let stored = try? item.read(), let existing = String(data: stored, encoding: .utf8),
            existing.count >= 6
        {
            cached = existing
            return existing
        }

        let fresh = UUID().uuidString
        cached = fresh

        // Deliberately not `throws`. If the Keychain will not hold the GUID, the user still
        // gets to sign in — they simply spend a session slot on this install more than once.
        // Blocking sign-in on a storage hiccup trades a degraded feature for no app at all.
        try? item.write(Data(fresh.utf8))
        return fresh
    }

    /// Sign-out must **not** call this.
    ///
    /// The GUID is what makes the next sign-in reuse this device's session slot instead of
    /// orphaning a new one, so it deliberately outlives the session. Exposed only so
    /// account deletion has a way to forget the device entirely.
    public func forget() {
        cached = nil
        item.delete()
    }
}
