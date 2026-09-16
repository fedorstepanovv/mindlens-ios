import Foundation

/// A calendar day with no time and no zone — `"2026-09-10"` on the wire.
///
/// The API uses three different date shapes (see docs/API.md): timestamps with
/// fractional seconds, plain days for `dateOfBirth` and range filters, and `"HH:mm"` for
/// reminders. One global `dateDecodingStrategy` cannot serve all three, so the two
/// non-timestamp shapes get their own types and encode themselves correctly by construction.
public struct CalendarDate: Codable, Sendable, Hashable, Comparable {
    public let year: Int
    public let month: Int
    public let day: Int

    public init(year: Int, month: Int, day: Int) {
        self.year = year
        self.month = month
        self.day = day
    }

    /// The calendar day `instant` falls on, in `timeZone`.
    ///
    /// "Today" is a *local* day. Which day an instant belongs to depends on the user's
    /// timezone, not the device's UTC offset — so the zone is required, never defaulted.
    public init(_ instant: Date, in timeZone: TimeZone) {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = timeZone
        let parts = calendar.dateComponents([.year, .month, .day], from: instant)
        year = parts.year ?? 1970
        month = parts.month ?? 1
        day = parts.day ?? 1
    }

    /// The first instant of this day in `timeZone`.
    public func startOfDay(in timeZone: TimeZone) -> Date? {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = timeZone
        return calendar.date(from: DateComponents(year: year, month: month, day: day))
    }

    public var wireFormat: String {
        String(format: "%04d-%02d-%02d", year, month, day)
    }

    public init(from decoder: any Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        let parts = raw.split(separator: "-")
        guard parts.count == 3,
            let year = Int(parts[0]), let month = Int(parts[1]), let day = Int(parts[2])
        else {
            throw DecodingError.dataCorrupted(
                .init(codingPath: decoder.codingPath, debugDescription: "Expected YYYY-MM-DD, got \(raw)")
            )
        }
        self.init(year: year, month: month, day: day)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.singleValueContainer()
        try container.encode(wireFormat)
    }

    public static func < (lhs: Self, rhs: Self) -> Bool {
        (lhs.year, lhs.month, lhs.day) < (rhs.year, rhs.month, rhs.day)
    }
}

/// A wall-clock time of day — `"HH:mm"` on the wire, used by reminder settings.
public struct TimeOfDay: Codable, Sendable, Hashable, Comparable {
    public let hour: Int
    public let minute: Int

    public init(hour: Int, minute: Int) {
        self.hour = hour
        self.minute = minute
    }

    public var wireFormat: String { String(format: "%02d:%02d", hour, minute) }

    public init(from decoder: any Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        let parts = raw.split(separator: ":")
        guard parts.count == 2, let hour = Int(parts[0]), let minute = Int(parts[1]),
            (0..<24).contains(hour), (0..<60).contains(minute)
        else {
            throw DecodingError.dataCorrupted(
                .init(codingPath: decoder.codingPath, debugDescription: "Expected HH:mm, got \(raw)")
            )
        }
        self.init(hour: hour, minute: minute)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.singleValueContainer()
        try container.encode(wireFormat)
    }

    public static func < (lhs: Self, rhs: Self) -> Bool {
        (lhs.hour, lhs.minute) < (rhs.hour, rhs.minute)
    }
}
