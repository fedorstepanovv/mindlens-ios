import Foundation
import Models
import TestSupport
import Testing

@testable import Networking

/// Decoding tests against **real captured responses**. With no OpenAPI spec on the
/// server, these are the only thing that catches a contract change before users do.
@Suite("Envelope decoding")
struct EnvelopeDecodingTests {

    @Test("decodes a success envelope")
    func decodesSuccessEnvelope() throws {
        let data = try Fixture.data("mood_create_200")
        let envelope = try JSONDecoder.api.decode(APIEnvelope<MoodRecordDTO>.self, from: data)

        #expect(envelope.success)
        #expect(envelope.data.mood == 3)
    }

    @Test("decodes a 401 whose error body carries no reason field")
    func decodesUnauthorized() throws {
        let data = try Fixture.data("error_unauthorized_401")
        let envelope = try JSONDecoder.api.decode(APIErrorEnvelope.self, from: data)

        #expect(envelope.success == false)
        #expect(envelope.error?.statusCode == 401)
        #expect(envelope.error?.messages == ["Unauthorized"])
        // The 401 body omits `error` entirely, unlike a 400. Decoding must tolerate that.
        #expect(envelope.error?.reason == nil)
    }

    @Test("decodes a validation error whose message is an array")
    func decodesValidationError() throws {
        let data = try Fixture.data("error_validation_400")
        let envelope = try JSONDecoder.api.decode(APIErrorEnvelope.self, from: data)

        #expect(envelope.error?.statusCode == 400)
        #expect(envelope.error?.reason == "Bad Request")
        #expect((envelope.error?.messages.count ?? 0) > 1)
    }

    @Test("decodes the server rejecting an unknown request field")
    func decodesUnknownFieldRejection() throws {
        // The server runs `whitelist` + `forbidNonWhitelisted`: an extra key is a 400,
        // not a silent drop. Encoding must stay exact — see docs/API.md.
        let data = try Fixture.data("error_unknown_field_400")
        let envelope = try JSONDecoder.api.decode(APIErrorEnvelope.self, from: data)

        #expect(envelope.error?.messages == ["property surprise should not exist"])
    }
}
