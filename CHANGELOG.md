# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

This file is **append-only history**. `docs/STATE.md` holds only what is true *now* and is
rewritten in place — when something stops being current, it moves here rather than
accumulating there. That split is what stops the state file growing into the 2,000-line
progress log this project has produced before.

## [Unreleased]

### Added
- **Auth's `## Behaviour`**, approved 2026-09-22: the first under ADR 0020. The pre-sign-in survey (goal, feeling,
  hurdle) belongs to auth, not Stage 4, and a new account's answers are posted after sign-in (auth step 7).
- `TestSupport/ConstructedResponse`: the one constructed body each for `POST /auth/apple` and `/auth/refresh`, which
  cannot be captured. Cross-checked against Prisma and the source app's production models (ADR 0024). The three
  hand-written copies it replaces are gone, and so are two `swiftgate:allow` comments that named deleted rules.
- The gate's judge lanes are **advisory**, and earn blocking on their own record. `.github/swiftgate.yml`'s
  `lanes:` becomes a map of lane → `{blocks, model}`: every listed lane runs, is reported on the pull request
  and writes a metrics record, and only one with `blocks: true` can stop a merge. All three start false —
  none had judged a real diff, so the gate was blocking on the word of judges whose noise rate nobody had
  measured. The grant and the kill rules sit in the file beside the flag: ≥10 judged pull requests with noise
  ≤30% and one finding classified `changed` earns it; ≥10 judged runs with noise >50% and none `changed`
  deletes the lane, with an ADR (ADR 0021). A lane name the gate does not have is now a config error, on the
  same argument as a misspelt severity — `blocks: true` under a typo reads as a granted lane and is a lane
  that never runs.
- **A `proof` per finding**, required by the schema and again by `Ingest`: the input or state and the line
  where it fails, or the exemplar contradicted. A finding without one is dropped before anyone reads it and
  counted in the lane's record, so a judge writing things it cannot support is visible rather than merely
  noisy. The three lane skills each define what a proof is for their question. A judge's verdict still stands
  when every finding behind it was dropped — "BLOCK, 0 findings, 2 unproven" is the truth about that judge.
- **Review levels** (ADR 0017). `swiftgate decide` computes *pass*, *brief* or *full* from the changed paths,
  the changed-line count, the lane findings and the size cap; posts it as the first line of the readiness
  comment with every reason; and sets a `review:<level>` label. `risk_paths:` and `pass:` live in
  `.github/swiftgate.yml`; the size cap is read from `MINDLENS_PR_SIZE_CAP`, the variable
  `Tools/check-pr-conventions.sh` already used, so one number serves both. A pull request that changes no
  Swift still gets a level — it can still touch `AGENTS.md`, a workflow or the gate.
- **A model per lane**, named in `.github/swiftgate.yml`, published by `prepare`, passed to the lane's step
  and written into its metrics record: verification and spec on Opus, idiom on the cheaper model. A noise rate
  is a number about a lane *and* a model, and the workflow had one model hard-coded in three places.
- **`Tools/setup.sh`** — the once-per-clone install a human runs: `core.hooksPath`, the hooks' executable bit,
  `gh` and its authentication, Go, and whether the gitignored Firebase plist is present. `AGENTS.md` says to
  run it; the session-start hook calls it and stays quiet when there is nothing to do. The guard is the
  script, which binds a terminal; the hook binds one agent (ADR 0018).

### Changed
- The **idiom lane runs only where structural translation happens**: a diff that adds a type conforming to
  `View`, an `@Observable` type, or a whole new file under a feature target. Every other diff records a skip
  with its reason, so `swiftgate metrics` can report how often the lane was eligible at all. It is the lane
  expected to fail the grant rule, and this is what bounds its cost until the measurement is in.
- **A run with no Swift is recorded**, one skipped record per lane. The records used to be silent about it,
  which meant they could say how a lane did on the runs it judged and never how often there was anything to
  judge — and coverage is half of whether a lane earns its keep.
- The caps drop to ADR 0017's numbers: **1,000 changed lines** per pull request (was 4,000) and **10 commits**
  no pull request can see (was 25). `swiftgate` counts changed lines with the same exclusions the conventions
  check uses, so the cap enforced and the number the level cites cannot drift apart.
