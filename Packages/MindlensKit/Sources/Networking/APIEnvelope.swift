import Foundation

/// Every successful response from the Mindlens API is wrapped like this.
/// See docs/API.md — unwrapping happens here so no call site ever sees `data`.
public struct APIEnvelope<Payload: Decodable & Sendable>: Decodable, Sendable {
    public let data: Payload
    public let success: Bool
}

/// The error half of the envelope.
public struct APIErrorEnvelope: Decodable, Sendable {
    public let success: Bool
    public let error: APIErrorBody?
}

/// NestJS returns `message` as either a string or an array of strings — class-validator
/// produces arrays, hand-thrown exceptions produce strings. Both are normalised here.
public struct APIErrorBody: Decodable, Sendable {
    public let statusCode: Int?
    public let messages: [String]
    public let reason: String?

    private enum CodingKeys: String, CodingKey {
        case statusCode, message, error
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        statusCode = try container.decodeIfPresent(Int.self, forKey: .statusCode)
        reason = try container.decodeIfPresent(String.self, forKey: .error)

        if let single = try? container.decode(String.self, forKey: .message) {
            messages = [single]
        } else if let many = try? container.decode([String].self, forKey: .message) {
            messages = many
        } else {
            messages = []
        }
    }
}
