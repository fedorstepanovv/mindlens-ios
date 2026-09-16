import Core
import Foundation

/// The live `POST /auth/refresh` call.
///
/// **Deliberately does not go through `APIClient`.** Two reasons, both structural:
///
/// 1. The refresh token goes in the `Authorization` header where the access token normally
///    goes. `APIClient` gets its bearer from the refresher, so it cannot express this.
/// 2. `APIClient` retries a 401 by asking the refresher to refresh. Routing refresh through
///    it would make a rejected refresh token trigger another refresh.
///
/// Classification matters as much as the call — see ADR 0004. A **400** here is a rejected
/// refresh token, not a malformed request, and must reach `TokenRefresher` as a definitive
/// `.server(status: 400)` so the session ends once, cleanly. A timeout or a 5xx must reach
/// it as something transient, or a thirty-second backend blip signs out every user.
public struct AuthTokenRefreshTransport: TokenRefreshing {
    private let baseURL: URL
    private let session: URLSession

    public init(baseURL: URL, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
    }

    public func refresh(using refreshToken: String) async throws -> TokenPair {
        var request = URLRequest(url: baseURL.appending(path: "/auth/refresh"))
        request.httpMethod = HTTPMethod.post.rawValue
        request.setValue("Bearer \(refreshToken)", forHTTPHeaderField: "Authorization")

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            // Transport failure: offline, timed out, connection lost. Transient by
            // definition — `AppError` maps the URLError codes.
            throw AppError(error)
        }

        guard let http = response as? HTTPURLResponse else {
            throw AppError(kind: .unknown, diagnostic: "Non-HTTP response from /auth/refresh")
        }

        guard (200..<300).contains(http.statusCode) else {
            throw refusal(from: data, status: http.statusCode)
        }

        do {
            return try JSONDecoder.api.decode(APIEnvelope<TokenPairDTO>.self, from: data).data.pair
        } catch {
            // A refresh that succeeded but did not decode is *not* a dead session. Treating
            // it as one would sign the user out over a shape change.
            throw AppError(kind: .decoding, diagnostic: "Failed to decode refreshed tokens: \(error)")
        }
    }

    private func refusal(from data: Data, status: Int) -> AppError {
        let message = try? JSONDecoder.api.decode(APIErrorEnvelope.self, from: data).error?.messages.first

        return switch status {
        case 401:
            AppError(kind: .unauthenticated, diagnostic: message)
        case 429:
            // `/auth/refresh` is capped at 20/min. Transient, and the caller may retry.
            AppError(kind: .throttled(retryAfter: nil), diagnostic: message)
        default:
            AppError(kind: .server(status: status), diagnostic: message)
        }
    }
}
