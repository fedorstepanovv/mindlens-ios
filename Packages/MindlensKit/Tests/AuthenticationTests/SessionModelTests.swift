import Analytics
import Core
import Models
import TestSupport
import Testing

@testable import Authentication

@Suite("Session restore")
@MainActor
struct SessionRestoreTests {

    @Test("nothing stored means signed out, without inventing a session")
    func noStoredSession() async {
        let model = SessionModel(auth: StubAuthRepository(restore: .none))

        await model.restore()

        #expect(model.state == .signedOut)
        #expect(model.error == nil)
    }

    @Test("a restored user who finished onboarding lands signed in")
    func restoredOnboardedUser() async {
        let user = User.fake(onboarded: true)
        let model = SessionModel(auth: StubAuthRepository(restore: .user(user)))

        await model.restore()

        #expect(model.state == .signedIn(user))
    }

    @Test("a restored user mid-onboarding resumes onboarding, not the dashboard")
    func restoredOnboardingUser() async {
        let user = User.fake(onboarded: false)
        let model = SessionModel(auth: StubAuthRepository(restore: .user(user)))

        await model.restore()

        #expect(model.state == .onboarding(user))
    }

    @Test("the server rejecting the stored session signs out")
    func rejectedSession() async {
        let model = SessionModel(
            auth: StubAuthRepository(restore: .failure(AppError(kind: .unauthenticated)))
        )

        await model.restore()

        #expect(model.state == .signedOut)
    }

    /// The failure mode this guards is a launch with no network reading as a sign-out. The
    /// tokens are still valid, so dropping to the sign-in screen both loses the session and
    /// strands the user there — they cannot sign back in either.
    @Test(
        "a transient failure at launch keeps the session and offers a retry",
        arguments: [
            AppError.Kind.offline,
            .server(status: 500),
            .throttled(retryAfter: nil),
            .decoding,
        ]
    )
    func transientRestoreFailure(kind: AppError.Kind) async {
        let model = SessionModel(auth: StubAuthRepository(restore: .failure(AppError(kind: kind))))

        await model.restore()

        #expect(model.state == .restoring)
        #expect(model.needsRestoreRetry)
        #expect(model.error?.kind == kind)
    }

    @Test("a retry after a transient failure clears the error before trying again")
    func retryClearsError() async {
        let model = SessionModel(auth: StubAuthRepository(restore: .failure(AppError(kind: .offline))))
        await model.restore()
        #expect(model.needsRestoreRetry)

        let recovered = SessionModel(auth: StubAuthRepository(restore: .user(.fake())))
        await recovered.restore()

        #expect(recovered.error == nil)
        #expect(recovered.needsRestoreRetry == false)
    }

    @Test("a cancelled restore is not an error")
    func cancelledRestore() async {
        let model = SessionModel(auth: StubAuthRepository(restore: .cancelled))

        await model.restore()

        #expect(model.error == nil)
        #expect(model.state == .restoring)
    }
}

@Suite("Sign in")
@MainActor
struct SignInTests {

    @Test("Apple's credential reaches the repository with the raw nonce, never the hash")
    func appleRequestCarriesRawNonce() async throws {
        let repository = StubAuthRepository(signIn: .user(.fake()))
        let model = SessionModel(auth: repository)

        let hashed = model.appleRequestNonce()
        await model.signInWithApple(identityToken: "apple-token", authorizationCode: "code", email: nil)

        let request = try #require(await repository.appleRequests.first)
        #expect(request.identityToken == "apple-token")
        #expect(request.authorizationCode == "code")
        // The hash goes to Apple; the raw value goes to the identity exchange. Sending the raw
        // one to Apple would make the token replayable, so these must not be equal.
        #expect(request.rawNonce != hashed)
        #expect(SignInNonce(raw: request.rawNonce).hashed == hashed)
    }

    @Test("a nonce is single-use: a second credential on the same nonce is refused")
    func nonceIsSingleUse() async {
        let repository = StubAuthRepository(signIn: .user(.fake()))
        let model = SessionModel(auth: repository)

        _ = model.appleRequestNonce()
        await model.signInWithApple(identityToken: "first", authorizationCode: nil, email: nil)
        await model.signInWithApple(identityToken: "second", authorizationCode: nil, email: nil)

        #expect(await repository.appleRequests.count == 1)
        #expect(model.error != nil)
    }

