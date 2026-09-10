import Core
import DesignSystem
import SwiftUI

/// `@MainActor` for the same reason `AppErrorPresentation` is: SPM's generated
/// `Bundle.module` accessor is main-actor isolated in a target that defaults to it, and an
/// extension on a type from another module does not pick up this target's default isolation.
@MainActor
extension AppError {
    /// Sign-in copy, in this feature's catalog rather than the shared one.
    ///
    /// `displayMessage` in `DesignSystem` is right for every screen that shows an error in
    /// passing. Sign-in is the exception: a 422 here has one specific cause and one specific
    /// remedy, and "Something went wrong" leaves the user tapping the same button forever.
    var signInMessage: String {
        switch kind {
        case .server(let status) where status == 422:
            // Short enough to need no lint waiver — the formatter reflows this literal, which
            // moves a `disable:next` off the line it was meant for.
            String(
                localized: "We couldn't verify that account. Try again, and choose Share My Email.",
                bundle: .module
            )
        case .server(let status) where status == 409:
            String(
                localized: "That email is already signed up with a different provider.",
                bundle: .module
            )
        default:
            displayMessage
        }
    }
}
