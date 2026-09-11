import Core
import Foundation
import Testing

@Suite("Sign-in nonce")
struct SignInNonceTests {

    @Test("the hash is what Apple sees, and it is not the raw value")
    func hashIsNotRaw() {
        let nonce = SignInNonce.generate()

        #expect(nonce.hashed != nonce.raw)
        // SHA-256, hex-encoded.
        #expect(nonce.hashed.count == 64)
        // Bound first: `#expect` decomposes a call it can see, and the macro's rewrite of
        // `allSatisfy` loses the fact that a key-path argument cannot throw.
        let isHex = nonce.hashed.allSatisfy(\.isHexDigit)
        #expect(isHex)
    }

    @Test("hashing is stable, so the value Firebase re-hashes matches the one Apple echoed")
    func hashIsStable() {
        let nonce = SignInNonce(raw: "abc")

        #expect(nonce.hashed == SignInNonce(raw: "abc").hashed)
        #expect(
            nonce.hashed == "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
        )
    }

    @Test("every nonce is different — a reused one is a replayable authorization")
    func noncesAreUnique() {
        let generated = Set((0..<50).map { _ in SignInNonce.generate().raw })

        #expect(generated.count == 50)
    }
}

@Suite("Device model")
struct DeviceModelTests {

    /// The failure this guards is a 400 on every sign-in in the simulator: `utsname.machine`
    /// reports the host architecture there, and `arm64` is one character under the API's
    /// six-character floor.
    @Test(
        "the reported model always satisfies the API's 6–36 character rule",
        arguments: [
            (simulated: "iPhone17,1", machine: "arm64", expected: "iPhone17,1"),
            (simulated: nil, machine: "iPhone17,1", expected: "iPhone17,1"),
            (simulated: nil, machine: "arm64", expected: SystemDeviceModel.fallback),
            (simulated: "x86", machine: "arm64", expected: SystemDeviceModel.fallback),
            (simulated: nil, machine: nil, expected: SystemDeviceModel.fallback),
            (
                simulated: nil, machine: String(repeating: "M", count: 40),
                expected: SystemDeviceModel.fallback
            ),
        ]
    )
    func modelResolution(simulated: String?, machine: String?, expected: String) {
        let resolved = SystemDeviceModel.resolve(simulated: simulated, machine: machine)

        #expect(resolved == expected)
        #expect(resolved.count >= 6)
        #expect(resolved.count <= 36)
    }

    @Test("the real device reports something the API will accept")
    func liveModelIsValid() {
        let value = SystemDeviceModel().value

        #expect(value.count >= 6)
        #expect(value.count <= 36)
    }
}
