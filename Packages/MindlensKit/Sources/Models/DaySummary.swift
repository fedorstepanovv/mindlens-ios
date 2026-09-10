import Foundation

/// One day on the dashboard.
public struct DaySummary: Sendable, Equatable, Identifiable {
    public let date: Date
    public let mood: Mood?
    public let tagCount: Int
    public let hasNote: Bool

    public var id: Date { date }

    public init(date: Date, mood: Mood?, tagCount: Int, hasNote: Bool) {
        self.date = date
        self.mood = mood
        self.tagCount = tagCount
        self.hasNote = hasNote
    }
}
