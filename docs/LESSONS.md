# Lessons

Behavioural memory. Mistakes made in this repository, so they are not made twice.

**The discipline:** a lesson is only finished when it has a **guard** — something
mechanical that fails when the mistake recurs. A lesson with no guard is a lesson that
will be relearned, so each entry must name its guard or say why none is possible.

**Hard cap: 40 lines.** When it fills up, entries whose guard is now automated are
deleted — the guard *is* the memory at that point, and a rule that a machine enforces
does not need a human to remember it. This file holds only what still relies on judgment.

| Mistake | Guard |
|---|---|
| A doc held a second copy of security-critical code (`TokenRefresher`) and drifted to the broken version | `PATTERNS.md` links to real files instead of copying them; swiftgate rule candidate |
| A localized string in a package target without `bundle: .module` — resolves against `Bundle.main` and silently never localizes. `SwiftUI.Text("…")` has the same hole as `String(localized:)` | SwiftLint `custom_rules.localized_needs_bundle` and `custom_rules.text_needs_bundle`, both severity error |
| Driving a load from `didSet { Task { … } }` produced uncancelled overlapping loads | SwiftLint `custom_rules.no_task_in_didset`; `.task(id:)` documented in `PATTERNS.md` |
| Static mutable state in a test helper broke under Swift Testing's parallel execution | `TESTING.md` "tests run in parallel" section; worked example kept there |
| A protocol placed in `Networking` forced `Persistence` to depend on it — arrow backwards | The compiler, via SPM target boundaries (ADR 0002) |
| Two sessions created ADR `0006` simultaneously | `CLAUDE.md` routing rule: re-list `docs/decisions/` immediately before adding one |
| Whole-file rewrites in a repo another session was editing | `CLAUDE.md` rule: check mtime/`git status` before overwriting; prefer targeted edits |
| `Config/Base.xcconfig` held the right values but nothing referenced it, so the build kept the template's Swift 5 / iOS 26.2 while every doc said Swift 6 / iOS 18 | `Tools/check-build-settings.sh` in the PR gate — it resolves the settings instead of trusting that a file exists |
| `X = //` in an xcconfig defines an *empty* value: the comment rule applies inside values too, so `API_BASE_URL` silently lost its scheme separator | Same script asserts the resolved `API_BASE_URL`, not just the file's contents |
| A force-unwrap was suppressed for SwiftLint and assumed handled; swift-format enforces the same ban separately, and the two disagree about which sites even count | CI lints `Tests` as well as `Sources`, so both tools see every file |
| `INFOPLIST_KEY_<custom>` resolved perfectly in `-showBuildSettings` and never reached the bundle: Xcode forwards only the names on its own allowlist and drops the rest silently. The app shipped a launch crash with every test passing | `Tools/check-build-settings.sh` reads the **built** `Info.plist`, not the setting. A setting that resolves is not a setting that shipped — assert the artifact |
| **`** BUILD SUCCEEDED` was treated as verification.** The app compiled, passed 75 tests, and died on the first line of its composition root | `mindlensUITests/LaunchSmokeTests.swift` in the PR gate: the gate now launches the app, at default and at the largest accessibility text size |
| A duplicate `docs/` tree under `mindlens/` survived a session unnoticed and was staged for commit | `Tools/check-doc-links.py` — one home per document, and exactly one `docs/` tree |
