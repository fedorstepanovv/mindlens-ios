import SwiftUI

@main
struct MindlensApp: App {
    /// The object graph, built once. `@State` so the scene owns it for the app's lifetime
    /// rather than rebuilding it on every body evaluation.
    @State private var container = AppContainer()

    var body: some Scene {
        WindowGroup {
            RootView(session: container.session)
        }
    }
}
