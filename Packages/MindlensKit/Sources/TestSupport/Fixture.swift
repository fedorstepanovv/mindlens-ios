import Foundation

/// Loads a real captured API response.
///
/// Fixtures are captured from the live API, never hand-written — a hand-written fixture
/// asserts what we hoped the server does. See docs/TESTING.md.
public enum Fixture {
    public static func data(_ name: String) throws -> Data {
        guard
            let url = Bundle.module.url(forResource: name, withExtension: "json", subdirectory: "Fixtures")
                ?? Bundle.module.url(forResource: name, withExtension: "json")
        else {
            throw FixtureMissing(name: name)
        }
        return try Data(contentsOf: url)
    }
}

public struct FixtureMissing: Error, CustomStringConvertible {
    public let name: String
    public var description: String { "No fixture named \(name).json. Capture one from the API." }
}