- `/pr status` leads with the review level and separates a blocking red from an advisory lane's verdict; its
  old wording implied a `CANNOT_EVALUATE` was what made the gate red, which is no longer true and would send
  the reader to fix the wrong thing. `/pr land` says Fedir reads to the level the gate assigned.
- `AGENTS.md` where `CLAUDE.md` was, in the files this branch touched anyway: the two static rules' `Doc:`
  strings, the SwiftLint `*Impl` message, the pre-commit header, and the `feature-start`, `orient` and
  `lane-idiom` skills. The import gets a Claude reader there either way; a Codex or Cursor reader landed on
  four lines about Claude Code.
- `docs/LESSONS.md`: the ADR-collision entry is retired — `Tools/check-pr-conventions.sh` checks decision
  numbers against `main` and every open pull request's head on every run, so the guard is the memory now.

- The pipeline, written down. `README.md` is the artifact: what this is, the seven stages and their owners
  (ADR 0019), the decisions in ten areas held against September 2026 practice with the incident behind each,
  an evidence section that stays empty until `swiftgate metrics` fills it, what to copy into another repository
  and what is Claude-specific, and a study of every gate component with its incident and its measurement.
  `AGENTS.md` is the canonical instruction file for every agent and person; `CLAUDE.md` is `@AGENTS.md` plus
  the Claude-only lines; `.agents/skills` is a symlink to `.claude/skills`, and the three checklists
  `feature-start`, `feature-done` and `orient` move from `.claude/commands/` into it as skills (ADR 0023). A Glossary in
  `docs/ARCHITECTURE.md` gives session, lane, gate, pipeline, spec, behaviour, source app, transfer, port,
  review level, risk class and parity checkpoint one meaning each. The feature template gains a human-approved
  `## Behaviour` section — flows, screens, copy, API calls, edge cases, acceptance criteria — and its cap rises
  from 100 to 150 lines (ADR 0020). Seven ADRs from the 2026-09-21 plan: review levels with a human on the
  button (0017), guards in git and CI not the agent (0018), pipeline stages and owners (0019), behaviour as
  the one intake artifact (0020), lanes advisory until earned with the grant and kill rules (0021), memory
  structure (0022), skills (0023). `Tools/check-doc-links.py` caps `AGENTS.md` at 200 lines and `CLAUDE.md` at
  40, and checks `.github/` and `.agents/` paths too. The mechanisms the ADRs decide — `blocks:`, `proof`, the
  review level, the caps, `setup.sh` — land with `feature/gate-advisory`; this branch is prose and config.
- Lane metrics: `swiftgate decide` writes one JSONL record per lane per run — verdict, the cause
  behind a `CANNOT_EVALUATE`, evidence, findings, and cost, duration and turns when the action's
  execution file reports them — uploaded as `swiftgate-metrics-pr<N>-…` and kept 90 days.
  `swiftgate metrics` sums them: verdicts, cost and noise per lane, fires per rule, rules that never
  fired. `swiftgate outcomes --pr N`, run by `.github/workflows/pr-outcomes.yml` when a pull request
  merges, classifies each finding `changed`, `resolved`, `waived` or `untouched` against the head
  that merged; `untouched ÷ classified` is the noise rate (ADR 0016). Verified against fixtures
  with known answers only — no lane has judged a real diff yet.
- The three judge lanes, each a skill under `.claude/skills/lane-<name>/` with its own rubric and
  calibration: `lane-verification` (do the tests prove the changed path — fakes throwing what
  production throws, staggered not raced, captured not typed), `lane-idiom` (is this the Swift we
  write here, held against exemplars), `lane-spec` (does the diff do what the open feature step
  agreed). They replace `idiom-review`. Every lane scores.
- Structured output: each lane returns `{verdict, summary, findings}` validated against the schema
  `swiftgate prepare` publishes; a verdict outside `PASS · CONCERNS · BLOCK` is `CANNOT_EVALUATE`.
- Exemplar retrieval: `prepare` picks up to three merged files resembling each changed one, by kind,
  name and neighbourhood, and writes them into the brief. They are the idiom lane's evidence; the
  Flutter checkout is optional context.
- Google Sign-In runs for real: the redirect scheme (`REVERSED_CLIENT_ID`) declared under
  `CFBundleURLTypes` in `mindlens/Info.plist`, and `Tools/check-build-settings.sh` asserts it against the
  bundled `GoogleService-Info.plist` when one is present (`docs/features/auth.md` step 4).
