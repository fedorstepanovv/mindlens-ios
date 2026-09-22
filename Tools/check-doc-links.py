#!/usr/bin/env python3
"""Fail if any doc points at a path that doesn't exist.

The Flutter repo's SESSION_HANDOFF.md told every new session to open
`memory/MEMORY.md` as the source of truth. That file was never created. A doc that
lies costs a reader real time before they work out it's wrong, so we make it
impossible to commit one.

Checks markdown links and backticked repo paths. Run from the repo root.
"""
from __future__ import annotations
import re, subprocess, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
# Backticked strings are only treated as paths when they start with one of these.
PATH_PREFIXES = ("docs/", "Packages/", "Tools/", ".claude/", ".agents/", ".github/", "mindlens/", "../")
MD_LINK = re.compile(r"\[[^\]]*\]\(([^)#]+?)(?:#[^)]*)?\)")
BACKTICKED = re.compile(r"`([^`\n]+)`")

def is_checkable(token: str) -> bool:
    if not token.startswith(PATH_PREFIXES):
        return False
    # A worktree is a transient second checkout. SKIP_DIRS already keeps it out of the
    # file walk; references to paths inside one must be skipped for the same reason, or
    # writing down that a worktree caused a bug becomes a bug.
    if token.startswith(".claude/worktrees/"):
        return False
    # Skip globs, placeholders and prose.
    return not any(c in token for c in "*<>? ")

# Directories whose markdown is not ours: build products, and the checkouts the merge
# gate makes. `worktrees` holds git worktrees (`.claude/worktrees/gate`) — a worktree is a
# *different checkout* of this repo, so its docs/ tree is that branch's copy of these same
# files, not a second home for ours. Without this the check fails for everyone the moment
# a gate run is in flight, and a guard that fails when nothing is wrong gets switched off.
SKIP_DIRS = {".git", ".build", ".swiftpm", "DerivedData", ".swiftgate", "worktrees"}

# Files whose entries are never edited once they land. A path one of them names may have
# been deleted since; that is history, not a lie — provided git remembers the path. A
# typo, git does not remember.
APPEND_ONLY = ("CHANGELOG.md", "docs/decisions/")


def once_existed(target: str) -> bool:
    out = subprocess.run(
        ["git", "log", "--oneline", "-1", "--all", "--", target.rstrip("/")],
        cwd=ROOT, capture_output=True, text=True,
    )
    return out.returncode == 0 and out.stdout.strip() != ""


def inside_repo(path: Path) -> tuple[str, ...]:
    """The path's parts below the repo root. SKIP_DIRS is matched against these, never
    against the absolute path: a checkout under a directory called `worktrees` — which is
    exactly where a git worktree lives — once made this script skip every document and
    report success."""
    return path.relative_to(ROOT).parts

# Files that must stay small, and why. A status file that grows without bound stops
# being read, which is how the equivalent doc in the server repo reached 2,000 lines.
SIZE_CAPS = {
    "docs/STATE.md": 80,
    "docs/LESSONS.md": 40,
    # The always-on instruction file. Every vendor's guidance lands near 200 lines; past that
    # the file is read less, not more (ADR 0022). CLAUDE.md is an import of it plus a few
    # Claude-only lines, so its cap is what those lines need and no more.
    "AGENTS.md": 200,
    "CLAUDE.md": 40,
    # One per feature: capability, behaviour, settled decisions, the next few steps, a short
    # journal. The journal is append-only, so the cap is what forces its oldest lines out to
    # git log instead of letting the file become the history it is meant to point at.
    "docs/features/*.md": 150,
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
    "AGENTS.md": ".",
}


