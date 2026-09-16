import Foundation

/// The single error type surfaced above the networking layer.
///
/// Deliberately carries **no user-facing copy**. Words are a presentation concern and
/// live in `DesignSystem`, where the String Catalog is — putting them here would mean
/// localizing from a target that has no business knowing how errors are shown.
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
    /// Kept for logs only — never displayed, never localized.
    public let diagnostic: String?

    public init(kind: Kind, diagnostic: String? = nil) {
        self.kind = kind
        self.diagnostic = diagnostic
    }

    /// Last-resort mapping for errors that escaped a more specific boundary.
    public init(_ error: some Error) {
        if let appError = error as? AppError {
            self = appError
            return
        }
        if let urlError = error as? URLError {
            // A captive portal, a dead DNS server and a failed TLS handshake are all "you are
            // not reaching us right now" — retryable, and worth saying so. Left in `.unknown`
            // they report as non-retryable and read as "something went wrong", which on the
            // launch path is the least useful thing we could say.
            let offline: Set<URLError.Code> = [
                .notConnectedToInternet, .networkConnectionLost, .dataNotAllowed, .timedOut,
                .cannotFindHost, .cannotConnectToHost, .dnsLookupFailed, .secureConnectionFailed,
                .internationalRoamingOff, .callIsActive,
            ]
            self.init(
                kind: offline.contains(urlError.code) ? .offline : .unknown,
                diagnostic: urlError.localizedDescription
            )
            return
        }
        if error is DecodingError {
            self.init(kind: .decoding, diagnostic: String(describing: error))
            return
        }
        self.init(kind: .unknown, diagnostic: String(describing: error))
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
