---
description: Get oriented in this repo before starting work
---

Read in this order, then summarise in under ten lines where the project stands and what
the next action is:

1. `docs/STATE.md` — the only file that says what is currently true.
2. `CLAUDE.md` — the rules you're working under.
3. `docs/ARCHITECTURE.md` — the module graph and dependency rules.
4. The most recent ADRs in `docs/decisions/` — decisions already settled. Don't re-open
   them without a reason.

Then run `git log --oneline -10` and `git status` to see where the code actually is.

If `docs/STATE.md` disagrees with the code, the code is right and the doc is stale — say
so, and fix the doc before starting anything new.
