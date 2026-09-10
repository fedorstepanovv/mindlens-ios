---
description: Close out a feature — verify it builds, tests, lints, and that the docs tell the truth
---

Walk this checklist in order. Do not skip a step because it "looks fine" — run it.

## 1. It builds and passes
```
xcodebuild -scheme mindlens -destination 'platform=iOS Simulator,name=iPhone 17' build test
```
Report real output. If tests fail, say so with the failure — never summarise a red suite as done.

## 2. The tests that must exist, exist
Per `docs/TESTING.md`:
- Every new ViewModel has tests covering loading, success, failure and empty.
- Every new API response type has a decoding test against a **real captured** fixture.
- Any bug fixed in this work has a regression test named for the bug.

If any are missing, write them now rather than noting them as follow-ups.

## 3. Design rules hold
Per `docs/DESIGN.md`: Dynamic Type (no fixed sizes), VoiceOver labels on interactive
elements, light and dark both correct, 44pt tap targets.

## 4. Docs tell the truth
- Update `docs/STATE.md` — status, what actually works, what is deliberately deferred and why.
- Write an ADR in `docs/decisions/` if you made a call someone might later question.
- Update `docs/API.md` if an endpoint's shape was touched.
- **Do not create a new top-level document.** Update the existing ones.

## 5. Nothing dangles
```
python3 Tools/check-doc-links.py
```

## 6. Commit
One commit, present-tense subject describing the behaviour change. Then report what
landed, what you deliberately left out, and anything you're unsure about.
