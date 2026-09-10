import Foundation

/// The single error type surfaced above the networking layer.
///
/// Feature code never sees `URLError`, `DecodingError`, or a vendor SDK's error type.
/// Mapping happens once, at the boundary, so presentation has one thing to switch on.
public struct AppError: Error, Equatable, Sendable {
    public enum Kind: Equatable, Sendable {
        /// The request reached the server and came back unhappy.
        case server(status: Int)
        /// The response did not match the contract in docs/API.md.
        case decoding
        /// Credentials are gone for good — the session is over.
        case unauthenticated
        /// No usable connection. Distinct from `server` because it is retryable.
        case offline
        /// Rate limited. `retryAfter` is seconds, when the server said.
        case throttled(retryAfter: Int?)
        case unknown
    }

    public let kind: Kind
    /// Shown to the user. Must already be localized.
    public let message: String
    /// Kept for logs only — never displayed.
    public let diagnostic: String?

    public init(kind: Kind, message: String, diagnostic: String? = nil) {
        self.kind = kind
        self.message = message
        self.diagnostic = diagnostic
    }

    /// Last-resort mapping for errors that escaped a more specific boundary.
    public init(_ error: some Error) {
        if let appError = error as? AppError {
            self = appError
            return
        }
        if let urlError = error as? URLError {
            let offline: Set<URLError.Code> = [
                .notConnectedToInternet, .networkConnectionLost, .dataNotAllowed, .timedOut,
            ]
            self.init(
                kind: offline.contains(urlError.code) ? .offline : .unknown,
                message: String(localized: "Something went wrong. Please try again."),
                diagnostic: urlError.localizedDescription
            )
            return
        }
        if error is DecodingError {
            self.init(
                kind: .decoding,
                message: String(localized: "Something went wrong. Please try again."),
                diagnostic: String(describing: error)
            )
            return
        }
        self.init(
            kind: .unknown,
            message: String(localized: "Something went wrong. Please try again."),
            diagnostic: String(describing: error)
        )
    }

    /// Whether retrying the same request could plausibly succeed.
    public var isRetryable: Bool {
        switch kind {
        case .offline, .throttled: true
        case .server(let status): status >= 500
        case .decoding, .unauthenticated, .unknown: false
        }
    }
}
