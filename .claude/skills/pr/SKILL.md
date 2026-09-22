---
description: Open, check, or land a pull request the way this repo does it — one branch per feature step, a green gate, Fedir the only author
argument-hint: [open] | status | land
disable-model-invocation: true
---

The third bookend: `/feature-start` opens a step, `/feature-done` closes it, `/pr` lands it.
This skill pushes to `origin` and merges into `main` — both outward-facing — so it runs only
when Fedir invokes it, and every mode stops and reports rather than working around a failed
check. The rules are ADR 0013; the body it fills is `.github/PULL_REQUEST_TEMPLATE.md`.

## Standing rules, every mode

- Never on `main`. Never `--force`. Never rewrite history that has been pushed — if a commit
  needs changing, say which and why, and let Fedir decide.
- The branch is `<kind>/<slug>`, kind one of `feature`, `bugfix`, `hotfix`, `docs`, named for what
  the reader gets. A product feature's `feature/<name>` has a `docs/features/<name>.md`; a
  `feature/` branch without one is tooling, and the spec lane says so and skips (ADR 0015).
- Fedir is the sole author. No trailer on any commit, no footer on the pull request.
- A red gate is not merged. The only way past a blocker is the `gate-override` label plus a
  `Gate override: <reason>` line in the body — Fedir's decision, spoken in this session, never
  inferred from a previous one.

## Mode: open — `/pr` or `/pr open`

1. **Where am I.** `git status --porcelain` is empty; `git branch --show-current` is not
   `main` and matches the naming rule. Anything else: stop and say so.
2. **`/feature-done` was walked.** If this session did not run it, run its steps 1, 2 and 6
   now — `swift test`, the linters, `python3 Tools/check-doc-links.py`. Do not open a pull
   request the gate will fail for a reason a local command would have caught.
3. **Sync with `main`.** `git fetch origin` then `git merge origin/main` — a merge, not a
   rebase, so the SHAs Fedir has already read stay what they are. A conflict in
   `docs/STATE.md` is resolved by rewriting the paragraph to what is true now, never by
   picking a side.
4. **Authorship.** `git log origin/main..HEAD --format='%an <%ae>%n%(trailers)'` — every
   author is Fedir and every trailer block is empty. If not, stop; do not amend unasked.
5. **The docs moved.** `git diff --stat origin/main...HEAD -- docs CHANGELOG.md` is
   non-empty. If Swift, an xcconfig, the project file or a workflow changed and neither
   `docs/STATE.md` nor a feature file did, stop — the Stop hook's rule, applied before the
   gate sees it.
6. **ADR numbers.** If the branch adds `docs/decisions/NNNN-*`, confirm NNNN is on neither
   `origin/main` nor the head of any open pull request: `gh pr list --json headRefName`, then
   `git ls-tree --name-only origin/<head> docs/decisions/`. 0006 and 0010 both collided.
7. **Push.** `git push -u origin HEAD`.
8. **Body.** Write it to the scratchpad in the template's shape: what and why, what was run
   outside the gate, fixes found by running it, the docs that moved, notes. The title is a
   commit subject — present tense, the behaviour change. Then
   `gh pr create --base main --title "…" --body-file <path>`.
9. **Watch.** `gh pr checks --watch --fail-fast`. Report the URL and the gate's verdict; if it
   is red, report what the status mode finds, and change nothing.

## Mode: status — `/pr status`

1. `gh pr view --json number,title,mergeable,mergeStateStatus,statusCheckRollup,labels`.
2. Read the readiness comment — the one starting `<!-- swiftgate:report -->`. Its first line
   is the **review level**: *pass* (merge on the gate's approval), *brief* (this comment and
   the body), *full* (the diff). It also lists every condition that fired, and points at each
   lane's own comment.
3. If a check failed, separate the two kinds of red. A lane at `CANNOT_EVALUATE` never judged:
   its comment says whether evidence was missing (fix the input it names) or the judge did not
   run (re-run the job and read that lane's step log). Every finding is a claim about the code,
   answered in code or, for a static rule, waived in place with `// swiftgate:allow <rule> —
   reason`.
4. **A lane marked advisory did not make the gate red** — it cannot, until its record earns
   `blocks: true` (ADR 0021). Its findings are still claims about the code and still worth
   answering; they are just not what is blocking. Say which is which.
5. Report in under ten lines: the level, what is red, why, and the one next action.

## Mode: land — `/pr land`

1. **Preconditions, all of them.** Every check green and `mergeable` is `MERGEABLE`; or the
   gate is red, the `gate-override` label is set *and* the body carries the reason line.
   Otherwise stop and run the status mode instead. **Fedir reads to the level the gate
   assigned** — the `review:pass|brief|full` label, and the reasons in the readiness comment —
   before he merges. A session never merges, whatever the level says (ADR 0017).
2. **If the branch lives in a worktree, remove it first** — `git worktree list` names it;
   `git worktree remove <path>`. A branch checked out anywhere cannot be deleted, and
   `prune` only forgets worktrees whose directory is already gone.
3. `gh pr merge --merge --delete-branch`. A merge commit: never `--squash`, which discards
   the messages the feature journal points at; never `--rebase`, which rewrites the SHAs.
4. Locally: `git checkout main && git pull --ff-only`.
5. **The ledger.** `docs/STATE.md` on `main` now says what just became true — the pull
   request moved it. If the merge closed the step "Next action" names, the next one is not
   written yet: say so, and let `/feature-start` pick it up.
