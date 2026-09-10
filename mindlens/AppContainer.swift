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

    init(
        identity: any IdentityAuthenticating = UnavailableIdentityProvider(),
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