- `Tools/swiftgate` judge lanes: a verdict contract (`PASS · CONCERNS · BLOCK · CANNOT_EVALUATE`),
  an evidence gate per lane run by `prepare` and on its own as `swiftgate evidence --lane`, and a
  deterministic scorer in `decide` that alone blocks — on a static blocker, a lane `BLOCK`, or a
  lane that could not evaluate. One SHA-stamped sticky comment per lane (ADR 0014).
- `Tools/swiftgate/main_test.go` — replays PR #1's empty Flutter checkout end to end and asserts
  the gate blocks with `CANNOT_EVALUATE`.
- Documentation system: `CLAUDE.md` front door, `docs/` reference set, ADRs, and
  `docs/STATE.md` as a bounded current-state file.
- `MindlensKit` local SPM package — `Core`, `Models`, `Networking`, `Persistence`,
  `DesignSystem`, `Analytics`, `Dashboard`, `TestSupport`. Swift 6 language mode,
  iOS 18 floor, strict concurrency, `ExistentialAny` and `MemberImportVisibility` on.
- `TokenRefresher` — single-flight refresh with a generation guard and failure
  classification (ADR 0004).
- `APIClient`, typed `Endpoint`, envelope decoding, `KeychainTokenStorage`.
- `CalendarDate` and `TimeOfDay` for the API's non-timestamp date formats.
- `StubURLProtocol` in `TestSupport`, so no test touches the network.
- SwiftLint + swift-format, split so the formatter owns formatting and the linter owns
  correctness.
- `Tools/check-doc-links.py` — fails on any doc referencing a path that does not exist.
- `Tools/capture-fixtures.sh` — re-captures API fixtures from the live server.
- `Tools/swiftgate` and the PR gate workflow — deterministic Flutter-ism rules plus an
  idiom review that runs as Claude Code on the subscription, not the API (ADR 0006, 0012).
  `.claude/skills/idiom-review/` carries the rubric; the gate keeps the exit code.
- `Config/Base.xcconfig` with the production API URL; secrets kept out of git.
- `Tools/check-build-settings.sh` — resolves the app's build settings and asserts them,
  so a claim about Swift version or deployment target cannot outlive the setting.
- ADR 0009 — build settings live in xcconfig, applied at the project level.
- **Stage 1, authentication.** The app has UI: the scene root is a `switch` on
  `SessionState`, and the signed-out branch is a sign-in screen built on the system
  `SignInWithAppleButton`, with the legal notice as inline markdown links.
- `SessionModel` — one `@Observable @MainActor` model for the whole session flow (restore,
  sign in, sign out) rather than one per screen.
- `APIAuthRepository` in `Networking`, so `AuthSessionDTO` never leaves that target;
  `Features/Authentication` links neither `Networking` nor `Persistence` (ADR 0010).
- `IdentityAuthenticating` in `Core` — the Firebase exchange behind a protocol we own, with
  only plain `Sendable` values crossing it. `UnavailableIdentityProvider` in the app target
  throws until the SDK is linked (ADR 0010).
- `AuthTokenRefreshTransport` — the live `POST /auth/refresh`, deliberately not routed
  through `APIClient`, which would recurse into refresh on a rejected refresh token.
  `TokenRefresher` had no concrete transport before this.
- `KeychainDeviceIdentity` — an install-persistent device GUID, an `actor` because
  get-or-create is a read-modify-write, and preserved across sign-out so the next sign-in
  reuses this device's session slot instead of evicting another device.
- `KeychainItem`, extracted so the token pair and the device GUID share one set of
  `SecItem` calls and one accessibility attribute. **Note for anyone with a build already
  installed:** the token pair moved from two Keychain items (`access`, `refresh`) to one
  (`tokens`), so an existing install is signed out once and the two old items are orphaned.
- `SystemDeviceModel` — `utsname.machine` with the simulator case handled: it reports
  `arm64` there, one character under the API's six-character floor, which was a 400 on
  every sign-in on every developer's machine.
