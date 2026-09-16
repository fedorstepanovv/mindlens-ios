import Foundation

/// Build-time configuration, read once.
enum AppConfiguration {
    /// The API origin, forwarded from `API_BASE_URL` in `Config/Base.xcconfig` through the
    /// generated Info.plist. Deliberately **not** duplicated as a Swift literal: a second
    /// copy is a second answer, and the xcconfig is the one `Tools/check-build-settings.sh`
    /// asserts.
    ///
    /// Missing means the build is misconfigured, not that the user did something — and there
    /// is nothing this app can do without an API. `Tools/check-build-settings.sh` fails the
    /// PR gate on exactly this, so it cannot reach anyone but us.
    static var apiBaseURL: URL {
        guard let raw = Bundle.main.object(forInfoDictionaryKey: "MindlensAPIBaseURL") as? String,
            let url = URL(string: raw)
        else {
            preconditionFailure(
                "MindlensAPIBaseURL is missing or unreadable. Check API_BASE_URL in Config/Base.xcconfig."
            )
        }
        return url
    }
}
