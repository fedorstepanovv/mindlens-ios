import Foundation

/// A request with its response type attached, so a call site cannot decode the wrong thing.
public struct Endpoint<Response: Decodable & Sendable>: Sendable {
    public var method: HTTPMethod
    public var path: String
    public var query: [URLQueryItem]
    public var body: Data?
    public var requiresAuth: Bool

    public init(
        method: HTTPMethod,
        path: String,
        query: [URLQueryItem] = [],
        body: Data? = nil,
        requiresAuth: Bool = true
    ) {
        self.method = method
        self.path = path
        self.query = query
        self.body = body
        self.requiresAuth = requiresAuth
    }
}

public extension Endpoint {
    static func get(_ path: String, query: [URLQueryItem] = [], requiresAuth: Bool = true) -> Self {
        Endpoint(method: .get, path: path, query: query, requiresAuth: requiresAuth)
    }

    static func delete(_ path: String, requiresAuth: Bool = true) -> Self {
        Endpoint(method: .delete, path: path, requiresAuth: requiresAuth)
    }

    static func post(
        _ path: String,
        body: some Encodable & Sendable,
        requiresAuth: Bool = true
    ) throws -> Self {
        Endpoint(
            method: .post,
            path: path,
            body: try JSONEncoder.api.encode(body),
            requiresAuth: requiresAuth
        )
    }

    static func patch(
        _ path: String,
        body: some Encodable & Sendable,
        requiresAuth: Bool = true
    ) throws -> Self {
        Endpoint(
            method: .patch,
            path: path,
            body: try JSONEncoder.api.encode(body),
            requiresAuth: requiresAuth
        )
    }
}

public extension JSONEncoder {
    /// The API rejects unknown fields outright, so encoding must stay exact.
    static var api: JSONEncoder {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        return encoder
    }
}

public extension JSONDecoder {
    static var api: JSONDecoder {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return decoder
    }
}