- `DateProvider.timeZone`, the seam that lets `TimeZone.current` be banned everywhere else.
- ADR 0010 — identity exchange behind a protocol, auth repository in `Networking`.
- **One file per feature** — `docs/features/<name>.md` from `docs/features/0000-template.md`:
  what the user can do and when it is done, decisions settled so they are not reinvented,
  the next few steps with `entries` (entry points) / `files` / `ready` and inline status, and an append-only
  journal. Capped at 100 lines. `/feature-start` takes the first open step, `/feature-done`
  ticks it, and the stop hook accepts a feature-file update. `docs/features/auth.md` is the
  first (ADR 0011). `Tools/check-doc-links.py` fails when a feature file has no stage row in
  `docs/STATE.md`, or when the two disagree about its status.
- The app's composition root (`AppContainer`) and session gate (`RootView`).
- `#Preview` blocks on `SignInView` (signed out; dark at the largest accessibility size;
  signing in; the 422 error) and `RootView` (restoring; unreachable server; signed out; signed
  in), backed by `PreviewAuthRepository` and `User.preview` in `Models` under `#if DEBUG`.
- **Firebase Auth, linked** (`firebase-ios-sdk` 12.19, `FirebaseAuth` product, app target
  only). `FirebaseIdentityProvider` does the Apple → Firebase → ID-token exchange behind
  `IdentityAuthenticating`, returning the email Firebase keeps from the first authorization;
  Firebase's network error maps to `.offline` at that boundary. `AppContainer.init()` configures
  Firebase only when `GoogleService-Info.plist` is bundled — it is gitignored, so CI never has
  one — and keeps `UnavailableIdentityProvider` otherwise; in Release a missing plist is a
  `preconditionFailure`. `mindlens/mindlens.entitlements` grants Sign in with Apple, and
  `Tools/check-build-settings.sh` now decodes the built binary's `__TEXT,__entitlements`
  section to prove it shipped — `codesign` reads an empty set off a simulator build.
  Verified live: with the native bundle ID registered as a second iOS app in the production
  Firebase project, Sign in with Apple lands on the signed-in stub and a relaunch restores the
  session. `DEVELOPMENT_TEAM` is now set and asserted — unset, Xcode guessed a team that did
  not own the App ID and AuthKit answered `-7022`.
- `Logger(category:)` in `Core` — one subsystem (the bundle ID), a category per concern.
  `SessionModel` now logs every failure that reaches `error` with its diagnostic, through one
  `failed()` so no site can map without logging. `AppError.diagnostic` had been written
  everywhere and read nowhere: a 422's server message never left the process.
- `docs/API.md` — the API verifies Firebase tokens against **one** project, production's, and
  there is no dev backend; `/auth/apple` has two distinct 422 messages.
- Branch guards that install themselves: `Tools/githooks` (pre-commit, pre-push), pointed at by
  `core.hooksPath` from the `SessionStart` hook, so ADR 0013 binds a terminal as well as a
  session. `Tools/check-pr-conventions.sh` runs in the gate — branch name, ADR numbers checked
  against every open pull request's head, and a 1,500-line cap lifted by `size-override`.
- Pull request conventions: `.github/PULL_REQUEST_TEMPLATE.md`, the `/pr` skill
  (`.claude/skills/pr/SKILL.md`) to open, check and land one, and ADR 0013 — one branch per
  feature step, merge commits, a red gate never merged. `main` cannot be protected on this
  plan, so the gate binds by convention; ADR 0006 had assumed otherwise.

### Changed
- Gate cleanup, from the 2026-09-21 inventory. `ParseSeverity` refuses a string outside
  `blocker · warning · nit · off` — it defaulted to `warning`, so a typo in `.github/swiftgate.yml`
  silently downgraded a blocker; the config loader and `Ingest` now fail on one. The doc-link check
  runs in the seconds-long `conventions` job instead of at the end of the macOS build. SwiftLint is
  pinned to a release and its checksum instead of brew's latest. `MINDLENS_ALLOW_ABSORBED=1` needs
  `MINDLENS_OVERRIDE_REASON`, and a `prepare-commit-msg` hook writes the reason into the commit as an
  `Override:` trailer. `pre-commit` refreshes the open-PR cache from `gh` itself. `session-start.sh`
  warns when the main checkout is off `main`. The workflow header says what enforces the gate on this
  plan: the human and the pre-push hook, not branch protection. `TestEveryRuleHasATest` fails on any
  static rule `static_test.go` does not name; `flutter/service-locator`, `arch/our-singleton` and
  `flutter/widget-suffix` got the tests they lacked. `docs/features/auth.md` step 5 no longer names
  `--static-only`, a flag that never existed.
