import Core
import Foundation
import Persistence
import Security
import Testing

@Suite("Keychain token storage")
struct KeychainTokenStorageTests {

    private let probe = KeychainProbe.unique(account: "tokens")
    private var storage: KeychainTokenStorage { KeychainTokenStorage(service: probe.service) }

    @Test("an empty Keychain loads no session, rather than throwing")
    func emptyLoadsNil() async throws {
        #expect(try await storage.load() == nil)
    }

    @Test("a saved pair loads back whole, and from a fresh instance")
    func roundTrips() async throws {
        defer { probe.remove() }
        let pair = TokenPair(access: "access", refresh: "refresh")

        try await storage.save(pair)

        #expect(try await KeychainTokenStorage(service: probe.service).load() == pair)
    }

    /// The pair is one item so a kill between two writes cannot leave a new access token
    /// beside a refresh token the server has already consumed (ADR 0004).
    @Test("a rotation replaces the pair in one item, never beside it")
    func rotationReplacesInPlace() async throws {
        defer { probe.remove() }
        try await storage.save(TokenPair(access: "old-access", refresh: "old-refresh"))

        try await storage.save(TokenPair(access: "new-access", refresh: "new-refresh"))

        #expect(try await storage.load() == TokenPair(access: "new-access", refresh: "new-refresh"))
        #expect(try probe.attributes() != nil)
    }

    @Test("clear ends the session, and clearing twice is not an error")
    func clears() async throws {
        defer { probe.remove() }
        try await storage.save(TokenPair(access: "access", refresh: "refresh"))

        try await storage.clear()
        try await storage.clear()

        #expect(try await storage.load() == nil)
        #expect(try probe.attributes() == nil)
    }

    /// Readable after first unlock, so a background refresh works on a locked device; never
    /// restored to another device, where the session means nothing.
    @Test("the item is readable after first unlock and never leaves this device")
    func accessibility() async throws {
        defer { probe.remove() }
        try await storage.save(TokenPair(access: "access", refresh: "refresh"))

        let attributes = try #require(try probe.attributes())

        #expect(
            attributes[kSecAttrAccessible as String] as? String
                == kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly as String
        )
    }

    /// `write` deleted with its *attributes*, accessibility class included, so an item stored
    /// under another class survived the delete, and the add failed as a duplicate.
    @Test(
        "a save replaces an item stored under another accessibility class",
        .bug("write deleted by attributes, so an item under another class survived it")
    )
    func saveReplacesItemUnderAnotherClass() async throws {
        defer { probe.remove() }
        try probe.plant(Data("written by an older build".utf8))

        try await storage.save(TokenPair(access: "access", refresh: "refresh"))

        #expect(try await storage.load() == TokenPair(access: "access", refresh: "refresh"))
        let attributes = try #require(try probe.attributes())
        #expect(
            attributes[kSecAttrAccessible as String] as? String
                == kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly as String
        )
    }

    @Test("an item that is not a stored pair throws, rather than loading as signed out")
    func unreadableItemThrows() async throws {
        defer { probe.remove() }
        try probe.plant(Data("not json".utf8))

        await #expect(throws: DecodingError.self) { try await storage.load() }
    }
}
