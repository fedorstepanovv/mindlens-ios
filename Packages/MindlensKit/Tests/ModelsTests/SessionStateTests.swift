import Models
import TestSupport
import Testing

@Suite("Session state")
struct SessionStateTests {

    /// The previous two tests here compared distinct cases of a synthesised `Equatable` enum —
    /// `.restoring != .signedOut` — which is true of every enum ever written and says nothing
    /// about this one. `docs/TESTING.md` names that shape as the thing not to do.
    ///
    /// What is worth asserting is the mapping, because it is a real decision: the same sign-in
    /// path serves a new account and a returning one, and the server's only signal for which is
    /// `isOnboardingComplete`. Getting it backwards drops a new user on a dashboard with no data.
    @Test(
        "a user's onboarding flag decides which authenticated tree they land in",
        arguments: [true, false]
    )
    func mapsOnboardingFlagToState(onboarded: Bool) {
        let user = User.fake(onboarded: onboarded)

        let state = SessionState(authenticated: user)

        #expect(state == (onboarded ? .signedIn(user) : .onboarding(user)))
    }

    @Test("the user is reachable from either authenticated state, and from neither other one")
    func exposesTheUser() {
        let user = User.fake()

        #expect(SessionState.signedIn(user).user == user)
        #expect(SessionState.onboarding(user).user == user)
        #expect(SessionState.restoring.user == nil)
        #expect(SessionState.signedOut.user == nil)
    }
}
