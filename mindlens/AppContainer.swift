import Analytics
import Authentication
import Core
import Models
import Networking
import Persistence

/// The composition root: the only place that knows concrete types.
///
/// Built once at launch and handed down. There is no global container and no service locator —
/// a dependency you cannot substitute is a test you cannot write.
@MainActor
final class AppContainer {
    let session: SessionModel

    /// Production wiring. Firebase when a `GoogleService-Info.plist` is bundled; otherwise the
    /// stand-in that throws — which is every CI run and every fresh clone, since the plist is a
    /// credential and is gitignored. Same build either way; the file decides.
    ///
    /// Debug only. A Release build with no plist is misconfigured, not missing a feature, and it
    /// fails the way `AppConfiguration.apiBaseURL` does: at once, before anyone taps Sign in
    /// and reads "something went wrong" on every attempt.
    convenience init() {
        if let firebase = FirebaseIdentityProvider.configuringFirebase() {
            self.init(identity: firebase)
        } else {
            #if DEBUG
            self.init(identity: UnavailableIdentityProvider())
            #else
            preconditionFailure("GoogleService-Info.plist is not in the bundle; this build cannot sign in.")
            #endif
        }
    }

    init(
        identity: any IdentityAuthenticating,
        analytics: any AnalyticsRecording = .noop
    ) {
        let baseURL = AppConfiguration.apiBaseURL
        let refresher = TokenRefresher(
            transport: AuthTokenRefreshTransport(baseURL: baseURL),
            storage: KeychainTokenStorage()
        )

        session = SessionModel(
            auth: APIAuthRepository(
                client: APIClient(baseURL: baseURL, refresher: refresher),
                refresher: refresher,
                identity: identity,
                device: KeychainDeviceIdentity()
            ),
            analytics: analytics
        )
    }
}
