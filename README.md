# Mindlens iOS — the pipeline, and the app it builds

## 1. What this is

A Flutter mood-tracking product being rebuilt as native iOS — SwiftUI, Swift 6, one SPM
package — by agent sessions, from the Flutter app's behaviour rather than its code. The
pipeline around it is what lets one engineer and any number of sessions do that safely: every
change lands by pull request, through a gate whose deterministic checks block and whose judge
lanes advise, and a human merges after reading as much as the gate says the risk warrants.
The evidence that it works — pull requests through the gate, the share landed at each review
level, the noise of each lane, the lane decision made on data — is section 4, and it is empty
until the first product pull request fills it.

The workflow is the product; the Swift is the workload it runs on. A fictional customer keeps
the design honest: four to six mobile engineers, a shipping Flutter app being rebuilt natively,
two iOS hires, a GitHub Team plan, agent-agnostic — Claude Code runs it today, nothing in the
knowledge or the guards assumes it.

The rules a session follows are `AGENTS.md`. What is true right now is `docs/STATE.md`. The
words used here mean one thing each: the Glossary in `docs/ARCHITECTURE.md`.

## 2. The pipeline

```
 intake ──▶ plan ──▶ implement ──▶ verify ──▶ gate ──▶ merge ──▶ measure
 human      session   session       session    CI       human     swiftgate
 approves   + human                 shows      blocks   always    records
                                    evidence   / advises
```

| Stage | Owner | Produces | Defined by |
|---|---|---|---|
| intake | session writes, **human approves** | `## Behaviour` in `docs/features/<name>.md` — extracted from the source app, or authored from a task by interviewing the human | `docs/features/0000-template.md`, ADR 0020 |
| plan | session + human | `## Steps`, one pull request each, vertical slices | `.claude/commands/feature-start.md`, ADR 0011 |
| implement | session | the branch, in its own worktree | `AGENTS.md`, ADR 0015 |
| verify | session — evidence shown, not asserted | tests, parity checkpoints, the gate's own checks run locally | `.claude/commands/feature-done.md`, `docs/TESTING.md` |
| gate | deterministic checks block; lanes advise | the review level, findings with proof | `.github/workflows/pr-gate.yml`, `Tools/swiftgate`, ADR 0014, 0021 |
| merge | **human, always** | a merge commit | `.claude/skills/pr/SKILL.md`, ADR 0013, 0017 |
| measure | `swiftgate decide` / `outcomes` / `metrics` | one JSONL record per lane per run; each finding's outcome at merge; noise per lane | `.github/workflows/pr-outcomes.yml`, ADR 0016 |

No planner agent: planning is where understanding is made, and it is the stage this pipeline
keeps a human in (ADR 0019).

## 3. Decisions, by area

Ten areas, matching the September 2026 research this pipeline was checked against. Each:
what teams document, what this repository does, and the incident here that made it so. Sources
are listed at the end of the section; dates are the sources' own.

### 3.1 Git — who commits, which branch, how it lands

**What teams do.** Agents commit, push and open pull requests; humans merge — Stripe (1,300+
agent pull requests a week, "human-reviewed, but containing no human-written code", 2026-02),
Shopify (2026-07), Sentry (2026-08), Cognition (2026-02), Ramp (2026-01). The one documented
exception is Anthropic, where an internal agent merges "more than half of all code" and a
"risk-weighted sample is reviewed by humans" after the fact (2026-07-21). Ona treats fewer than
1,000 changed lines as one of six low-risk criteria (2026-04-09). Worktrees are the isolation
unit for parallel sessions (Claude Code docs; Shopify 2026-09-10).

**This repository.** Sessions commit, push and open pull requests; a human merges, always, with
a merge commit. One branch per initiative, `<kind>/<slug>`, in its own worktree; pull requests
capped at 1,000 changed lines and 10 commits nobody has seen, each cap with a recorded override
(ADR 0013, 0015; the caps sit at 4,000 and 25 until the gate branch lands them). No agent
trailer on any commit — Fedir is the sole author.

