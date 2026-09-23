import Foundation

/// Supplies the current instant, and the device's timezone.
///
/// Injected rather than read from `Date()` directly so that time-dependent behaviour —
/// local-day boundaries, recap windows, token expiry — is testable without waiting.
public protocol DateProvider: Sendable {
    var now: Date { get }

    /// The **device's** timezone.
    ///
    /// Reported to the server at sign-in, which is how the user's profile timezone gets
    /// set in the first place. Note the difference from the timezone that day-boundary
    /// math uses: that one comes from the user's profile, because a user who travels
    /// keeps their days where they live. This is the one place the ambient zone is the
    /// right answer, and it is a seam so tests can move the device somewhere else.
    var timeZone: TimeZone { get }
}

public struct SystemDateProvider: DateProvider {
    public init() {}

    public var now: Date { Date() }

    // The one read of the ambient zone: everything else takes a `DateProvider`.
    public var timeZone: TimeZone { TimeZone.current }
}