def check_sizes() -> list[str]:
    problems = []
    for pattern, cap in SIZE_CAPS.items():
        for path in sorted(ROOT.glob(pattern)):
            rel = path.relative_to(ROOT).as_posix()
            lines = len(path.read_text(encoding="utf-8").splitlines())
            if lines > cap:
                if rel.startswith("docs/features/"):
                    advice = "Drop the oldest journal lines — they are in git log — or shrink done steps to one line."
                elif rel in ("AGENTS.md", "CLAUDE.md"):
                    advice = "Move a procedure to a skill, a fact about one area to its docs/ file, or cut it."
                else:
                    advice = "Move what is no longer current to CHANGELOG.md, or prune."
                problems.append(f"{rel} is {lines} lines, cap is {cap}. {advice}")
    return problems


FEATURE_STATUS = re.compile(r"^Status:\s*(\S+)", re.M)
STAGE_ROW = re.compile(r"^\|\s*\d+\s*\|[^|]*\|\s*(\S+)\s*\|([^\n]*)$", re.M)


def check_features() -> list[str]:
    """The two layers must agree: every feature file has a stage row, and their statuses match.

    docs/STATE.md is the ledger across features; docs/features/<name>.md is one feature's
    steps and journal. A file with no row is invisible to a session that orients from the
    ledger; a row that says 🟡 while the file says ⬜ is two answers to one question.
    """
    problems = []
    state = (ROOT / "docs/STATE.md").read_text(encoding="utf-8")
    rows = [(m.group(1), m.group(2)) for m in STAGE_ROW.finditer(state)]

    for path in sorted((ROOT / "docs/features").glob("*.md")):
        rel = path.relative_to(ROOT).as_posix()
        if path.name.startswith("0000-"):
            continue

        row_status = next((status for status, rest in rows if rel in rest), None)
        if row_status is None:
            problems.append(f"{rel} has no row in docs/STATE.md's stage table that names it.")
            continue

        m = FEATURE_STATUS.search(path.read_text(encoding="utf-8"))
        if not m:
            problems.append(f"{rel} has no `Status:` line — the template's first line under the title.")
            continue
        if m.group(1) != row_status:
            problems.append(
                f"{rel} says {m.group(1)} but its docs/STATE.md row says {row_status}. One of them is stale."
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
        if d.is_dir() and d != ROOT / "docs" and not SKIP_DIRS.intersection(inside_repo(d)):
            problems.append(f"{d.relative_to(ROOT)} — there is one docs/ tree, at the repo root.")
    return problems


def main() -> int:
    problems: list[str] = []
    # A `../` target points at a sibling repository — the Flutter app, the server. Those
    # sit beside this one on a working machine, but not in a CI checkout or a git
    # worktree. Report them, don't fail on them: their absence says nothing about
    # whether this repo's docs are honest.
    external: list[str] = []
    historical: list[str] = []
    docs = [md for md in sorted(ROOT.rglob("*.md")) if not SKIP_DIRS.intersection(inside_repo(md))]
    if not docs:
        print("No documents found — the walk is broken, and a check that checks nothing must not pass.")
        return 1
    for md in docs:
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
                if target.startswith("../"):
                    external.append(entry)
                elif rel.as_posix().startswith(APPEND_ONLY) and once_existed(target):
                    historical.append(entry)
                else:
                    problems.append(entry)

    if historical:
        print("Historical references — the path is gone, and the file is append-only history:\n")
        for h in historical:
            print(f"  {h}")
        print()

    if external:
        print("Sibling-repo references not resolvable here (expected in CI and worktrees):\n")
        for e in external:
            print(f"  {e}")
        print()

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

    disagreeing = check_features()
    if disagreeing:
        print("Feature files that disagree with the ledger:\n")
        for p in disagreeing:
            print(f"  {p}")
        print()

    if problems:
        print("Broken references in documentation:\n")
        for p in problems:
            print(f"  {p}")
        print(f"\n{len(problems)} broken reference(s). Fix the path or create the file.")
        return 1

    if oversized or misplaced or disagreeing:
        return 1

    print(f"Docs OK — {len(docs)} documents checked; every reference resolves, every capped file is\n"
          "within budget, every document is in its one home, and every feature file agrees with the ledger.")
    return 0

if __name__ == "__main__":
    sys.exit(main())
