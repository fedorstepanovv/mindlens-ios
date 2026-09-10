import Core
import Foundation
import Models

/// The live `AuthRepository`, talking to the Mindlens API.
///
/// It lives in `Networking` rather than in the Authentication feature on purpose. Its only
/// collaborators are protocols from `Core` and the client next door, and keeping it here
/// means `AuthSessionDTO` never crosses a module boundary — the wire shape stays sealed
/// inside this target while the feature sees only `Models.User`. A copy of it inside the
/// feature would export the DTO to get there.
///
/// Nothing about Firebase appears here either: `IdentityAuthenticating` is injected, so this
/// type would be unchanged if the backend started accepting Apple's token directly.
public struct APIAuthRepository: AuthRepository {
    private let client: APIClient
    private let refresher: TokenRefresher
    private let identity: any IdentityAuthenticating
    private let device: any DeviceIdentifying
    private let dates: any DateProvider

    public init(
        client: APIClient,
        refresher: TokenRefresher,
        identity: any IdentityAuthenticating,
        device: any DeviceIdentifying,
        dates: any DateProvider = SystemDateProvider()
    ) {
        self.client = client
        self.refresher = refresher
        self.identity = identity
        self.device = device
        self.dates = dates
    }

    /// Nothing stored → `nil`, without a request. Stored → ask the server who this is.
    ///
    /// A 401 on that request is handled a layer down: `APIClient` refreshes once and
    /// retries, and only a refresh the server *rejects* surfaces as `.unauthenticated`.
    /// Every other error propagates as itself, which is what lets the caller tell "your
    /// session is over" from "you are on a train".
    public func restore() async throws -> User? {
        guard try await refresher.credentials() != nil else { return nil }
        return try await client.send(.currentUser)
    }

    public func signInWithApple(_ request: AppleIdentityRequest) async throws -> User {
        let credential = try await identity.exchangeAppleCredential(request)

        return try await establishSession(
            try .appleSignIn(
                identityToken: credential.firebaseIDToken,
                guid: await device.guid(),
                deviceModel: device.model,
                timezone: dates.timeZone.identifier,
                // Apple hands over an email only on the very first authorization, so the
                // identity provider may know one when Apple no longer does. Either source
                // will do; having neither for a new account is the server's 422.
                email: credential.email ?? request.email
            )
        )
    }

    public func signInWithGoogle() async throws -> User {
        let credential = try await identity.signInWithGoogle()

        return try await establishSession(
            try .googleSignIn(
                identityToken: credential.firebaseIDToken,
                guid: await device.guid(),
                deviceModel: device.model,
                timezone: dates.timeZone.identifier
            )
        )
    }

    /// Best effort remotely, unconditional locally.
    ///
    /// The logout call needs the access token, so it goes first. Its failure is discarded
    /// deliberately: a user who asked to sign out is signed out either way, and the
    /// server-side session expires on its own. The device GUID is **not** cleared — that is
    /// what makes the next sign-in reuse this device's session slot.
    public func signOut() async {
        _ = try? await client.send(.logout)
        await refresher.signOut()
    }

    // MARK: - Private

    private func establishSession(_ endpoint: Endpoint<AuthSessionDTO>) async throws -> User {
        let session = try await client.send(endpoint)

        // Adopted before returning, so the very next request is authenticated. A Keychain
        // failure here fails the sign-in, which is the opposite of the right answer during
        // *refresh* — and the difference is real: no single-use token has been consumed yet,
        // and retrying with the same device GUID replaces this session rather than spending
        // another of the user's five slots. A session that silently would not survive
        // relaunch is worse than a visible "try again".
        try await refresher.adopt(session.tokens.pair)

        return session.user
    }
}