- Correction to the `main_test.go` line under *Added* below: since exemplar retrieval, the test
  gives the idiom lane a base branch with no exemplar and asserts the `CANNOT_EVALUATE` block. It
  never read the Flutter tree; that step is now removed.
- Branch conventions (ADR 0015, amending 0013): one branch per initiative, named for what the reader
  gets, kind one of `feature`, `bugfix`, `hotfix`, `docs`. `tooling` and `fix` are gone. The pre-commit
  cap on commits in no pull request rises from 5 to 25 and the size cap from 1,500 to 4,000 lines. A
  `feature/` branch with no feature file is tooling: the spec lane skips it and says so.
- The launch smoke test runs only when a pull request touches something that can break a launch: the
  app target, project, UI tests, xcconfig, package manifest or the workflow. It was six of the build
  job's eighteen minutes on every pull request.
- `CLAUDE.md`: a session works the one branch it was asked for, and reports a red gate instead of
  fixing it unasked.
- `CLAUDE.md`: one worktree per branch, and the main checkout stays on `main`. A session switched
  the shared checkout to `feature/auth` under another session's feet; nothing was lost, once.
- The PR gate's `idiom` job is `readiness`; the reviewer runs only when the idiom lane's evidence
  is on disk, and `anthropics/claude-code-action` is pinned to the commit behind `v1`.
- Three static rules removed as duplicates of SwiftLint errors: `flutter/impl-suffix`,
  `swift/force-try-cast`, `swift/todo-in-gate`.
- Split the memory system so no file grows without bound: `docs/STATE.md` is now
  current-state only (rewritten, never appended, capped at 80 lines), with history moving
  here and decisions to ADRs.
- Added `docs/LESSONS.md` as behavioural memory — each entry must name the mechanical
  guard that stops the mistake recurring, and is deleted once the guard makes it
  unrepeatable.
- Added a routing table to `CLAUDE.md` so a session reads ~100 lines to start, then only
  the documents its task actually touches.
- `Tools/check-doc-links.py` now enforces size caps as well as resolving references.
- SwiftLint custom rules turn three past mistakes into build failures:
  `localized_needs_bundle`, `no_task_in_didset`, `no_impl_suffix`.
- `Config/Base.xcconfig` is now the Xcode project's base configuration for Debug and
  Release. The `SWIFT_VERSION` and `IPHONEOS_DEPLOYMENT_TARGET` entries were deleted from
  all six target configurations, where they had been overriding it (ADR 0009).
- The PR gate lints `Packages/MindlensKit/Tests` alongside `Sources`, and verifies the
  resolved build settings before linting.
- `Tools/check-doc-links.py` now also fails on a document living outside its one home.
- Per-user Xcode scheme state is no longer tracked; `.gitignore` already listed it.
- `docs/API.md` documents a **third** error-envelope shape: a 422 spells its status
  `status`, not `statusCode`, and carries no `error` key. Captured live as
  `error_invalid_token_422.json`. `data.user` is documented as the whole Prisma row.
- `AppConfiguration` reads the API origin from the app's Info.plist, which
  `mindlens/Info.plist` fills in from `API_BASE_URL` — one source of truth.
  `Tools/check-build-settings.sh` asserts the key in the **built** bundle.
- `SwiftLint custom_rules.text_needs_bundle` — `SwiftUI.Text("…")` in a package target has
  the same silent non-localization hole as `String(localized:)`.
- `Tools/check-doc-links.py` skips `worktrees`. A git worktree is a different checkout, so
  `.claude/worktrees/gate/docs` was being reported as a duplicate `docs/` tree — the check
  failed for everyone whenever a gate run was in flight.

### Removed
- `swiftgate check` and `swiftgate evidence`: no workflow, hook, skill or script invoked either.
  Static findings come from the `readiness` job in seconds.
- The Flutter checkout step, `FLUTTER_SPEC_TOKEN`, the brief's "Flutter spec" section and the idiom
  skill's permission to open the tree. Exemplars from this repository replaced it (ADR 0014); the
  step had been running on every pull request for nothing.
