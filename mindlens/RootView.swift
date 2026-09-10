import Authentication
import DesignSystem
import Models
import SwiftUI

/// The session gate.
///
/// A `switch`, not a router redirect: the signed-in tree is never *built* while signed out, so
/// an authenticated view cannot be reached by accident. This is the structural difference from
/// a global redirect callback, and it lives at the scene root because that is the only place
/// that knows the whole graph.
struct RootView: View {
    let session: SessionModel

    var body: some View {
        content
            .task { await session.restore() }
    }

    @ViewBuilder
    private var content: some View {
        switch session.state {
        case .restoring:
            if session.needsRestoreRetry {
                restoreFailed
            } else {
                ProgressView()
                    .controlSize(.large)
                    .accessibilityLabel("Restoring your session")
            }

        case .signedOut:
            SignInView(model: session)

        case .onboarding(let user):
            // Stage 4 builds the survey this leads to; the gate that routes here is Stage 1's.
            placeholder(
                title: "Onboarding",
                message: "Signed in as \(user.email). Onboarding is not built yet.",
                symbol: "sparkles"
            )

        case .signedIn(let user):
            // Stage 2 builds the dashboard.
            placeholder(
                title: "Signed in",
                message: "Signed in as \(user.email). The dashboard is not built yet.",
                symbol: "checkmark.circle"
            )
        }
    }

    /// Tokens are still valid and the network is not. Showing the sign-in screen here would
    /// look identical to being logged out, and a user with no connection cannot sign back in.
    private var restoreFailed: some View {
        ContentUnavailableView {
            Label("Can't reach Mindlens", systemImage: "wifi.exclamationmark")
        } description: {
            Text(session.error?.displayMessage ?? "")
        } actions: {
            Button("Try Again") {
                Task { await session.restore() }
            }
            .buttonStyle(.borderedProminent)
        }
    }

    private func placeholder(title: String, message: String, symbol: String) -> some View {
        ContentUnavailableView {
            Label(title, systemImage: symbol)
        } description: {
            Text(message)
        } actions: {
            Button("Sign Out") {
                Task { await session.signOut() }
            }
        }
    }
}
