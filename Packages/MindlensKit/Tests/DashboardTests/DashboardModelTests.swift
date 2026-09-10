import Core
import Foundation
import Models
import TestSupport
import Testing

@testable import Dashboard

@Suite("Dashboard")
@MainActor
struct DashboardModelTests {

    private let day = Date(timeIntervalSince1970: 1_757_520_000)

    @Test("surfaces an error when loading fails")
    func showsError() async {
        let model = DashboardModel(
            moods: StubMoodRepository(error: AppError(kind: .offline, message: "offline")),
            dates: FixedDateProvider(day)
        )

        await model.load()

        #expect(model.error?.kind == .offline)
        #expect(model.summaries.isEmpty)
        #expect(model.isLoading == false)
    }

    @Test("clears a previous error once loading succeeds")
    func clearsErrorOnSuccess() async {
        let summary = DaySummary(date: day, mood: .steady, tagCount: 2, hasNote: true)
        let model = DashboardModel(
            moods: StubMoodRepository(summaries: [summary]), dates: FixedDateProvider(day))

        await model.load()

        #expect(model.error == nil)
        #expect(model.summaries == [summary])
    }

    @Test("is not left loading after a failure")
    func resetsLoadingFlag() async {
        let model = DashboardModel(
            moods: StubMoodRepository(error: AppError(kind: .unknown, message: "boom")),
            dates: FixedDateProvider(day)
        )

        await model.load()

        #expect(model.isLoading == false)
    }
}
