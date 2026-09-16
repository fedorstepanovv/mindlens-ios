import Core
import Foundation

/// Returned by endpoints that answer 204.
public struct NoContent: Decodable, Sendable {
    public init() {}
}

/// Stateless: all three properties are `let`, so there is nothing to isolate.
/// `Sendable` rather than an `actor` — an actor here would add an executor hop per
/// request and protect nothing. Isolation lives in `TokenRefresher`, which does have
/// mutable state.
public final class APIClient: Sendable {
    private let baseURL: URL
    private let session: URLSession
    private let refresher: TokenRefresher

    public init(baseURL: URL, session: URLSession = .shared, refresher: TokenRefresher) {
        self.baseURL = baseURL
        self.session = session
        self.refresher = refresher
    }

    public func send<Response>(_ endpoint: Endpoint<Response>) async throws -> Response {
        var credentials: TokenRefresher.Credentials?

        if endpoint.requiresAuth {
            credentials = try await refresher.credentials()
            guard credentials != nil else {
                throw AppError(kind: .unauthenticated)
            }
        }

        let (data, response) = try await perform(endpoint, using: credentials?.tokens)

        // One retry, and only for a genuine 401. The refresher decides whether that means
        // "token was stale" or "session is over" — see TokenRefresher.
        if response.statusCode == 401, endpoint.requiresAuth, endpoint.retriesAfterRefresh,
            let credentials
        {
            let fresh = try await refresher.refreshed(after: credentials.generation)
            let (retryData, retryResponse) = try await perform(endpoint, using: fresh.tokens)
            return try decode(retryData, response: retryResponse)
        }

        return try decode(data, response: response)
    }

    // MARK: - Private

    private func perform(
        _ endpoint: Endpoint<some Decodable & Sendable>,
        using tokens: TokenPair?
    ) async throws -> (Data, HTTPURLResponse) {
        guard
            var components = URLComponents(
                url: baseURL.appending(path: endpoint.path),
                resolvingAgainstBaseURL: false
            )
        else {
            throw AppError(kind: .unknown, diagnostic: "Bad URL for \(endpoint.path)")
        }
        if !endpoint.query.isEmpty { components.queryItems = endpoint.query }

        guard let url = components.url else {
            throw AppError(kind: .unknown, diagnostic: "Bad query for \(endpoint.path)")
        }

        var request = URLRequest(url: url)
        request.httpMethod = endpoint.method.rawValue
        request.httpBody = endpoint.body
        if endpoint.body != nil {
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        if let tokens {
            request.setValue("Bearer \(tokens.access)", forHTTPHeaderField: "Authorization")
        }

        do {
            let (data, response) = try await session.data(for: request)
            guard let http = response as? HTTPURLResponse else {
                throw AppError(kind: .unknown)
            }
            return (data, http)
        } catch let error as AppError {
            throw error
        } catch {
            throw AppError(error)
        }
    }

    private func decode<Response: Decodable & Sendable>(
        _ data: Data,
        response: HTTPURLResponse
    ) throws -> Response {
        guard (200..<300).contains(response.statusCode) else {
            throw failure(from: data, status: response.statusCode)
        }

        if data.isEmpty || response.statusCode == 204, let empty = NoContent() as? Response {
            return empty
        }

        do {
            return try JSONDecoder.api.decode(APIEnvelope<Response>.self, from: data).data
        } catch {
            // A decoding failure here means the response no longer matches docs/API.md.
            throw AppError(
                kind: .decoding, diagnostic: "Failed to decode \(Response.self): \(error)"
            )
        }
    }

    private func failure(from data: Data, status: Int) -> AppError {
        let body = try? JSONDecoder.api.decode(APIErrorEnvelope.self, from: data)
        let serverMessage = body?.error?.messages.first

        return switch status {
        case 401:
            AppError(kind: .unauthenticated, diagnostic: serverMessage)
        case 429:
            AppError(kind: .throttled(retryAfter: nil), diagnostic: serverMessage)
        default:
            AppError(kind: .server(status: status), diagnostic: serverMessage)
        }
    }
}
