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

# Directories whose markdown is not ours: build products, and the Flutter spec the
# merge gate checks out.
SKIP_DIRS = {".git", ".build", ".swiftpm", "DerivedData", ".swiftgate"}

# Files that must stay small, and why. A status file that grows without bound stops
# being read, which is how the equivalent doc in the server repo reached 2,000 lines.
SIZE_CAPS = {
    "docs/STATE.md": 80,
    "docs/LESSONS.md": 40,
    "CLAUDE.md": 150,
}

# Every document has exactly one home. A second copy is not a backup — it is a second
# answer to the same question, and the reader has no way to tell which one is current.
# A duplicate `docs/` tree once sat under `mindlens/` for a whole session unnoticed,
# because the size caps above are keyed to literal paths and never looked at it.
CANONICAL_HOME = {
    "STATE.md": "docs",
    "LESSONS.md": "docs",
    "ARCHITECTURE.md": "docs",
    "PATTERNS.md": "docs",
    "DESIGN.md": "docs",
    "TESTING.md": "docs",
    "API.md": "docs",
    "CHANGELOG.md": ".",
    "CLAUDE.md": ".",
}


def check_sizes() -> list[str]:
    problems = []
    for rel, cap in SIZE_CAPS.items():
        path = ROOT / rel
        if not path.exists():
            continue
        lines = len(path.read_text(encoding="utf-8").splitlines())
        if lines > cap:
            problems.append(
                f"{rel} is {lines} lines, cap is {cap}. "
                "Move what is no longer current to CHANGELOG.md, or prune."
            )
    return problems


def check_locations(docs: list[Path]) -> list[str]:
    problems = []
    for md in docs:
        rel = md.relative_to(ROOT)
        home = CANONICAL_HOME.get(md.name)
        if home is not None and str(rel.parent) != home:
            where = "the repo root" if home == "." else f"{home}/"
            problems.append(f"{rel} — {md.name} belongs in {where} and nowhere else.")

    for d in sorted(ROOT.rglob("docs")):
        if d.is_dir() and d != ROOT / "docs" and not SKIP_DIRS.intersection(d.parts):
            problems.append(f"{d.relative_to(ROOT)} — there is one docs/ tree, at the repo root.")
    return problems


def main() -> int:
    problems: list[str] = []
    docs = [md for md in sorted(ROOT.rglob("*.md")) if not SKIP_DIRS.intersection(md.parts)]
    for md in docs:
        rel = md.relative_to(ROOT)
        text = md.read_text(encoding="utf-8")

        for line_no, line in enumerate(text.splitlines(), 1):
            targets = [t for t in MD_LINK.findall(line) if not t.startswith(("http://", "https://", "mailto:"))]
            targets += [t for t in BACKTICKED.findall(line) if is_checkable(t)]
            for target in targets:
                # A path may be written relative to the repo root or to the file itself.
                if not any((base / target).resolve().exists() for base in (ROOT, md.parent)):
                    problems.append(f"{rel}:{line_no}  →  {target}")

    oversized = check_sizes()
    if oversized:
        print("Documents over their size cap:\n")
        for p in oversized:
            print(f"  {p}")
        print()

    misplaced = check_locations(docs)
    if misplaced:
        print("Documents in the wrong place:\n")
        for p in misplaced:
            print(f"  {p}")
        print()

    if problems:
        print("Broken references in documentation:\n")
        for p in problems:
            print(f"  {p}")
        print(f"\n{len(problems)} broken reference(s). Fix the path or create the file.")
        return 1

    if oversized or misplaced:
        return 1

    print("Docs OK — every reference resolves, every capped file is within budget,\n"
          "and every document is in its one home.")
    return 0

if __name__ == "__main__":
    sys.exit(main())