    @Test("an Apple credential with no nonce prepared never reaches the network")
    func appleWithoutNonce() async {
        let repository = StubAuthRepository(signIn: .user(.fake()))
        let model = SessionModel(auth: repository)

        await model.signInWithApple(identityToken: "token", authorizationCode: nil, email: nil)

        #expect(await repository.appleRequests.isEmpty)
        #expect(model.error?.kind == .unknown)
        #expect(model.state == .restoring)
    }

    @Test("signing in with Google reaches the repository and moves the gate")
    func googleSignIn() async {
        let user = User.fake(onboarded: true)
        let repository = StubAuthRepository(signIn: .user(user))
        let model = SessionModel(auth: repository)

        await model.signInWithGoogle()

        #expect(await repository.googleSignInCount == 1)
        #expect(model.state == .signedIn(user))
        #expect(model.pending == nil)
    }

    @Test("a new account is sent to onboarding, not to the dashboard")
    func newAccountGoesToOnboarding() async {
        let user = User.fake(onboarded: false)
        let model = SessionModel(auth: StubAuthRepository(signIn: .user(user)))

        await model.signInWithGoogle()

        #expect(model.state == .onboarding(user))
    }

    @Test("a failed sign-in shows an error and leaves the gate where it was")
    func failedSignIn() async {
        let model = SessionModel(
            auth: StubAuthRepository(signIn: .failure(AppError(kind: .server(status: 422))))
        )

        await model.signInWithGoogle()

        #expect(model.error?.kind == .server(status: 422))
        #expect(model.state == .restoring)
        #expect(model.pending == nil)
    }

    @Test("cancelling mid-flow leaves no error behind")
    func cancelledSignIn() async {
        let model = SessionModel(auth: StubAuthRepository(signIn: .cancelled))

        await model.signInWithGoogle()

        #expect(model.error == nil)
        #expect(model.pending == nil)
    }

    /// Dismissing the provider sheet is the most common outcome of tapping a sign-in button.
    /// A banner for it would mean the app scolds the user for changing their mind.
    @Test("a dismissed provider sheet is silent, while a real provider failure is not")
    func providerOutcomesAreDistinguished() async {
        let cancelled = SessionModel(auth: StubAuthRepository())
        _ = cancelled.appleRequestNonce()
        cancelled.signInWasCancelled()
        #expect(cancelled.error == nil)

        let failed = SessionModel(auth: StubAuthRepository())
        failed.signInFailed(AppError(kind: .offline))
        #expect(failed.error?.kind == .offline)
        #expect(failed.pending == nil)
    }

    @Test("a cancelled sheet also discards the nonce it prepared")
    func cancellationDiscardsNonce() async {
        let repository = StubAuthRepository()
        let model = SessionModel(auth: repository)

        _ = model.appleRequestNonce()
        model.signInWasCancelled()
        await model.signInWithApple(identityToken: "token", authorizationCode: nil, email: nil)

        #expect(await repository.appleRequests.isEmpty)
    }
}

@Suite("Session analytics and sign-out")
@MainActor
struct SessionSideEffectTests {

    @Test("a completed sign-in records the provider and whether the account is new")
    func signInAnalytics() async throws {
        let analytics = RecordingAnalytics()
        let model = SessionModel(
            auth: StubAuthRepository(signIn: .user(.fake(onboarded: false))), analytics: analytics
        )

        await model.signInWithGoogle()

        let event = try #require(analytics.events.first)
        #expect(event.name == "sign_in_completed")
        #expect(event.properties["provider"] == "google")
        #expect(event.properties["new_account"] == "true")
    }

    @Test("a returning account is not counted as new")
    func returningAccountAnalytics() async throws {
        let analytics = RecordingAnalytics()
        let model = SessionModel(
            auth: StubAuthRepository(signIn: .user(.fake(onboarded: true))), analytics: analytics
        )

        await model.signInWithGoogle()

        #expect(try #require(analytics.events.first).properties["new_account"] == "false")
    }

    @Test("a failed sign-in records nothing")
    func noAnalyticsOnFailure() async {
        let analytics = RecordingAnalytics()
        let model = SessionModel(
            auth: StubAuthRepository(signIn: .failure(AppError(kind: .offline))), analytics: analytics
        )

        await model.signInWithGoogle()

        #expect(analytics.events.isEmpty)
    }

    @Test("signing out ends the session locally and clears any error")
    func signOut() async {
        let repository = StubAuthRepository(signIn: .failure(AppError(kind: .offline)))
        let model = SessionModel(auth: repository)
        await model.signInWithGoogle()
        #expect(model.error != nil)

        await model.signOut()

        #expect(await repository.signOutCount == 1)
        #expect(model.state == .signedOut)
        #expect(model.error == nil)
    }
}
