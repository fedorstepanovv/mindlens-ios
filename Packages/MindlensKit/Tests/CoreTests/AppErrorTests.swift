import Foundation
import Testing

@testable import Core

@Suite("AppError")
struct AppErrorTests {

    @Test("maps a lost connection to offline")
    func mapsOffline() {
        let error = AppError(URLError(.notConnectedToInternet))
        #expect(error.kind == .offline)
        #expect(error.isRetryable)
    }

    @Test("does not wrap an AppError twice")
    func preservesExisting() {
        let original = AppError(kind: .server(status: 500))
        #expect(AppError(original) == original)
    }

    @Test(
        "server errors are retryable only when the server may recover",
        arguments: [
            (500, true), (503, true), (400, false), (404, false),
        ])
    func retryability(status: Int, expected: Bool) {
        #expect(AppError(kind: .server(status: status)).isRetryable == expected)
    }
}
