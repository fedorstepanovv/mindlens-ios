import Analytics
import Core
import Foundation
import Models
import Observation

@Observable
@MainActor
public final class DashboardModel {
    public private(set) var summaries: [DaySummary] = []
    public private(set) var isLoading = false
    public private(set) var error: AppError?

    public var selectedDate: Date

    private let moods: any MoodRepository
    private let analytics: any AnalyticsRecording

    public init(
        moods: any MoodRepository,
        analytics: any AnalyticsRecording = .noop,
        dates: any DateProvider = SystemDateProvider()
    ) {
        self.moods = moods
        self.analytics = analytics
        self.selectedDate = dates.now
    }

    public func load() async {
        isLoading = true
        defer { isLoading = false }

        do {
            summaries = try await moods.summaries(around: selectedDate)
            error = nil
        } catch {
            self.error = AppError(error)
        }
    }

    public func log(_ mood: Mood) async {
        do {
            try await moods.log(mood, on: selectedDate)
            analytics.record(AnalyticsEvent("daily_log_submitted"))
            await load()
        } catch {
            self.error = AppError(error)
        }
    }
}