**Why.** A branch reached twenty commits with no pull request and re-diverged from `main` within
two hours of merging it (`docs/LESSONS.md`). ADR 0006 claimed the gate was "wired to branch
protection"; protection is a 403 on this plan, and nothing had ever stood between `git push` and
`main` — so the guards moved into git hooks and a pre-push refusal.

### 3.2 CI with agents — judge lanes, blocking, noise

**What teams do.** Every vendor ships LLM review advisory by default and blocking by opt-in:
Claude Code Review's check "always completes with a neutral conclusion"; Bugbot defaults to
`neutral`; Codex review posts only P0/P1; Copilot review can approve, off by default and
path-scoped (2026-09-01). Anthropic's internal reviewers are "scoped to a specific, narrow
focus", and requiring each "to write a proof that their finding is valid" took the substantive-
comment rate from 16% to 54% (2026-07-21). Shopify runs verifiers on a different model than
hunters (2026-07-29). No vendor publishes a precision number.

**This repository.** Three narrow lanes — `verification`, `idiom`, `spec` — each behind a
deterministic evidence gate, each returning a schema-validated verdict, aggregated by a scorer
that alone decides (ADR 0014). Every run leaves a record and every merge classifies each finding
`changed`, `resolved`, `waived` or `untouched`; noise is `untouched ÷ classified` (ADR 0016).
Lanes are advisory until a reviewed pull request grants `blocks: true` on ≥10 judged pull
requests, noise ≤30% and ≥1 finding `changed`; a lane at >50% noise and 0 `changed` after 10 runs
is killed the same way; every finding carries a proof or is not reported (ADR 0021 — the
mechanism lands with the gate branch).

**Why.** The first reviewer "succeeded" on an empty Flutter checkout and the gate reported
*Passed*: a judge that could not look was scored as a judge that found nothing. The evidence
gate and `CANNOT_EVALUATE` came from that. Then 5,576 lines of gate were built faster than they
were read — two subcommands nothing invoked, nine rules no test covered — and the cleanup left a
rule: a rule ships with a test or not at all.

### 3.3 Human review — every pull request, or by risk

**What teams do.** Stripe, Cognition, Ramp and GitHub's Copilot docs read every agent pull
request. Anthropic tiers codebases by risk. Ona's agent approves pull requests meeting six
objective criteria — under 1,000 lines, no migrations, infrastructure, auth or audit code —
"but a person always clicks the merge button"; median lead time went from 4.1 h to 1.1 h
(2026-04-09).

