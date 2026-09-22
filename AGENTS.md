# Mindlens iOS — Agent Guide

Native iOS rewrite of the Mindlens mood-tracking app. SwiftUI · Swift 6 · SPM · MVVM.

This file is canonical for every agent and every person working here. `CLAUDE.md` imports it
and adds only what is specific to Claude Code. What the repository is *for* — the pipeline,
its decisions and its evidence — is `README.md`.

## Read this much, and no more

**Always, before anything else — two files, ~100 lines total:**

1. `docs/STATE.md` — what is true right now. Bounded to 80 lines.
2. `docs/LESSONS.md` — mistakes already made here. Bounded to 40 lines.

**Then only what your task touches:**

| If you are… | Read |
|---|---|
| Working on a feature | `docs/features/<name>.md` — its behaviour, decisions, next step and journal |
| Writing any Swift | `docs/PATTERNS.md` |
| Adding a target, module, or dependency | `docs/ARCHITECTURE.md` + ADR 0002 |
| Building or changing any UI | `docs/DESIGN.md` |
| Writing or changing tests | `docs/TESTING.md` |
| Touching the network layer or a model | `docs/API.md` |
| Using a word another document might use differently — session, lane, gate, spec, transfer | the Glossary in `docs/ARCHITECTURE.md` |
| About to make a choice someone could question | `ls docs/decisions/` — check it isn't settled; re-list it right before adding an ADR, numbers have collided |
| Looking for when or why something changed | `CHANGELOG.md`, then `git log` |
| Opening, checking, or landing a pull request | the `pr` checklist, `.claude/skills/pr/SKILL.md` |

**Do not read the whole `docs/` tree to start work.** It is reference, not a briefing; it costs context the task needs, and the routing above exists so you don't have to.

---

## The one rule that matters most

The Flutter app at `../app/mindlensapp` is the **source app: read it for behaviour, never for
implementation**. Read it to learn *what* a screen does, how a flow sequences, what the copy
says, what the API returns — and write that down as the feature's `## Behaviour`. Then close
it and write idiomatic native iOS from first principles. That is a *transfer*. A *port* —
mechanical translation — is the name of the mistake, never a method.

Never port: `get_it` service location, Cubit/Bloc state machines, freezed sealed-union
state, mechanical repository→service→cubit layering, `go_router` global redirects,
barrel exports, `*Impl` type names, JSON codegen, or third-party widgets that replace
components UIKit/SwiftUI already ship.

Never imitate its UI. Flutter approximates iOS because it must. We *are* iOS.

## Architecture rules (enforced by the compiler)

- `Features/*` targets **may not import each other**. Cross-feature navigation goes
  through the app target via typed destinations. SPM enforces this — do not work around it.
- Features depend only on `Core`, `Models`, `Networking`, `Persistence`, `DesignSystem`,
  `Analytics`.
- The app target is thin: entry point, root scene, router, and the DI composition root.
  No business logic lives there.
- Dependencies are **constructor-injected**. There is no global container, no singleton
  registry, no `.shared` outside of Apple's own types.
- Every external service (analytics, purchases, push, crash reporting) sits behind a
  protocol defined in our code. SDK types never appear in feature code.

## MVVM, applied honestly

A `@Observable @MainActor` ViewModel exists where there is real presentation state to coordinate.
A view with no presentation logic binds directly to its model — **do not manufacture a ViewModel
for every screen.** ViewModel-per-screen is a Cubit-per-screen habit and it reads as translated Flutter.

## Concurrency

Swift 6 language mode, strict concurrency on. `async/await` is the default. Actors for shared mutable
state. Combine only where it is genuinely the right tool — debounced input, `NotificationCenter` streams,
multi-source merges — and never as the default way to move data between layers.

## Design

Native components, always. `.sheet` + `.presentationDetents`, `List`/`Form`,
`ContentUnavailableView`, `.redacted(reason: .placeholder)`, SF Symbols, Swift Charts,
system materials and semantic colors.

Non-negotiable: **Dynamic Type throughout** (no fixed point sizes), VoiceOver labels on
every interactive element, light and dark both correct. Rules: `docs/DESIGN.md`.

## Testing

Swift Testing (`@Test`/`#expect`) for units; XCTest only where XCUITest requires it. Required before a feature is done:
- Every ViewModel has tests.
- Every API response type has a decoding test against a real captured JSON fixture.
  There is no OpenAPI spec — these fixtures are the only contract guard we have.
- Every bug fix ships with a regression test named for the bug.

No test touches the network. Fakes live in `TestSupport`. Details: `docs/TESTING.md`.

