import Foundation
import Synchronization

/// Intercepts requests so tests never touch the network.
///
/// This is the mechanism behind `docs/TESTING.md`'s "no test touches the network, ever."
///
/// Handlers are registered **per session**, not globally. Swift Testing runs suites in
/// parallel by default, so a single shared handler is a race: tests overwrite each
/// other's stub and fail in ways that look like product bugs. Each session tags its
/// requests with an id and the protocol looks up that session's handler.
public final class StubURLProtocol: URLProtocol {
    public typealias Handler = @Sendable (URLRequest) throws -> (HTTPURLResponse, Data)

    private static let header = "X-Stub-Session"
    private static let handlers = Mutex<[String: Handler]>([:])

    /// A session wired to this protocol, with `handler` installed for it alone.
    public static func session(_ handler: @escaping Handler) -> URLSession {
        let id = UUID().uuidString
        handlers.withLock { $0[id] = handler }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [StubURLProtocol.self]
        configuration.httpAdditionalHeaders = [header: id]
        return URLSession(configuration: configuration)
    }

    /// Responds with a fixed status and body, optionally counting requests.
    public static func session(status: Int, body: Data = Data(), counter: Counter? = nil) -> URLSession {
        session { _ in
            counter?.increment()
            return (Self.response(status), body)
        }
    }

    // The origin every stubbed request is addressed to. Tests share this rather than
    // each building their own, so the one force-unwrap the codebase tolerates lives at
    // exactly one site: a compile-time constant in test-only code, where a nil would
    // mean the harness itself is broken.
    //
    // swift-format flags this; SwiftLint does not. Both enforce the ban independently
    // and neither honours the other's directive.
    // swift-format-ignore: NeverForceUnwrap
    public static let url = URL(string: "https://example.test")!

    // This one both linters flag, so it carries a directive for each.
    // swift-format-ignore: NeverForceUnwrap
    public static func response(_ status: Int) -> HTTPURLResponse {
        // swiftlint:disable:next force_unwrapping
        HTTPURLResponse(url: url, statusCode: status, httpVersion: nil, headerFields: nil)!
    }

    override public static func canInit(with request: URLRequest) -> Bool {
        request.value(forHTTPHeaderField: header) != nil
    }

    override public static func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override public func startLoading() {
        guard let id = request.value(forHTTPHeaderField: Self.header),
            let handler = Self.handlers.withLock({ $0[id] })
        else {
            client?.urlProtocol(self, didFailWithError: URLError(.unsupportedURL))
            return
        }

        do {
            let (response, data) = try handler(request)
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: data)
            client?.urlProtocolDidFinishLoading(self)
        } catch {
            client?.urlProtocol(self, didFailWithError: error)
        }
    }

    override public func stopLoading() {}
}

/// A thread-safe tally, for asserting how many times something happened.
public final class Counter: Sendable {
    private let storage = Mutex(0)

    public init() {}
    public func increment() { storage.withLock { $0 += 1 } }
    public var value: Int { storage.withLock { $0 } }
}
