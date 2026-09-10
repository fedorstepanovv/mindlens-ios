// swift-tools-version: 6.2
import PackageDescription

// Feature targets are added as features are built — see docs/STATE.md.
// A feature target must never depend on another feature target; cross-feature
// navigation goes through the app target. See docs/ARCHITECTURE.md.

/// Applied to every target. `ExistentialAny` makes the `any` we already write mandatory
/// rather than stylistic; `MemberImportVisibility` stops a target using API it never
/// imported, which is the module-boundary equivalent of the dependency rule.
let strict: [SwiftSetting] = [
    .enableUpcomingFeature("ExistentialAny"),
    .enableUpcomingFeature("MemberImportVisibility"),
]

/// UI-facing targets are MainActor by default (SE-0466). The app target already sets
/// SWIFT_DEFAULT_ACTOR_ISOLATION = MainActor, so without this the isolation default
/// would flip at the module boundary and produce confusing diagnostics.
let uiFacing: [SwiftSetting] = strict + [.defaultIsolation(MainActor.self)]

let package = Package(
    name: "MindlensKit",
    defaultLocalization: "en",
    // macOS is declared so `swift build` / `swift test` work from the CLI for the
    // platform-agnostic targets. The app itself ships iOS only.
    platforms: [.iOS(.v18), .macOS(.v15)],
    products: [
        // No umbrella product: an "everything" library is a barrel export, which
        // CLAUDE.md bans, and it would force the app to link every feature.
        .library(name: "Core", targets: ["Core"]),
        .library(name: "Models", targets: ["Models"]),
        .library(name: "Networking", targets: ["Networking"]),
        .library(name: "Persistence", targets: ["Persistence"]),
        .library(name: "DesignSystem", targets: ["DesignSystem"]),
        .library(name: "Analytics", targets: ["Analytics"]),
        .library(name: "Dashboard", targets: ["Dashboard"]),
    ],
    targets: [
        // MARK: Foundation
        .target(name: "Core", swiftSettings: strict),
        .target(name: "Models", dependencies: ["Core"], swiftSettings: strict),
        .target(name: "Networking", dependencies: ["Core", "Models"], swiftSettings: strict),
        .target(name: "Persistence", dependencies: ["Core", "Models"], swiftSettings: strict),
        .target(
            name: "DesignSystem",
            dependencies: ["Core"],
            resources: [.process("Resources")],
            swiftSettings: uiFacing
        ),
        .target(name: "Analytics", dependencies: ["Core", "Models"], swiftSettings: strict),

        // MARK: Features
        // Dependencies are listed per feature, not shared: a feature that doesn't touch
        // persistence shouldn't link it, and shouldn't rebuild when it changes.
        .target(
            name: "Dashboard",
            dependencies: ["Core", "Models", "DesignSystem", "Analytics"],
            path: "Sources/Features/Dashboard",
            swiftSettings: uiFacing
        ),

        // MARK: Test support
        // Fakes implement protocols that live in shared targets — never in features —
        // which is what lets one TestSupport serve every test target.
        .target(
            name: "TestSupport",
            dependencies: ["Core", "Models", "Networking", "Analytics"],
            resources: [.process("Fixtures")],
            swiftSettings: strict
        ),

        // MARK: Tests
        .testTarget(name: "CoreTests", dependencies: ["Core", "TestSupport"], swiftSettings: strict),
        .testTarget(name: "ModelsTests", dependencies: ["Models", "TestSupport"], swiftSettings: strict),
        .testTarget(name: "NetworkingTests", dependencies: ["Networking", "TestSupport"], swiftSettings: strict),
        .testTarget(name: "DashboardTests", dependencies: ["Dashboard", "TestSupport"], swiftSettings: uiFacing),
    ]
)
