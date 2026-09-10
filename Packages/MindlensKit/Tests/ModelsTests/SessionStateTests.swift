import Models
import Testing

@Suite("Session state")
struct SessionStateTests {

    private let user = User(
        id: 1, email: "a@b.c", authProvider: .apple, timezone: "Europe/Kyiv", isOnboardingComplete: true)

    @Test("restoring is distinct from signed out so launch does not flash the sign-in screen")
    func restoringIsNotSignedOut() {
        #expect(SessionState.restoring != .signedOut)
    }

    @Test("an onboarding user is not treated as fully signed in")
    func onboardingIsNotSignedIn() {
        #expect(SessionState.onboarding(user) != .signedIn(user))
    }
}