- Six static rules with no test: `arch/imports-app-target` and `arch/dto-escapes-networking` (the
  compiler refuses both — a package cannot import the app module, and every DTO is internal to
  `Networking`); `swift/dispatch-semaphore`, `swift/combine-as-transport`, `core/calendar-current`
  (advice in `PATTERNS.md`, none of it an absolute, and the Combine one fired on the uses the document
  allows); `test/viewmodel-untested` and `test/dto-without-fixture` (substring heuristics — the
  verification lane owns that question). The waivers those last two forced in
  `AuthEndpoints.swift` and `DateProvider.swift` are dead comments until the next Swift branch
  deletes them.
- `MINDLENS_PUSH_MAIN=1`: `main` moves by pull request, no exceptions; `gh pr merge` never touches
  the pre-push hook, and a team gets the rule from a ruleset.

### Fixed
- `Tools/check-doc-links.py` checked nothing when run inside a worktree and reported success; it
  now matches skip directories below the repo root and refuses to pass with zero documents. Append-only
  history (`CHANGELOG.md`, `docs/decisions/`) may name a deleted path git still remembers.
- `CLAUDE.md` was 157 lines against its cap of 150 after the worktree rule landed without waiting for the
  gate. Trimmed back under the cap by rewording, no rule changed.
- The pre-commit commit cap fired only when a pull request was already open: with none, bash 3.2
  rejected the empty `heads` array under `set -u`, and the fallback counted zero unreviewed commits.
- The launch smoke test on GitHub's macOS runners: the simulator is booted before the builds
  start instead of inside the first UI test, a failed launch is retried, and the screen wait
  is 60 s. Three of five runs had failed with no code change.
Review follow-up, 2026-09-11 — two independent reviews of the Stage 1 branch:
- **A refresh in flight at sign-out overwrote the next session's tokens.** `adopt()` clears
  `sessionIsOver`, so a refresh that resumed after a sign-out *and a fresh sign-in* passed the
  guard and wrote the dead session's rotated pair over the live one — ADR 0004's incident through
  the side door. The guard is a generation comparison now, and sits outside the `catch`, where an
  `.unauthenticated` would otherwise be classified as a rejected token and tear down the new
  session it was protecting.
- **`defer { inFlight = nil }` unregistered whichever refresh was current, not its own**, so a
  finishing refresh could silently detach another and the next caller would start a second
  concurrent refresh on a single-use token. Registrations are stamped.
- Both are covered by `TokenRefresherRegressionTests`, which reproduces each deterministically —
  and `signOutDuringRefreshDiscardsResult`, cited in `docs/TESTING.md` as one of the two tests
  "that would have caught real bugs", never called the transport at all.
- A **Keychain read failure was indistinguishable from "no session"**: `try?` turned a locked or
  corrupt Keychain into a silent sign-out at launch, with the tokens left on disk.
- `SessionModel.restore()` set `.signedOut` on a rejected session **without ending it**, so every
  launch rotated a fresh refresh token, failed, and parked the user on sign-in — indefinitely.
- **Sign-out spent a single-use refresh token** on its way out: `POST /auth/logout` went through
  `APIClient`, which refreshes on 401. `Endpoint.retriesAfterRefresh` opts it out.
- `SessionModel` had **no re-entrancy guard** — two sign-ins could run concurrently, the first
  `defer` re-enabling both buttons mid-flight, and a stale error could survive into a live session.
- A **malformed Apple credential was reported as a cancellation**, so a real failure was silent.
- The Google button **lost its VoiceOver name while signing in**; its label stays in the tree now.
  Two transcribed `opacity(0.4)` values went with it — SwiftUI already dims a disabled button.
- `GeometryReader` wrapping the `ScrollView` became `containerRelativeFrame(.vertical)`.
- A stored device GUID over 36 characters was accepted, cached and re-read forever — a permanent
  400 on every sign-in. Both bounds are checked.
- DNS, host-unreachable and TLS failures mapped to `.unknown`, so a captive portal reported
  "something went wrong" and claimed to be non-retryable.
- The 409 sign-in copy diagnosed "already signed up with a different provider", which `docs/API.md`
  does not substantiate. Removed rather than shown to a user as fact.
