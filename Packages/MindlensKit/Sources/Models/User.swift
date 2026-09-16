import Foundation

public enum AuthProvider: String, Decodable, Sendable {
    case apple = "APPLE"
    case google = "GOOGLE"
    /// A provider this build predates. Decoding must not fail the whole response —
    /// the server is unversioned, so a new provider can appear at any time.
    case unknown

    public init(from decoder: any Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = AuthProvider(rawValue: raw) ?? .unknown
    }
}

public struct User: Decodable, Sendable, Equatable, Identifiable {
    public let id: Int
    public let email: String
    public let authProvider: AuthProvider
    public let timezone: String
    public let isOnboardingComplete: Bool

    public init(
        id: Int, email: String, authProvider: AuthProvider, timezone: String, isOnboardingComplete: Bool
    ) {
        self.id = id
        self.email = email
        self.authProvider = authProvider
        self.timezone = timezone
        self.isOnboardingComplete = isOnboardingComplete
    }
}
