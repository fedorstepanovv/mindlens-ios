<!--
Title: the behaviour change, present tense — the same rule as a commit subject.
Everything the gate can check, it checks. This body is for what it cannot.
Delete any section that does not apply. No attribution footer of any kind.
-->

What this changes and why now. If it is a feature step, name it: `docs/features/<name>.md`, step N.

**Ran outside the gate.** What was run on a device or simulator that CI cannot — a real
sign-in, both appearances, the largest text size — or "nothing beyond the gate".

### Fixes found by running it

One line each. Every one is a candidate for `docs/LESSONS.md`, with its guard.

### Docs that moved

Every pull request moves at least one. Keep the lines that apply.

- `docs/features/<name>.md` — step N ticked, journal line appended
- `docs/STATE.md` — stage, next action, known gaps
- `docs/decisions/NNNN-*.md` — new ADR; the directory and every open PR's head re-listed first
- `docs/API.md` — an endpoint's shape
- `CHANGELOG.md`

### Notes

What was deliberately left out, what you are unsure about, what to read first.

<!-- To merge past a blocker: add the `gate-override` label AND a line here reading `Gate override: <the reason>`. The gate copies the reason into its comment. -->
