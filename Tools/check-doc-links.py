#!/usr/bin/env python3
"""Fail if any doc points at a path that doesn't exist.

The Flutter repo's SESSION_HANDOFF.md told every new session to open
`memory/MEMORY.md` as the source of truth. That file was never created. A doc that
lies costs a reader real time before they work out it's wrong, so we make it
impossible to commit one.

Checks markdown links and backticked repo paths. Run from the repo root.
"""
from __future__ import annotations
import re, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
# Backticked strings are only treated as paths when they start with one of these.
PATH_PREFIXES = ("docs/", "Packages/", "Tools/", ".claude/", "mindlens/", "../")
MD_LINK = re.compile(r"\[[^\]]*\]\(([^)#]+?)(?:#[^)]*)?\)")
BACKTICKED = re.compile(r"`([^`\n]+)`")

def is_checkable(token: str) -> bool:
    if not token.startswith(PATH_PREFIXES):
        return False
    # Skip globs, placeholders and prose.
    return not any(c in token for c in "*<>? ")

def main() -> int:
    problems: list[str] = []
    # A `../` target points at a sibling repository — the Flutter app, the server. Those
    # sit beside this one on a working machine, but not in a CI checkout or a git
    # worktree. Report them, don't fail on them: their absence says nothing about
    # whether this repo's docs are honest.
    external: list[str] = []
    for md in sorted(ROOT.rglob("*.md")):
        if ".git" in md.parts or "DerivedData" in md.parts:
            continue
        rel = md.relative_to(ROOT)
        text = md.read_text(encoding="utf-8")

        for line_no, line in enumerate(text.splitlines(), 1):
            targets = [t for t in MD_LINK.findall(line) if not t.startswith(("http://", "https://", "mailto:"))]
            targets += [t for t in BACKTICKED.findall(line) if is_checkable(t)]
            for target in targets:
                # A path may be written relative to the repo root or to the file itself.
                if any((base / target).resolve().exists() for base in (ROOT, md.parent)):
                    continue
                entry = f"{rel}:{line_no}  →  {target}"
                (external if target.startswith("../") else problems).append(entry)

    if external:
        print("Sibling-repo references not resolvable here (expected in CI and worktrees):\n")
        for e in external:
            print(f"  {e}")
        print()

    if problems:
        print("Broken references in documentation:\n")
        for p in problems:
            print(f"  {p}")
        print(f"\n{len(problems)} broken reference(s). Fix the path or create the file.")
        return 1

    print("Docs OK — every referenced path exists.")
    return 0

if __name__ == "__main__":
    sys.exit(main())