**This repository.** A human always merges; the gate assigns how much to read first — *pass*
(≤200 lines, no risk path, no new ADR, dependency or entitlement: merge on the gate's word),
*brief* (the gate's comment and the body), *full* (the diff: any risk path, any lane finding at
warning or above with a proof, or an oversized pull request). The risk class is a path list in
`.github/swiftgate.yml` (ADR 0017; lands with the gate branch — every pull request is *full*
until then).

**Why.** One reviewer, and every product pull request session-written. Reading every diff is the
bottleneck; reading none is the incident.

### 3.4 Guardrails — which layer holds which guard

**What teams do.** Three layers, named by every vendor: advisory instructions (`CLAUDE.md`,
`AGENTS.md`), deterministic agent hooks ("Unlike CLAUDE.md instructions which are advisory, hooks
are deterministic" — for that agent), and the agent-neutral floor of rulesets, required checks
and CODEOWNERS. Hooks, permissions and MCP configuration do not transfer between agents
(`.claude/settings.json` ≠ `.codex/hooks.json` ≠ `.cursor/hooks.json`). Credentials and egress
live in the VM or proxy, never the agent (Anthropic, Stripe, Ramp).

**This repository.** A guard that protects the repository is installed by git (`Tools/githooks`)
or run by CI (`.github/workflows/pr-gate.yml`); agent hooks and permissions are convenience and
may duplicate a guard for a faster refusal, never hold one alone. Rulesets are named as the floor
this plan lacks (ADR 0018).

**Why.** The inventory found the git hooks installed only by the Claude session-start hook: a
terminal, or another agent, had no commit cap and no refusal to push `main`. A pre-commit cap was
also silently off in the case it existed for — an empty bash array under `set -u` — and the
lesson generalised: a guard's error path fails closed or prints, never defaults to "fine".

### 3.5 Starting a feature — what artifact, who writes it

**What teams do.** Plan first is the vendor default ("if you could describe the diff in one
sentence, skip the plan"); for larger work, interview the human and write a self-contained spec,
then start a fresh session on it (Claude Code best practices). Copilot researches and plans on a
branch before any pull request (2026-04-01). Spec Kit and Kiro formalise spec → plan → tasks as
files. Shopify's migration had subagents document each feature's behaviour and prepare plans,
which engineers reviewed before implementation (2026-09-10).

**This repository.** One file per feature is the spec: a `## Behaviour` section — flows, screens,
copy, API calls, edge cases, acceptance criteria as checkable lines — approved by a human before
the first step opens; then `## Steps`, two or three ahead, one pull request each, picked up cold
by the first that is not ✅ (ADR 0011, 0020). Behaviour is extracted from the source app for a
transfer or authored from a task for a new feature, and the section reads the same either way.

**Why.** The first feature's screens were written after its code, and the spec lane was holding
pull requests to `ready` lines the implementing session had written itself.

### 3.6 Finishing a feature — done, docs, decisions

**What teams do.** Evidence over assertion: "the test output, the command it ran and what it
returned, or a screenshot"; Stripe allows an agent two CI runs before a human looks (2026-02).
Claude Code Review flags `CLAUDE.md` statements a pull request makes stale. Nobody documents
ADRs or a changelog as an agent definition-of-done step.

**This repository.** `/feature-done` runs the tests, the settings check, the linters and the
doc check locally before a pull request exists; the step is ticked and the journal gets one line
per pull request; an ADR is written for any choice someone could later question; a lesson gets a
row only with the guard that catches its recurrence. `docs/STATE.md` is rewritten to what is true
now, and a Stop hook re-prompts a session that moved code without it.

**Why.** `** BUILD SUCCEEDED` was once treated as verification: the app compiled, 75 tests passed,
and it died on the first line of its composition root. The gate now launches the app, at the
default and the largest accessibility text size. A build setting resolved perfectly and never
reached the bundle — so the check reads the *built* `Info.plist`, not the setting.

### 3.7 Test-first, or a verification loop

**What teams do.** Test-first is advocated (Simon Willison's red/green; Spec Kit's constitution
makes it non-negotiable) but no company account reports it as standard; the dominant documented
practice is "give the agent a check it can run", with test-first specifically for bug fixes.
Agents modify tests in 23% of commits versus 13% for humans and add mocks in 36% versus 26%
(MSR 2026), so mocking guidance in the instruction file is the one empirically motivated rule.

**This repository.** A verification loop with specific tests required: every ViewModel, every
response type against a *captured* fixture, every bug fix with a regression test named for the
bug. The verification lane's question is whether the tests reach the production path — fakes that
throw what production throws, concurrency tests that stagger rather than race, fixtures captured
rather than typed (`docs/TESTING.md`, `.claude/skills/lane-verification/SKILL.md`).

**Why.** A concurrency test that raced the actor never called the transport at all, and passed
against two real `TokenRefresher` bugs while claiming to cover them. A fake threw a type
production never throws, so two green tests covered nothing. Both are in `docs/LESSONS.md`; the
first has a helper in `TestSupport` as its guard, the second is the lane's first rubric line.

### 3.8 More than one agent on one repository

**What teams do.** `AGENTS.md` is the shared instruction file, stewarded by the Agentic AI
Foundation, read by Codex, Copilot, Cursor, Devin and others; Claude Code reads it natively only
when no `CLAUDE.md` exists, or through an `@AGENTS.md` import. Skills are portable
(agentskills.io); skill *directories* are not — Codex scans `.agents/skills/`, Claude Code only
`.claude/skills/`, Cursor, Copilot and Devin both. Hooks, permissions, MCP config and path-scoped
rules transfer nowhere.

**This repository.** `AGENTS.md` is canonical; `CLAUDE.md` is the import plus Claude-only lines.
Skills live in `.claude/skills/` with `.agents/skills` a symlink to it; a skill only a human
should run says so in its frontmatter and its first paragraph (ADR 0023). The lane runner is
`claude-code-action`, the scorer consumes only the verdict schema, and the model is a parameter.

**Why.** Session-to-session, not agent-to-agent, so far: two sessions created ADR 0006 at once;
one rewrote a file another was editing. The rules those left — re-list the ADRs before numbering,
check `git status` before overwriting, one worktree per branch — hold for a second agent as well.

### 3.9 Memory — what is always on, what is on demand

**What teams do.** Every vendor caps the always-on file (Claude Code: under 200 lines; Codex:
32 KiB; Cursor: 500 lines; Copilot: "2 pages") and describes progressive disclosure: a tiny
always-on file, path-scoped rules, skills on demand. Lessons are documented as an instruction-file
append habit ("anytime we see Claude do something incorrectly we add it to the CLAUDE.md"). Vercel
measured an 8 KB docs index inside `AGENTS.md` at 100% against skills at 79% (2026-01-27).

**This repository.** `AGENTS.md` ≤200 lines and mostly a routing table; two files always
(`docs/STATE.md` ≤80, rewritten in place; `docs/LESSONS.md` ≤40, a guard per row, rows deleted
once the guard is automated); one reference file per task; one file per feature ≤150 lines;
ADRs and the changelog append-only. Caps enforced by `Tools/check-doc-links.py`. An agent's
private memory binds nobody (ADR 0022).

**Why.** The server repository's status file reached 2,000 lines and stopped being read. A doc
held a second copy of `TokenRefresher` and drifted to the broken version. A duplicate `docs/` tree
survived a session unnoticed. Each is a row in `docs/LESSONS.md` with a mechanical guard.

### 3.10 Transferring a feature between platforms

**What teams do.** Every first-party port used the old code as the reference plus a parity
harness: Anthropic's migrations accept when the old test suite passes in full (2026-07-16);
Shopify's React Native → SwiftUI/Kotlin move had subagents document behaviour, prepare platform
plans and check parity at named checkpoints with screenshots and event windows; "native expertise
remained essential" (2026-09-10). Nothing Flutter → SwiftUI exists in first-party form.

**This repository.** A *transfer*: behaviour extracted from the source app into the feature file,
approved, then re-derived in this platform's idiom with the Dart as reference only, verified by
parity checkpoints — screen copy, API calls made, navigation sequence. A *port* is the name of the
mistake. The idiom lane judges the Swift against exemplars from this repository, not against the
Dart (ADR 0014, 0020).

**Why.** The failure ADR 0006 was written for: a diff that builds, passes tests, and is Dart with
Swift syntax — a Cubit renamed to a model, a freezed union renamed to an enum. The repository's
own incidents so far have been correctness, not Flutter-isms, which is why the idiom lane is the
one expected to fail its grant (ADR 0021).

### Sources

Vendor docs (accessed 2026-09-21): Claude Code best practices, memory, hooks, skills, permission
modes, GitHub Actions and Code Review pages at code.claude.com; OpenAI Codex AGENTS.md, skills,
hooks and GitHub review pages; agents.md; agentskills.io; GitHub Copilot cloud agent and code
review docs, rulesets docs, and changelogs of 2025-11-13, 2026-03-13, 2026-04-01, 2026-08-27,
2026-09-01; github/spec-kit; kiro.dev; Cursor Bugbot, rules, skills and hooks docs.
First-party accounts: Anthropic, "How Anthropic secures its AI-native software development
lifecycle" (2026-07-21) and large-scale code migration (2026-07-16); Boris Cherny (2026-01-02);
Stripe, Minions parts 1–2 (2026-02-09, 2026-02-19); Shopify, Shop app migration (2026-09-10)
and agentic harness (2026-07-29); Vercel, AGENTS.md vs skills evals (2026-01-27); Sentry
(2026-08-06); Cognition (2026-02-27); Ramp (2026-01-12); Ona (2026-04-09); Uber (2026-08-27);
Simon Willison, red/green TDD. Studies: Yoshimoto et al., arXiv 2603.13724 (2026-03); Hora &
Robbes, MSR 2026, arXiv 2602.00409 (2026-01).

## 4. Evidence

**No records yet — the first Swift pull request fills this.** Every number below is produced by
`swiftgate metrics` and `swiftgate outcomes` from the JSONL records the gate writes (ADR 0016);
none is typed by hand. The bar for calling the pipeline done: twenty product pull requests through
the gate; every lane with a measured noise rate; one lane granted, refused or killed on data; one
defect the gate caught that a full read would have missed, documented; behaviour-first intake run
on three features.

### Pull requests through the gate

### Share landed at each review level

### Noise per lane

### The lane decision made on data

### Cost per pull request

## 5. Install this in your repo

**Copy as is** — none of it knows which agent is running:

- `Tools/githooks/` and `git config core.hooksPath Tools/githooks` once per clone: no commit on
  `main`, the branch kinds, the commit cap, the override trailer, no push to `main`.
- `Tools/swiftgate/` (Go, no dependencies beyond the standard library), `.github/swiftgate.yml`
  and `.github/workflows/pr-gate.yml` — the rule catalogue, the evidence gates, the scorer, the
  records. `Tools/check-pr-conventions.sh` for the branch and size rules.
- `Tools/check-doc-links.py` and the `docs/` shape it enforces: `STATE.md`, `LESSONS.md`,
  `features/`, `decisions/`, the caps.
- `AGENTS.md`, `.github/PULL_REQUEST_TEMPLATE.md`, the skills under `.claude/skills/` with the
  `.agents/skills` symlink.

**Claude Code–specific**, and what replaces it elsewhere:

- The lane runner is `anthropics/claude-code-action`, billed to a subscription through
  `CLAUDE_CODE_OAUTH_TOKEN` (ADR 0012). Another runner needs to read `.swiftgate/run/context.md`
  and write the lane's verdict as JSON matching the schema `swiftgate prepare` publishes; the
  scorer never sees which runner did it.
- `.claude/settings.json` hooks and permissions, `.claude/hooks/`, `.claude/commands/`: each is a
  convenience with a git or CI guard behind it (ADR 0018). Drop them and nothing is unprotected.
- `CLAUDE.md` is an import plus a few lines; a Codex or Cursor clone reads `AGENTS.md` directly.

**What a ruleset should enforce**, on a plan that has them — this one does not, and the hooks
are the substitute: require a pull request before merging; require the `gate` check; forbid
force pushes; allow merge commits only; require one approval, from someone other than the last
pusher, once there are two people.

## 6. Study

How a pull request flows through the gate on `main` today, then every component with the
incident it answers and what measures it. A row with nothing in the measurement column is a
cleanup candidate.

### How a pull request flows

1. **Before a commit exists.** A session start — or, by hand, `git config core.hooksPath
   Tools/githooks` — installs the hooks. `pre-commit` refuses a commit on `main`, a branch not
   named `feature|bugfix|hotfix|docs/…`, a branch with 25 commits in no pull request, and a
   commit onto a pushed branch another open pull request already contains unless an override
   reason is given; `prepare-commit-msg` writes that reason into the commit as an `Override:`
   trailer; `pre-push` refuses `main` and any non-fast-forward.
2. **Opening.** The `pr` checklist checks a clean tree, the branch name, authorship, that `docs/`
   moved, ADR numbers against every open pull request's head, then pushes and creates the pull
   request from the template. Nothing enforces the checklist; the gate checks what it can.
3. **The workflow fires** on open, push, reopen, ready-for-review and label changes — the last so
   an override re-runs the gate — one run per pull request, the previous cancelled. Three jobs run
   in parallel; `gate` waits on them.
4. **`build`** (macOS): the newest Xcode, a booted simulator, `xcodebuild test` on the package, a
   build of the app, the launch smoke test at default and largest text size — only when the change
   can break a launch, with retries — then `Tools/check-build-settings.sh` on the *built* product,
   SwiftLint pinned by checksum and swift-format, both strict, over sources and tests.
5. **`readiness`** (Linux): `go test` and a build of the gate; `swiftgate prepare` diffs against
   the merge base, picks up to three exemplar files per changed Swift file, asserts each lane's
   evidence, runs the twelve static rules over added lines only, and writes the brief. No Swift
   changed: it writes *Skipped* and stops.
6. **The token guard** compares the OAuth token to its trimmed self before any lane runs.
7. **Three lanes**, each `claude-code-action` pinned to a commit, `continue-on-error`, `Read Grep
   Glob` only, forty turns, returning `{verdict, summary, findings}` validated against the schema
   `prepare` published. A lane that did not run leaves no file.
8. **`swiftgate decide`** writes one metrics record per lane, then scores: a static blocker, a lane
   `BLOCK`, or a lane `CANNOT_EVALUATE` — evidence missing, judge did not run, output off the
   contract — blocks. The `gate-override` label with a `Gate override:` body line lifts it and is
   copied into the comment. One sticky comment per lane and one for the scorer, each stamped with
   the head SHA; inline comments on added lines; findings and records uploaded as artifacts.
9. **`conventions`** (seconds): branch kind, ADR collisions against `main` and every open head, the
   size cap with `size-override`, and `Tools/check-doc-links.py`.
10. **`gate`** fails if any of the three is not `success`. It is the one name a ruleset would
    require; on this plan it binds through the human who lands and the pre-push hook.
11. **Landing.** `/pr land` merges with a merge commit, never a squash, and deletes the branch.
    `.github/workflows/pr-outcomes.yml` downloads the run's records and `swiftgate outcomes`
    classifies each finding against the merged head. `swiftgate metrics` sums the records.
12. **What has exercised the judge half:** the single-lane reviewer on pull requests #1 and #2.
    Every pull request since the three lanes landed changed no Swift. The three-lane design is
    proven by its own tests and nothing else — which is why ADR 0021 makes it advisory first.

### Components

| Component | Layer | Purpose | Incident or ADR | Measurement |
|---|---|---|---|---|
| `Tools/githooks/pre-commit` | git | Refuse a commit on `main`, a mis-named branch, 25 commits in no pull request, a commit into another open pull request without a reason | `base-setup` at twenty commits; the cap silently off under bash 3.2 `set -u`; ADR 0013, 0015 | `Override:` trailers in `git log` |
| `Tools/githooks/prepare-commit-msg` | git | Write the override reason into the commit | An escape hatch whose only record was the terminal | Same |
| `Tools/githooks/pre-push` | git | Refuse `main` and any non-fast-forward | ADR 0006 claimed protection that was a 403 | none |
| `.claude/skills/pr/SKILL.md` | skill | Open, diagnose, land; refuse a red gate without a written override; merge commit only | PR #1 landed on a false "protected" claim; ADR 0013 | none |
| `.github/PULL_REQUEST_TEMPLATE.md` | config | What was run outside the gate, what running it found, the docs that moved, the override line | ADR 0013 | none |
| `build` › simulator boot | CI | Boot before the first test | Launch test booted the simulator itself: 73–928 s, 3 of 5 runs failed | none since |
| `build` › tests and app build | CI | The suite and the app compile and pass | foundational | `docs/STATE.md` test count |
| `build` › launch smoke test | CI | Launch at default and largest text size, when a launch-relevant path changed | `** BUILD SUCCEEDED` treated as verification; the app died at its composition root | pass/fail |
| `Tools/check-build-settings.sh` | CI | Resolve the real build settings; read the built `Info.plist` and the binary's entitlements | Unreferenced xcconfig; `X = //`; `INFOPLIST_KEY_` dropped; `codesign` empty on simulator; no `DEVELOPMENT_TEAM`; ADR 0009 | Shown to fail on a renamed key before trusted to pass |
| `build` › SwiftLint + swift-format | CI | Correctness and formatting, strict, over sources *and* tests; custom rules for `bundle: .module` and `didSet { Task }` | A force-unwrap passed because tests were not linted; a string that never localized; overlapping loads | none |
| `Tools/check-doc-links.py` | CI | Links resolve; caps (STATE 80, LESSONS 40, AGENTS 200, features 150); one home per document; one `docs/` tree; feature status = ledger; fail on zero documents | A dead `memory/MEMORY.md` in the Flutter repo; a duplicate `docs/` tree; the skip list matched an absolute path inside a worktree and checked nothing | Prints the count of documents checked |
| `Tools/check-pr-conventions.sh` | CI | Branch kind; ADR number against `main` and every open head; size cap with `size-override` | ADR 0006 and 0010 collided; ADR 0013, 0015 | Prints branch, ADR, size per run |
| `swiftgate prepare` | gate | Merge-base diff; exemplars; evidence per lane; static rules on added lines; the brief | An empty Flutter checkout scored *Passed*; ADR 0012, 0014 | `main_test.go` replays it |
| Static rules (twelve) | gate | `ObservableObject`, service locators, `.shared`, feature-imports-feature, third-party imports, fixed fonts, hardcoded colours, `Date()` in features, Dart file names, GCD queues, `print`, `*Widget`/`*Cubit` names | ADR 0006; each cites the document that gives it authority | `TestEveryRuleHasATest`; `swiftgate metrics` lists rules that never fired |
| Token guard | CI | Compare the OAuth token to its trimmed self; never print it | A leading space, a hidden 401, three runs dead at turn 1 | none |
| Lanes: `verification`, `idiom`, `spec` | gate | One question each, evidence-gated, structured output; skills under `.claude/skills/lane-*/` | ADR 0014: one reviewer doing nine things; 0021: advisory until earned | **never fired on a real diff**; one record per run when they do |
| `swiftgate decide` | gate | The scorer; records; override; comments; artifacts | ADR 0014, 0016 | Score truth table in `verdict_test.go`; the records |
| `.github/workflows/pr-outcomes.yml` → `swiftgate outcomes` | measure | Classify each finding `waived → changed → resolved → untouched` at merge | ADR 0016; ADR 0006's unmeasured override ratio | Is the measurement; fixtures only so far |
| `swiftgate metrics` | measure | Sum the records: verdicts, cost, noise per lane, fires per rule, rules that never fired | ADR 0016 | Is the measurement |
| `.claude/hooks/session-start.sh` | agent | Install the hooks, refresh the open-PR cache, print orientation, warn when the main checkout is off `main` | Stale numbers from a checkout on the wrong branch | none — convenience (ADR 0018) |
| `.claude/hooks/state-doc-reminder.sh` | agent | Re-prompt when code moved and no state or feature file did | A Swift 6 claim over a Swift 5 build | none — convenience |
| `.claude/settings.json` deny list | agent | Refuse force pushes, pushes to `main`, reading secrets — for the agent | Duplicates the hooks and `.gitignore` | none — convenience |
| `.claude/commands/feature-start.md`, `feature-done.md`, `orient.md`; `.claude/skills/study/` | skill | Open a step, close it, orient, learn | ADR 0011, 0023 | none |
