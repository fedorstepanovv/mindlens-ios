# ADR 0026 — The Keychain tests are hosted by the app, not the package

Date: 2026-09-23 · Status: Accepted · Replaces auth step 6's "under a `Persistence` test target"

## Context

`KeychainTokenStorage` and `KeychainDeviceIdentity` had run for real since 2026-09-11 and never
under a test. `docs/features/auth.md` step 6 planned a `PersistenceTests` target in the package,
beside the other unit tests. Two spikes on 2026-09-23 settled where the tests can run:

- **Package test bundle, iOS simulator** (the gate's `xcodebuild test -scheme MindlensKit-Package`):
  the first `SecItemAdd` returns `-34018`, `errSecMissingEntitlement`. A hostless test bundle is not
  a signed app, so it gets no Keychain.
- **Package test bundle, macOS host** (`swift test`): passes, but against the macOS file-based
  keychain. That keychain is not the data-protection keychain iOS uses, so what it says about
  `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` is not what an iPhone does. On a CI runner it
  also depends on a login keychain being unlocked.
- **The app-hosted `mindlensTests` bundle, iOS simulator**: passes, against the same
  data-protection Keychain a device uses. It was an empty Xcode template stub that the gate never ran.

## Decision

The Keychain tests live in `mindlensTests`, hosted by the signed app on the simulator, and use only
`Persistence`'s public API. `KeychainProbe` reads attributes back and plants items the public
API would never write. The gate runs `-only-testing:mindlensTests` on every PR, with no retry. It
is a separate step from the launch test, which is skipped on most PRs and retried when it fails.

The package keeps no `PersistenceTests` target. A target that can only pass on a host other than
the one the gate uses would be a second way of being green.

## Consequences

- The accessibility class is asserted by reading it back from the real Keychain.
  This matters: the first run found that `KeychainItem.write` deleted by *attributes*, so an item
  stored under another class survived the delete. The add then failed as a duplicate, which cost
  a GUID per launch or a session that could never save a rotated pair.
- Each run launches the app on the simulator to host the bundle. Its cost in the gate is unmeasured until the first run.
- Tests of a package module live outside the package. `swift test` does not run them. To run them
  locally, use `xcodebuild test -scheme mindlens -only-testing:mindlensTests`.
- `mindlensTests` is now linted by SwiftLint and swift-format like every other Swift directory.

## What would change this

A package test bundle that can reach the Keychain on the simulator, for example through a test-plan
host setting that Swift Package Manager honours. Also a second store that needs the device's
behaviour, such as SwiftData's file protection in Stage 2. If that arrives, the app-hosted bundle
becomes the home for every device-behaviour test, and this ADR should say so.
