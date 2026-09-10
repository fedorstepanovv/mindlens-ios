import Foundation

/// A product analytics event.
///
/// Properties are structural only. Mood values, note text, survey answers and health
/// data never leave the device through this path — a rule the existing product enforces
/// with a dedicated test, and one worth keeping.
public struct AnalyticsEvent: Sendable, Equatable {
    public let name: String
    public let properties: [String: String]

    public init(_ name: String, properties: [String: String] = [:]) {
        self.name = name
        self.properties = properties
    }
}

public protocol AnalyticsRecording: Sendable {
    func record(_ event: AnalyticsEvent)
}

/// Default binding. Real SDKs are wired at the app target's composition root — see ADR 0005.
public struct NoopAnalytics: AnalyticsRecording {
    public init() {}
    public func record(_ event: AnalyticsEvent) {}
}

public extension AnalyticsRecording where Self == NoopAnalytics {
    static var noop: NoopAnalytics { NoopAnalytics() }
}
