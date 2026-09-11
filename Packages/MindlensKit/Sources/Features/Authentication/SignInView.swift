import AuthenticationServices
import Core
import DesignSystem
import SwiftUI

/// The signed-out screen.
///
/// Stage 1 shows sign-in on its own. The existing product reaches the same buttons at the end
/// of an onboarding survey ("One last step"), and that survey is Stage 4 — so the copy here
/// is the product's opening line rather than a promise about answers the user has not given.
public struct SignInView: View {
    @Environment(\.colorScheme) private var colorScheme
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    /// The control height, scaled with Dynamic Type. A fixed 50 collapses
    /// `SignInWithAppleButton` at accessibility sizes — it loses its background entirely and
    /// falls under the 44pt tap-target floor.
    @ScaledMetric(relativeTo: .body) private var controlHeight: CGFloat = 50

    /// The same height, clamped to the range Apple documents for this button: **30–64pt**.
    ///
    /// Outside it the control misbehaves rather than complains — stretched past 64 it draws its
    /// label with no capsule behind it, which at an accessibility text size is most of the
    /// screen's width of unstyled text where a button should be. 44 is the floor because that
    /// is the tap target, so the usable range here is 44–64.
    private var appleButtonHeight: CGFloat { min(max(controlHeight, 44), 64) }

    /// The scroll view's own height, measured. See `body`.
    @State private var availableHeight: CGFloat = 0

    private let model: SessionModel

    public init(model: SessionModel) {
        self.model = model
    }

    public var body: some View {
        // The whole screen is one scroll view, buttons included. Pinning them to the bottom
        // with `safeAreaInset` reads better at default sizes but leaves the panel unable to
        // scroll — and at accessibility sizes the panel alone is taller than the screen, so it
        // compresses until the buttons are unusable.
        //
        // `minHeight: proxy.size.height` buys back the default-size layout: the stack fills the
        // screen, so the `Spacer` genuinely pushes the buttons down, and content taller than
        // that scrolls instead of clipping.
        ScrollView {
            VStack(alignment: .leading, spacing: Spacing.snug) {
                Text("See what’s behind", bundle: .module)
                    .font(.largeTitle.weight(.bold))
                Text("your good and bad days", bundle: .module)
                    .font(.title3)
                    .foregroundStyle(.secondary)

                Spacer(minLength: Spacing.section)

                signInPanel
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, Spacing.loose)
            .padding(.vertical, Spacing.section)
            // *At least* as tall as the scroll view: the `Spacer` becomes real at default sizes
            // and pushes the buttons down, and anything taller scrolls instead of compressing.
            //
            // The minimum is the point. `containerRelativeFrame` reads better but sets an
            // **exact** height, so at accessibility sizes the legal text was truncated with an
            // ellipsis rather than being allowed to overflow into scroll. A `GeometryReader`
            // wrapper gives the right semantics but has no intrinsic size and takes every point
            // offered; `onGeometryChange` measures without laying anything out.
            .frame(minHeight: availableHeight, alignment: .top)
        }
        .scrollBounceBehavior(.basedOnSize)
        .onGeometryChange(for: CGFloat.self) {
            $0.size.height
        } action: {
            availableHeight = $0
        }
    }

    // MARK: - Sign-in panel

    private var signInPanel: some View {
        VStack(spacing: Spacing.regular) {
            if let error = model.error {
                Text(error.signInMessage)
                    .font(.footnote)
                    .foregroundStyle(.red)
                    .multilineTextAlignment(.center)
            }

            appleButton
            googleButton
            legalNotice
        }
        .frame(maxWidth: .infinity)
    }

