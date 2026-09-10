import Foundation

public enum AuthProvider: String, Decodable, Sendable {
    case apple = "APPLE"
    case google = "GOOGLE"
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