- `Tools/capture-fixtures.sh` truncated its target before knowing the outcome, so a 502 page or a
  dropped connection destroyed the fixture it was refreshing. It writes to a temp file, checks the
  status and that the body is JSON, and only then replaces.
- `docs/API.md` claimed `statusCode` was "absent under one name or the other in two of the three
  shapes" — it is absent from one. Its 401 comment contradicted the JSON beside it. Both fixed, and
  the 6–36 character validation bounds are now written down.
- `docs/DESIGN.md` claimed typography tokens live in `DesignSystem`; `Spacing.swift` says the
  opposite, deliberately. The doc was wrong.
- `docs/STATE.md` claimed the Keychain device GUID was tested. `Persistence` has no test target.
- The apostrophe in the app's first line was straight where the product's is typographic.
- The two sign-in buttons did not pair. The HIG for Sign in with Apple requires other sign-in
  buttons beside it to be the same size and corner radius; the Google button had a 10pt radius
  against Apple's 6, chrome sized to its label rather than a control, and at accessibility
  sizes ran to two lines and twice the height. It now takes Apple's radius, its height from
  `.controlSize(.large)`, the same weight, and caps its label's type range at `.xxxLarge` as
  Apple's own label does — the one documented exception to Dynamic Type everywhere.

Stage 1 follow-up, 2026-09-11 — all three found by running the app rather than building it:
- **The app crashed on launch.** `INFOPLIST_KEY_MindlensAPIBaseURL` resolved correctly in the
  build settings and never reached the bundle: Xcode's generated Info.plist forwards only the
  `INFOPLIST_KEY_` names on its own allowlist and drops the rest without a word. Replaced with
  a partial `mindlens/Info.plist` that Xcode merges its generated keys into, and a
  `PBXFileSystemSynchronizedBuildFileExceptionSet` so the synchronized folder does not also
  copy that file as a resource and collide with the processed one.
- `Tools/check-build-settings.sh` asserted the build setting, which was correct, rather than
  the built `Info.plist`, which was missing the key. It now reads the bundle.
- The sign-in screen broke at accessibility text sizes: the bottom panel could not scroll, and
  `SignInWithAppleButton` lost its capsule and fell under the 44pt tap target. The screen is
  one scroll view now, and the button's height is clamped to the 30–64pt range Apple documents
  and rebuilt on a text-size change, which it does not survive on its own.

Findings from an adversarial architecture review, 2026-09-10:
- Keychain wrote the token pair as two items; a tear left a new access token beside an
  already-consumed refresh token. Now one atomic item.
- `signOut()` left the in-flight refresh running, which completed afterwards and
  re-persisted live tokens for a signed-out user.
- `APIClient` was an `actor` with only `let` properties — an executor hop per request
  protecting nothing. Now a `Sendable final class`.
- All seven `String(localized:)` calls omitted `bundle: .module`, so no package string
  would ever localize. User-facing copy moved to `DesignSystem`.
- `APIClient` had no tests at all. Now covered for envelope unwrapping, the 401 retry
  path, error mapping, 204, and decoding failure.
- `PATTERNS.md` shipped a broken copy of `TokenRefresher` beside the correct code, and
  three doc snippets did not compile.
- A single global `dateDecodingStrategy` could not serve three wire formats.
- `Package.swift` exported an umbrella library — a barrel export, which `CLAUDE.md` bans.
- Cancelled loads rendered as user-facing errors.
- `SessionState` lived in a feature target that another feature would need.

Finishing the base setup, 2026-09-10:
- The app target built at `SWIFT_VERSION = 5.0` and `IPHONEOS_DEPLOYMENT_TARGET = 26.2`
  while every document claimed Swift 6 and iOS 18. `Config/Base.xcconfig` existed with the
  right values and was referenced by nothing.
- `API_BASE_URL` resolved to `https:mindlens-api-production…` — an xcconfig treats `//` as
  a comment inside values, so the variable meant to hold the scheme separator was empty.
- `StubURLProtocol`'s force-unwraps were suppressed for SwiftLint but not swift-format, so
  a full-tree format lint failed. Tests now share one `StubURLProtocol.url` instead of
  each building their own.
- Removed a duplicate `docs/` and `Config/` tree under `mindlens/`. The docs copy was a
  pre-trim snapshot and was staged for commit.
