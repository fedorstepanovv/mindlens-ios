import Foundation
import Testing

@testable import Core

@Suite("Dates and timezones")
struct DateHandlingTests {

    @Test("a local day depends on the user's timezone, not the device's")
    func localDayFollowsTimeZone() throws {
        // 23:30 UTC on the 10th is already the 11th in Kyiv.
        let instant = try #require(
            try Date.ISO8601FormatStyle(includingFractionalSeconds: false).parse("2026-09-10T23:30:00Z")
        )

        let utc = CalendarDate(instant, in: try #require(TimeZone(identifier: "UTC")))
        let kyiv = CalendarDate(instant, in: try #require(TimeZone(identifier: "Europe/Kyiv")))
        let la = CalendarDate(instant, in: try #require(TimeZone(identifier: "America/Los_Angeles")))

        #expect(utc.day == 10)
        #expect(kyiv.day == 11)
        #expect(la.day == 10)
    }

    @Test("round-trips the wire format the API expects")
    func calendarDateRoundTrip() throws {
        let json = Data(#""2026-09-10""#.utf8)
        let decoded = try JSONDecoder().decode(CalendarDate.self, from: json)

        #expect(decoded == CalendarDate(year: 2026, month: 9, day: 10))
        #expect(decoded.wireFormat == "2026-09-10")
        #expect(try JSONEncoder().encode(decoded) == json)
    }

    @Test("rejects a malformed day rather than guessing")
    func rejectsBadCalendarDate() {
        #expect(throws: (any Error).self) {
            try JSONDecoder().decode(CalendarDate.self, from: Data(#""10/09/2026""#.utf8))
        }
    }

    @Test("reminder times round-trip as HH:mm")
    func timeOfDayRoundTrip() throws {
        let decoded = try JSONDecoder().decode(TimeOfDay.self, from: Data(#""09:05""#.utf8))
        #expect(decoded == TimeOfDay(hour: 9, minute: 5))
        #expect(decoded.wireFormat == "09:05")
    }

    @Test("rejects an out-of-range time", arguments: [#""24:00""#, #""12:60""#, #""noon""#])
    func rejectsBadTime(raw: String) {
        #expect(throws: (any Error).self) {
            try JSONDecoder().decode(TimeOfDay.self, from: Data(raw.utf8))
        }
    }
}
