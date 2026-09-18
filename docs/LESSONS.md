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
| A concurrency test that *races* the actor is a coin toss the actor always wins. `signOutDuringRefreshDiscardsResult` never called the transport at all, and passed against two real `TokenRefresher` bugs while claiming to cover them | `GatedRefreshTransport` in `TestSupport` — hold the call open, act on the actor, *then* let it land. Judgment still required: nothing mechanical can tell a staggered test from a raced one |
| A boolean cannot tell "signed out" from "signed out and signed back in" — `adopt()` clears it, so a stale refresh passed the guard and wrote a dead pair over a live session | The generation counter, and `TokenRefresherRegressionTests`. Reach for the monotonic counter, not the flag, whenever an `await` sits between check and use |
| A duplicate `docs/` tree under `mindlens/` survived a session unnoticed and was staged for commit | `Tools/check-doc-links.py` — one home per document, and exactly one `docs/` tree |
| `codesign -d --entitlements` on a simulator build printed `{}` and read as "the entitlement was dropped". It was not: the simulator enforces `__TEXT,__entitlements`, and ad-hoc signing leaves the signature's set empty. The asserting tool can lie in *both* directions | `Tools/check-build-settings.sh` decodes the section itself, and was shown to fail on a binary with the key renamed before it was trusted to pass |
| No `DEVELOPMENT_TEAM` in the project, so Xcode filled it from the signed-in account — a team that did not own the App ID. Sign in with Apple failed with `-7022` and the build had looked entitled | `Tools/check-build-settings.sh` asserts the team, like every other setting the docs claim |
| `base-setup` reached twenty commits with no pull request; a gate branch forked from a stale `main`, merged it once, and the two re-diverged within two hours — every landing needed a `docs/STATE.md` conflict resolved by hand | `Tools/githooks/pre-commit`: at most five commits no pull request can see, and a refusal to commit on a pushed branch another open PR already contains. Installed by the `SessionStart` hook, so it binds a terminal too |
| ADR 0006 said the gate's exit code was "wired to branch protection". Protection is a 403 on this plan; nothing had ever stood between `git push` and `main` | `Tools/githooks/pre-push` refuses `main` and any non-fast-forward; `Tools/check-pr-conventions.sh` in the gate checks branch name, ADR collisions across open PRs, and size |
| The launch smoke test waited 30 s for a screen on a simulator nobody had booted; on GitHub's macOS runners the boot ran inside the first test and took 73–928 s, so 3 of 5 runs failed on whichever test launched first, and each looked like a flake to re-run | The workflow boots the simulator in the pick step and waits for `bootstatus`; a failed launch test is retried (`-retry-tests-on-failure`), a crash still fails three times |
| A CI secret stored with a leading space. The action masked it, hid the 401 behind "output hidden for security", and three runs of the reviewer died at turn 1 with no cause anyone could read | `show_full_output: true` on the reviewer step, and a step that compares the token to its trimmed self before the reviewer runs — never prints it |
| `AppError.diagnostic` was written at every boundary and read by nothing, so a 422 reached the screen as generic copy and its cause stayed inside the process. A field "kept for logs" with no logger is a field kept for nobody | `SessionModel.failed()` — the only way an error reaches `error` — logs it; swiftgate's print rule already points at `Logger` |
| The Flutter checkout "succeeded" with an empty tree, so the step keyed on its outcome never warned, the reviewer said in prose it found no Dart, and the gate reported *Passed* on PR #1. A judge that could not look was scored as a judge that found nothing | The evidence gate in `swiftgate prepare`: a lane's inputs are asserted off disk before the judge runs, and absence is `CANNOT_EVALUATE`, which the scorer blocks on (ADR 0014). `Tools/swiftgate/main_test.go` replays the empty checkout |
