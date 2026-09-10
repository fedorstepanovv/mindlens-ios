import Foundation

/// The seam between features and how mood data is actually stored and synced.
///
/// Lives here rather than in a feature because more than one feature needs it. Features
/// depend on this protocol; the concrete local-first implementation is bound at the
/// app target's composition root.
public protocol MoodRepository: Sendable {
    func summaries(around date: Date) async throws -> [DaySummary]
    func log(_ mood: Mood, on date: Date) async throws
}