    /// The system button, so its label, localization, shape and accessibility come from
    /// Apple rather than from us. Its style is the one thing we choose, and it has to follow
    /// the appearance: black on light, white on dark.
    private var appleButton: some View {
        SignInWithAppleButton(.continue) { request in
            request.requestedScopes = [.email, .fullName]
            request.nonce = model.appleRequestNonce()
        } onCompletion: { result in
            handleAppleCompletion(result)
        }
        .signInWithAppleButtonStyle(colorScheme == .dark ? .white : .black)
        // An exact height, not a minimum: given only a minimum this becomes the stack's
        // flexible child and swallows every point of slack on the screen. Given no width it
        // sizes to its label and drops its capsule. Both dimensions have to be said.
        .frame(height: appleButtonHeight)
        .frame(maxWidth: .infinity)
        // Rebuilt when the text size changes. It is a UIKit view underneath, and on a live
        // trait change it keeps its old capsule geometry — the height updates, the background
        // does not redraw, and it ends up rendering as bare text. Changing the identity forces
        // a fresh one. Only reachable by changing the setting while the app is running, which
        // is exactly what someone adjusting accessibility settings does.
        .id(dynamicTypeSize)
        .disabled(model.isSigningIn)
        .overlay {
            if model.pending == .apple {
                ProgressView().tint(colorScheme == .dark ? .black : .white)
            }
        }
    }

    /// Not a system component — there is no Google equivalent of `SignInWithAppleButton`, so
    /// this is the exception `docs/DESIGN.md` asks us to note. It stays a plain bordered
    /// button until Google's mark is added as an asset; an unbranded button is better than a
    /// wrong one.
    private var googleButton: some View {
        Button {
            Task { await model.signInWithGoogle() }
        } label: {
            // The label stays in the tree and is merely invisible. Swapping it out for a bare
            // `ProgressView` leaves the button with no accessibility name at all — VoiceOver
            // announces an unnamed busy button — and lets its width jump as the label goes.
            Text("Continue with Google", bundle: .module)
                .opacity(model.pending == .google ? 0 : 1)
                .frame(maxWidth: .infinity, minHeight: controlHeight)
                .overlay { if model.pending == .google { ProgressView() } }
        }
        .buttonStyle(.bordered)
        .buttonBorderShape(.roundedRectangle(radius: Radius.control))
        .disabled(model.isSigningIn)
        .accessibilityValue(
            model.pending == .google ? Text("Signing in", bundle: .module) : Text(verbatim: "")
        )
    }

    /// Links live inline in the string as markdown, which `Text` renders and opens for us —
    /// and which keeps a URL out of Swift, where it would need a force-unwrap.
    private var legalNotice: some View {
        // The key has to stay one literal — splitting it changes the key, and concatenation
        // cannot be localized at all.
        Text(
            // swiftlint:disable:next line_length
            "We respect your privacy. By continuing, you agree to our [Terms of Use](https://trymindlens.com/terms) and [Privacy Policy](https://trymindlens.com/privacy).",
            bundle: .module
        )
        .font(.caption)
        .foregroundStyle(.secondary)
        .multilineTextAlignment(.center)
    }

    // MARK: - Apple completion

    /// Unwrapping the credential is the only work the view does, and it is here rather than in
    /// the model because `ASAuthorizationAppleIDCredential` cannot be constructed in a test.
    private func handleAppleCompletion(_ result: Result<ASAuthorization, any Error>) {
        switch result {
        case .success(let authorization):
            guard
                let credential = authorization.credential as? ASAuthorizationAppleIDCredential,
                let tokenData = credential.identityToken,
                let identityToken = String(data: tokenData, encoding: .utf8)
            else {
                // Apple reported success and handed back something unusable. Reporting that as a
                // cancellation is the silent failure the branch below exists to avoid: the sheet
                // closes, the screen returns to idle, and nothing tells the user or us.
                model.signInFailed(
                    AppError(kind: .unknown, diagnostic: "Apple returned no usable identity token")
                )
                return
            }

            let code = credential.authorizationCode.flatMap { String(data: $0, encoding: .utf8) }

            Task {
                await model.signInWithApple(
                    identityToken: identityToken,
                    authorizationCode: code,
                    email: credential.email
                )
            }

        case .failure(let error):
            // `.canceled` is the user closing the sheet — the most common outcome of tapping a
            // sign-in button, and never something to report. Anything else genuinely failed.
            // The classification happens here because this is where the framework type is; the
            // model takes it as one of two plain outcomes.
            if (error as? ASAuthorizationError)?.code == .canceled {
                model.signInWasCancelled()
            } else {
                model.signInFailed(error)
            }
        }
    }
}
