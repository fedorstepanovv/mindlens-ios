import Foundation
import os

extension Logger {
    /// One subsystem for the whole app — the bundle identifier — so a single Console.app
    /// filter catches every target. The category names the concern: `session`, `network`, …
    ///
    /// `docs/ARCHITECTURE.md` § Error handling: no error is swallowed silently — if it cannot
    /// be shown, it is logged. `AppError.diagnostic` exists for these calls and nothing else;
    /// a server message or a vendor error code that only ever reaches the generic banner is a
    /// failure nobody can diagnose.
    public init(category: String) {
        self.init(subsystem: Bundle.main.bundleIdentifier ?? "mindlens", category: category)
    }
}
