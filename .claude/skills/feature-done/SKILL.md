---
description: Close out a feature — verify it builds, tests, lints, and that the docs tell the truth
---

Walk this in order. Run each step; do not assume.

## 1. It builds and the suite is green
```
cd Packages/MindlensKit && swift test          # fast loop, ~2s
xcodebuild test -scheme mindlens -destination 'generic/platform=iOS Simulator'
Tools/check-build-settings.sh                  # resolved settings, not claimed ones
```
Report real output. A failing suite is never summarised as done.

## 2. Lint and format
```
swift-format format --in-place --recursive Packages/MindlensKit/Sources Packages/MindlensKit/Tests mindlens
swiftlint lint --quiet
```
Custom rules encode past mistakes (`docs/LESSONS.md`). If one fires, fix the code — do not
disable the rule without a written reason.

## 3. The tests that must exist, do
Per `docs/TESTING.md`: every new ViewModel covered for loading/success/failure/empty;
every new API response type has a decoding test against a **captured** fixture
(`Tools/capture-fixtures.sh`); any bug fixed here has a regression test tagged `.bug(...)`.

## 4. Design rules hold
Per `docs/DESIGN.md`: Dynamic Type, VoiceOver labels, light and dark, 44pt targets.

## 5. Write to the right file
| What | Where |
|---|---|
| The step you did, and what you learned | `docs/features/<name>.md` — tick the step, add the next one if it is now clear, **append** a journal line. ≤100 lines. |
| What is true now | `docs/STATE.md` — **rewrite**, don't append. ≤80 lines. Only if the stage status moved. |
| What changed | `CHANGELOG.md` under `[Unreleased]` |
| A choice worth questioning | new ADR — `ls docs/decisions/` first, numbers have collided |
| A mistake you made | `docs/LESSONS.md`, **with its guard** |

Do not create a new top-level document. A feature file is not one — it lives in
`docs/features/`, one per feature, from the template.

## 6. Nothing dangles
```
python3 Tools/check-doc-links.py
```
Checks every referenced path exists and every capped file is within budget.

## 7. Commit
One commit, present-tense subject describing the behaviour change. Then report what
landed, what you deliberately left out, and anything you are unsure about.
