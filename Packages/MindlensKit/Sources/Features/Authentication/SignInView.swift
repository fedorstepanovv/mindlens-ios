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

    private let model: SessionModel

    public init(model: SessionModel) {
        self.model = model
    }

    public var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Spacing.snug) {
                Text("See what's behind", bundle: .module)
                    .font(.largeTitle.weight(.bold))
                Text("your good and bad days", bundle: .module)
                    .font(.title3)
                    .foregroundStyle(.secondary)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, Spacing.loose)
            .padding(.top, Spacing.section)
        }
        // Content this short only scrolls at large Dynamic Type sizes, which is exactly when
        // it must.
        .scrollBounceBehavior(.basedOnSize)
        .safeAreaInset(edge: .bottom) { signInPanel }
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
        .padding(.horizontal, Spacing.loose)
        .padding(.bottom, Spacing.snug)
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
        .frame(height: 50)
        .disabled(model.isSigningIn)
        .opacity(model.pending == .google ? 0.4 : 1)
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
            Group {
                if model.pending == .google {
                    ProgressView()
                } else {
                    Text("Continue with Google", bundle: .module)
                }
            }
            .frame(maxWidth: .infinity, minHeight: 50)
        }
        .buttonStyle(.bordered)
        .buttonBorderShape(.roundedRectangle(radius: Radius.control))
        .disabled(model.isSigningIn)
        .opacity(model.pending == .apple ? 0.4 : 1)
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
                model.signInWasCancelled()
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