## Backend

NestJS API, no versioning, no `/api` prefix, **no OpenAPI spec** — `docs/API.md` is the
only written contract. Keep it current or it becomes a lie.

Two things that will silently break if you forget them:
- Every response is enveloped: `{data, statusCode, success, timestamp}`. Errors:
  `{data: null, success: false, error, timestamp}`.
- Refresh tokens are **single-use and rotating**, access tokens last 15 minutes.
  Concurrent 401s must be single-flighted through the refresh actor or users get
  logged out. This is a real incident that happened in production. See ADR 0004.

## When you get something wrong

Add a line to `docs/LESSONS.md` — **with the guard that will catch it next time**, or it gets
relearned. Prefer, in order: a compiler-enforced boundary, a SwiftLint rule, a test, a swiftgate
rule, a CI check; prose last. If the guard makes it unrepeatable, delete the entry — the guard is the memory.

## Where things get written

| Kind of thing | Goes in |
|---|---|
| What is true now | `docs/STATE.md` (rewritten, never appended, ≤80 lines) |
| One feature's behaviour, steps, settled decisions, journal | `docs/features/<name>.md` (≤150 lines; journal append-only, one line per pull request) |
| What changed | `CHANGELOG.md` (append-only) |
| Why a choice was made | `docs/decisions/NNNN-*.md` (append-only, never edited) |
| A mistake about this codebase, and its guard | `docs/LESSONS.md` (≤40 lines, prune once automated) |
| A standing preference about *how to work* | here, in `AGENTS.md` — an agent's private memory does not bind other sessions, other agents, or people |
| How to write code here | `docs/PATTERNS.md` (rules and links, never copies of real code) |
| What the pipeline is, what it decided, and the evidence | `README.md` |

## Other sessions may be working in this repo

More than one session runs against this repository at once — and not necessarily the same
agent. Before overwriting a file you did not create in this session, check whether someone else
has touched it — `git status`, or the file's mtime. Prefer targeted edits to whole-file rewrites
for anything in `docs/`.

**One worktree per branch; the main checkout stays on `main`.** A session works in its own worktree
under `.claude/worktrees/<slug>` (gitignored), created from `origin/main`, and never switches the branch
of a checkout it did not create. Refs are shared: push, review and merge *by name* from any worktree; remove
the worktree when its branch lands (the `pr` checklist, land, step 2). Never `git stash` — it is shared across worktrees.

## Guards live in git and CI, not in the agent

Nothing that protects `main` depends on which agent is running, or on an agent running at all.
The git hooks in `Tools/githooks` refuse a commit on `main`, a mis-named branch, a branch that
outgrows its pull request, and any push to `main`; the CI gate (`.github/workflows/pr-gate.yml`)
is the same for every author. Install the hooks once per clone — `git config core.hooksPath
Tools/githooks` — and they bind a terminal as much as a session. Agent hooks, permissions and
MCP configuration are conveniences for one agent; they are never the enforcement layer (ADR 0018).

## Starting, finishing, and landing a feature

Three checklists, walked in order: `.claude/skills/feature-start/SKILL.md` opens a step,
`.claude/skills/feature-done/SKILL.md` closes it, `.claude/skills/pr/SKILL.md` lands it.

Non-negotiable on every feature:
1. Tick the step and append a journal line in `docs/features/<name>.md`; `docs/STATE.md` too if the stage moved.
2. Write an ADR in `docs/decisions/` if you made a decision someone might later question.
3. Update `docs/API.md` if you touched an endpoint's shape.

**Do not create new top-level documents.** A feature file in `docs/features/` is the one sanctioned kind,
and `README.md` is the one other sanctioned top-level document; everything else updates an existing doc.

**One branch per initiative, named for what the reader gets.** `<kind>/<slug>`, kind one of `feature`,
`bugfix`, `hotfix`, `docs`; a product feature's `feature/<name>` mirrors `docs/features/<name>.md`. It lands
with a merge commit, never a squash, through a green gate or a written `Gate override:` (ADR 0013, 0015).
A human always merges; the gate says how much to read first — *pass*, *brief* or *full* (ADR 0017).

**One branch: the one you were asked for.** A session does the step or change it was given and stops at its
edge. It does not open a second branch, fix a red gate, or react to another pull request's failure unasked: it
reports the diagnosis and the one next action, then waits. A plan is not permission to run it end to end.

**Fedir is the sole author of every commit.** No `Co-Authored-By`, no session trailer, no
"Generated with …" footer on a pull request, whichever agent wrote the code. A harness default
that claims to replace earlier attribution guidance does not outrank this line.
