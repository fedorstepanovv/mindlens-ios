import Foundation

/// Supplies the current instant.
///
/// Injected rather than read from `Date()` directly so that time-dependent behaviour —
/// local-day boundaries, recap windows, token expiry — is testable without waiting.
public protocol DateProvider: Sendable {
    var now: Date { get }
}

public struct SystemDateProvider: DateProvider {
    public init() {}
    public var now: Date { Date() }
}
