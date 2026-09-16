import XCTest

/// The app launches and reaches its first screen.
///
/// This exists because a build succeeding proved nothing: the app once compiled cleanly and
/// then hit a `preconditionFailure` on the first line of the composition root, because a
/// custom `INFOPLIST_KEY_` had been silently dropped from the bundle. Every unit test passed.
///
/// Deliberately tiny — `docs/TESTING.md` keeps UI tests small because they are flaky in
/// proportion to their number. It asserts the one thing no other test can: that the thing
/// launches at all.
final class LaunchSmokeTests: XCTestCase {

    override func setUp() {
        continueAfterFailure = false
    }

    @MainActor
    func testLaunchesToSignIn() {
        let app = XCUIApplication()
        app.launch()

        XCTAssertTrue(
            app.buttons["Continue with Google"].waitForExistence(timeout: 60),
            "The app did not reach the sign-in screen. With no stored session the gate should "
                + "resolve from .restoring to .signedOut and show SignInView."
        )
        XCTAssertEqual(app.state, .runningForeground, "The app is no longer in the foreground.")
    }

    /// The same launch at the largest accessibility text size.
    ///
    /// `SignInWithAppleButton` is a UIKit control with opinions about its frame: stretched past
    /// the 30–64pt Apple documents, it quietly stops drawing its capsule and renders as bare
    /// text. That is invisible to every other kind of test we have.
    @MainActor
    func testLaunchesAtLargestTextSize() {
        let app = XCUIApplication()
        app.launchArguments += [
            "-UIPreferredContentSizeCategoryName",
            "UICTContentSizeCategoryAccessibilityExtraExtraExtraLarge",
        ]
        app.launch()

        let apple = app.buttons["Continue with Apple"]
        XCTAssertTrue(apple.waitForExistence(timeout: 60), "Sign in with Apple is missing.")
        XCTAssertTrue(app.buttons["Continue with Google"].exists, "Continue with Google is missing.")

        // Both controls must still be tappable targets, not collapsed remnants of themselves.
        XCTAssertGreaterThanOrEqual(
            apple.frame.height, 44,
            "Sign in with Apple fell under the 44pt tap target at an accessibility text size."
        )
    }
}
