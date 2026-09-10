// swift-tools-version: 6.0
import PackageDescription

// Feature targets are added here as features are built — see docs/STATE.md for what
// exists. A feature target must never depend on another feature target; cross-feature
// navigation goes through the app target. See docs/ARCHITECTURE.md.

let sharedDependencies: [Target.Dependency] = [
    "Core", "Models", "Networking", "Persistence", "DesignSystem", "Analytics",
]

let package = Package(
    name: "MindlensKit",
    platforms: [.iOS(.v18)],
    products: [
        .library(name: "MindlensKit", targets: ["Authentication", "Dashboard"]),
        .library(name: "Core", targets: ["Core"]),
        .library(name: "Models", targets: ["Models"]),
        .library(name: "Networking", targets: ["Networking"]),
        .library(name: "Persistence", targets: ["Persistence"]),
        .library(name: "DesignSystem", targets: ["DesignSystem"]),
        .library(name: "Analytics", targets: ["Analytics"]),
    ],
    targets: [
        // MARK: Foundation
        .target(name: "Core"),
        .target(name: "Models", dependencies: ["Core"]),
        .target(name: "Networking", dependencies: ["Core", "Models"]),
        .target(name: "Persistence", dependencies: ["Core", "Models"]),
        .target(name: "DesignSystem", dependencies: ["Core"]),
        .target(name: "Analytics", dependencies: ["Core", "Models"]),

        // MARK: Features
        .target(name: "Authentication", dependencies: sharedDependencies, path: "Sources/Features/Authentication"),
        .target(name: "Dashboard", dependencies: sharedDependencies, path: "Sources/Features/Dashboard"),

        // MARK: Test support
        .target(
            name: "TestSupport",
            dependencies: ["Core", "Models", "Networking", "Analytics"],
            resources: [.process("Fixtures")]
        ),

        // MARK: Tests
        .testTarget(name: "CoreTests", dependencies: ["Core", "TestSupport"]),
        .testTarget(name: "NetworkingTests", dependencies: ["Networking", "TestSupport"]),
        .testTarget(name: "AuthenticationTests", dependencies: ["Authentication", "TestSupport"]),
        .testTarget(name: "DashboardTests", dependencies: ["Dashboard", "TestSupport"]),
    ]
)
