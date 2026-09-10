import Foundation

/// The device identity the sign-in request carries.
///
/// Declared in `Core` for the same reason `TokenStorage` is: `Persistence` implements it,
/// and having persistence depend on networking would point the arrow the wrong way.
public protocol DeviceIdentifying: Sendable {
    /// A stable, install-persistent GUID.
    ///
    /// The server caps sessions at **5 devices per user** and LRU-evicts, and a re-login
    /// from the same GUID *replaces* that device's session rather than consuming another
    /// slot. A GUID that changes per launch therefore silently evicts the user's other
    /// devices — so this must survive sign-out and app reinstall. See `docs/API.md`.
    func guid() async -> String

    /// The hardware identifier, e.g. `iPhone17,1`.
    var model: String { get }
}

/// `utsname.machine`, with the two cases where that string is unusable handled.
public struct SystemDeviceModel: Sendable {
    public init() {}

    public var value: String {
        Self.resolve(
            simulated: ProcessInfo.processInfo.environment["SIMULATOR_MODEL_IDENTIFIER"],
            machine: Self.machine
        )
    }

    /// The API validates `deviceModel` as **6–36 characters**, and `utsname.machine` violates
    /// the lower bound on a simulator, where it reports the *host* architecture: `arm64`, five
    /// characters, a 400 on every sign-in attempt on every developer's machine. The simulator
    /// publishes the device it is impersonating in its environment, which is both longer and
    /// more truthful.
    ///
    /// Split out from `value` so the three cases are testable without a simulator.
    public static func resolve(simulated: String?, machine: String?) -> String {
        if let simulated, simulated.count >= 6, simulated.count <= 36 { return simulated }
        if let machine, machine.count >= 6, machine.count <= 36 { return machine }
        return fallback
    }

    /// `utsname.machine`, or `nil` if `uname` fails.
    static var machine: String? {
        var system = utsname()
        guard uname(&system) == 0 else { return nil }

        return withUnsafeBytes(of: &system.machine) { raw in
            String(bytes: raw.prefix(while: { $0 != 0 }), encoding: .utf8)
        }
    }

    /// Long enough to pass validation and obviously not a real model, so a support ticket
    /// reading `unknown-device` points at this line rather than at a mystery.
    public static let fallback = "unknown-device"
}
