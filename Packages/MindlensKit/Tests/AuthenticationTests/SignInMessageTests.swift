import Core
import DesignSystem
import Testing

@testable import Authentication

@Suite("Sign-in error copy")
@MainActor
struct SignInMessageTests {

    /// The one error with a remedy the user can act on. A `case .server(let status) where status
    /// == 422` compiles just as happily when the number is wrong, and nothing else would notice.
    @Test("a 422 explains the private-relay remedy instead of shrugging")
    func unprocessableExplainsItself() {
        let message = AppError(kind: .server(status: 422)).signInMessage

        #expect(message.contains("Share My Email"))
        #expect(message != AppError(kind: .server(status: 422)).displayMessage)
    }

    /// Everything else defers to the shared mapping rather than inventing feature-specific copy.
    @Test(
        "other failures use the shared wording",
        arguments: [
            AppError.Kind.offline,
            .unauthenticated,
            .server(status: 500),
            .server(status: 409),
            .decoding,
            .unknown,
        ]
    )
    func othersDeferToSharedCopy(kind: AppError.Kind) {
        let error = AppError(kind: kind)

        #expect(error.signInMessage == error.displayMessage)
    }
}
