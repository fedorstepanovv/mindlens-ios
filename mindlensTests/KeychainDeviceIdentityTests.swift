import Foundation
import Persistence
import Security
import Testing

@Suite("Keychain device identity")
struct KeychainDeviceIdentityTests {

    private let probe = KeychainProbe.unique(account: "guid")

    private func identity() -> KeychainDeviceIdentity {
        KeychainDeviceIdentity(service: probe.service, model: "iPhone")
    }

    /// The server keys a session slot on the GUID, so one that changes per launch quietly
    /// evicts the user's other devices.
    @Test("the GUID outlives the instance that created it")
    func persistsAcrossLaunches() async throws {
        defer { probe.remove() }

        let first = await identity().guid()
        let second = await identity().guid()

        #expect(first == second)
        #expect(UUID(uuidString: first) != nil)
    }

    @Test("concurrent sign-in taps on one install share one GUID")
    func concurrentCallersShareOne() async throws {
        defer { probe.remove() }
        let shared = identity()

        let guids = await withTaskGroup(of: String.self) { group in
            for _ in 0..<20 { group.addTask { await shared.guid() } }
            return await group.reduce(into: Set<String>()) { $0.insert($1) }
        }

        #expect(guids.count == 1)
        #expect(guids.first == (await identity().guid()))
    }

    /// The API accepts 6–36 characters. A stored value outside that would be cached and
    /// re-sent forever: a permanent 400 on every sign-in. Replaced in memory is not enough —
    /// the write once failed silently, and every launch minted a GUID of its own.
    @Test(
        "a stored GUID the API would reject is replaced, not reused",
        .bug("write deleted by attributes, so an item under another class survived it"),
        arguments: ["short", String(repeating: "x", count: 37)]
    )
    func invalidStoredGUIDIsReplaced(stored: String) async throws {
        defer { probe.remove() }
        try probe.plant(Data(stored.utf8))

        let guid = await identity().guid()

        #expect(guid != stored)
        #expect((6...36).contains(guid.count))
        #expect(await identity().guid() == guid)
    }

    @Test("a stored GUID the API accepts is reused as it is")
    func validStoredGUIDIsReused() async throws {
        defer { probe.remove() }
        try probe.plant(Data("stored-guid".utf8))

        #expect(await identity().guid() == "stored-guid")
    }

    @Test("forgetting the device makes the next sign-in a new one")
    func forgetIssuesANewGUID() async throws {
        defer { probe.remove() }
        let device = identity()
        let before = await device.guid()

        await device.forget()

        #expect(try probe.attributes() == nil)
        #expect(await device.guid() != before)
    }

    @Test("the GUID is readable after first unlock and never leaves this device")
    func accessibility() async throws {
        defer { probe.remove() }
        _ = await identity().guid()

        let attributes = try #require(try probe.attributes())

        #expect(
            attributes[kSecAttrAccessible as String] as? String
                == kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly as String
        )
    }
}
