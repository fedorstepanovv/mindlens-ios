import Core
import Models
import TestSupport
import Testing

@testable import Authentication

@Suite("Survey answers after sign-in")
@MainActor
struct SurveyAnswersSubmissionTests {

    private let answers = SurveyAnswers(goal: .fixSleep, feeling: 7, hurdle: .overthinking)

    @Test("a new account posts its answers, then lands on onboarding though the flag now reads complete")
    func newAccountPostsAnswers() async {
        let user = User.fake(onboarded: false)
        let onboarding = StubOnboardingRepository()
        let model = SessionModel(auth: StubAuthRepository(signIn: .user(user)), onboarding: onboarding)
        model.answers = answers

        await model.signInWithGoogle()

        #expect(await onboarding.submissions == [answers])
        #expect(model.state == .onboarding(user))
        #expect(model.answers == nil)
        #expect(!model.canRetryAnswers)
    }

    @Test("a returning account posts nothing and discards the answers")
    func returningAccountPostsNothing() async {
        let user = User.fake(onboarded: true)
        let onboarding = StubOnboardingRepository()
        let model = SessionModel(auth: StubAuthRepository(signIn: .user(user)), onboarding: onboarding)
        model.answers = answers

        await model.signInWithGoogle()

        #expect(await onboarding.submissions.isEmpty)
        #expect(model.state == .signedIn(user))
        #expect(model.answers == nil)
    }

    @Test("a new account with no answers lands on onboarding with nothing posted")
    func noAnswersPostsNothing() async {
        let user = User.fake(onboarded: false)
        let onboarding = StubOnboardingRepository()
        let model = SessionModel(auth: StubAuthRepository(signIn: .user(user)), onboarding: onboarding)

        await model.signInWithGoogle()

        #expect(await onboarding.submissions.isEmpty)
        #expect(model.state == .onboarding(user))
    }

    @Test("answers that fail to post keep the sign-in page, with the error and a retry")
    func failedPostStaysOnSignIn() async {
        let model = SessionModel(
            auth: StubAuthRepository(signIn: .user(.fake(onboarded: false))),
            onboarding: StubOnboardingRepository(failures: [AppError(kind: .offline)])
        )
        model.answers = answers

        await model.signInWithGoogle()

        #expect(model.state == .restoring)
        #expect(model.error?.kind == .offline)
        #expect(model.canRetryAnswers)
        #expect(model.pending == nil)
    }

    @Test("a retry posts the answers again without reopening the provider's sheet")
    func retryDoesNotSignInAgain() async {
        let user = User.fake(onboarded: false)
        let auth = StubAuthRepository(signIn: .user(user))
        let onboarding = StubOnboardingRepository(failures: [AppError(kind: .offline)])
        let model = SessionModel(auth: auth, onboarding: onboarding)
        model.answers = answers
        await model.signInWithGoogle()

        await model.retryAnswers()

        #expect(await auth.googleSignInCount == 1)
        #expect(await onboarding.submissions == [answers, answers])
        #expect(model.state == .onboarding(user))
        #expect(model.error == nil)
        #expect(!model.canRetryAnswers)
    }

    @Test("a retry that fails again keeps the retry on offer")
    func retryFailsAgain() async {
        let model = SessionModel(
            auth: StubAuthRepository(signIn: .user(.fake(onboarded: false))),
            onboarding: StubOnboardingRepository(
                failures: [AppError(kind: .offline), AppError(kind: .server(status: 500))]
            )
        )
        model.answers = answers
        await model.signInWithGoogle()

        await model.retryAnswers()

        #expect(model.error?.kind == .server(status: 500))
        #expect(model.canRetryAnswers)
        #expect(model.state == .restoring)
    }

    @Test("with nothing left to post, a retry does nothing")
    func retryWithNothingPending() async {
        let onboarding = StubOnboardingRepository()
        let model = SessionModel(auth: StubAuthRepository(), onboarding: onboarding)
        model.answers = answers

        await model.retryAnswers()

        #expect(await onboarding.submissions.isEmpty)
        #expect(model.state == .restoring)
    }

    @Test("signing out drops the pending answers and the retry")
    func signOutDropsPendingAnswers() async {
        let model = SessionModel(
            auth: StubAuthRepository(signIn: .user(.fake(onboarded: false))),
            onboarding: StubOnboardingRepository(failures: [AppError(kind: .offline)])
        )
        model.answers = answers
        await model.signInWithGoogle()

        await model.signOut()

        #expect(!model.canRetryAnswers)
        #expect(model.answers == nil)
        #expect(model.state == .signedOut)
    }
}
