import Foundation

/// The API accepts 1...4. Modelled as a closed set so an invalid rating cannot be built.
public enum Mood: Int, Sendable, CaseIterable, Codable, Identifiable {
    case low = 1
    case unsettled = 2
    case steady = 3
    case good = 4

    public var id: Int { rawValue }

    public var symbolName: String {
        switch self {
        case .low: "cloud.rain"
        case .unsettled: "cloud"
        case .steady: "cloud.sun"
        case .good: "sun.max"
        }
    }
}

/// A logged mood as the API returns it.
///
/// Note the asymmetry, which is real and not a typo: the request field is `moodRate`,
/// the response field is `mood`. See docs/API.md.
public struct MoodRecordDTO: Decodable, Sendable, Equatable {
    public let id: String
    public let mood: Int
    public let createdAt: Date
}
