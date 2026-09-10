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
        encoder.dateEncodingStrategy = .custom { date, encoder in
            var container = encoder.singleValueContainer()
            try container.encode(Date.ISO8601FormatStyle(includingFractionalSeconds: true).format(date))
        }
        return encoder
    }
}

public extension JSONDecoder {
    static var api: JSONDecoder {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom(Self.decodeTimestamp)
        return decoder
    }

    /// Timestamps from this API carry fractional seconds (`…:41.512Z`).
    ///
    /// `.iso8601` happens to accept them on the current toolchain but **discards the
    /// milliseconds**, and its documented option set (`.withInternetDateTime`) does not
    /// include fractional seconds at all. Parsing both forms explicitly means the
    /// behaviour is ours rather than a Foundation implementation detail.
    ///
    /// Plain days and wall-clock times use `CalendarDate` and `TimeOfDay` instead — one
    /// global strategy cannot serve three formats.
    private static let decodeTimestamp: @Sendable (any Decoder) throws -> Date = { decoder in
        let raw = try decoder.singleValueContainer().decode(String.self)

        if let parsed = try? Date.ISO8601FormatStyle(includingFractionalSeconds: true).parse(raw) {
            return parsed
        }
        if let parsed = try? Date.ISO8601FormatStyle(includingFractionalSeconds: false).parse(raw) {
            return parsed
        }
        throw DecodingError.dataCorrupted(
            .init(codingPath: decoder.codingPath, debugDescription: "Not an ISO-8601 timestamp: \(raw)")
        )
    }
}
