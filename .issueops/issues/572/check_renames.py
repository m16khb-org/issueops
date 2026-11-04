"""Gate G9 for issue #572: renamed Go identifiers leave no stale text.

Reads renames.txt next to this file ("<pkgpath> <Old> <New>" per line; a
trailing "# keep: <reason>" allows the old word to stay in text). The compiler
already proves no Go code still refers to a renamed object, so this script
looks where the compiler cannot: Go comments and string literals, testdata,
skills, configs, and the current (undated) .issueops documents. Dated records
must be unchanged against the sealed base.
"""

import pathlib
import re
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
BASE = "7714640b0b40b4b03e64ad1405140d47d26e9c84"
DATED = (
    ".issueops/adr/20",
    ".issueops/cautions/20",
    ".issueops/archive/",
    ".issueops/issues/",
    ".issueops/verified-execution/",
    ".issueops/evidence/",
    ".issueops/research/",
    ".issueops/plans/",
)
# Index tables copy dated record titles verbatim; those rows stay as recorded.
INDEX_ROW = re.compile(r"^\|\s*\d{4}-\d{2}-\d{2}\s*\|")


def load_renames():
    names = {}
    for raw in (HERE / "renames.txt").read_text(encoding="utf-8").splitlines():
        line, _, comment = raw.partition("#")
        parts = line.split()
        if len(parts) != 3:
            continue
        if comment.strip().startswith("keep:"):
            continue
        names[parts[1]] = parts[2]
    return names


def go_text_spans(src):
    """Yield (offset, text) of comments and string literals in Go source."""
    i, n = 0, len(src)
    while i < n:
        c = src[i]
        if src.startswith("//", i):
            j = src.find("\n", i)
            j = n if j < 0 else j
            yield i, src[i:j]
            i = j
        elif src.startswith("/*", i):
            j = src.find("*/", i + 2)
            j = n if j < 0 else j + 2
            yield i, src[i:j]
            i = j
        elif c in "\"'`":
            j = i + 1
            while j < n and src[j] != c:
                if c != "`" and src[j] == "\\":
                    j += 1
                j += 1
            yield i, src[i : j + 1]
            i = j + 1
        else:
            i += 1


def tracked(*patterns):
    # Tracked or new files that exist now: a file deleted in the worktree stays in the index until the commit.
    out = subprocess.run(["git", "ls-files", "--cached", "--others", "--exclude-standard", "--", *patterns], cwd=ROOT, capture_output=True, text=True, check=True)
    return sorted({p for p in out.stdout.splitlines() if p and (ROOT / p).is_file()})


def main():
    names = load_renames()
    if not names:
        print("renames.txt has no entries", file=sys.stderr)
        return 1
    word = re.compile(r"\b(" + "|".join(sorted(map(re.escape, names), key=len, reverse=True)) + r")\b")
    hits = []
    for rel in tracked("*.go"):
        if rel.startswith(DATED) or rel.startswith(".issueops/"):
            continue
        src = (ROOT / rel).read_text(encoding="utf-8", errors="replace")
        for offset, text in go_text_spans(src):
            for m in word.finditer(text):
                line = src.count("\n", 0, offset + m.start()) + 1
                hits.append(f"{rel}:{line}: {m.group(1)} -> {names[m.group(1)]}")
    docs = [p for p in tracked("*.md", "*.json", "*.txt", "*.yml", "*.yaml", "*.js", "*.py") if not p.startswith(DATED) and not p.endswith("renames.txt")]
    for rel in docs:
        path = ROOT / rel
        if not path.is_file():
            continue
        for number, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            if INDEX_ROW.match(line):
                continue
            for m in word.finditer(line):
                hits.append(f"{rel}:{number}: {m.group(1)} -> {names[m.group(1)]}")
    changed = subprocess.run(["git", "diff", "--name-only", BASE, "--", *[d.rstrip("/") + ("*" if not d.endswith("/") else "") for d in DATED]], cwd=ROOT, capture_output=True, text=True, check=True).stdout.split()
    changed = [p for p in changed if not p.startswith(".issueops/issues/572/")]
    for p in changed:
        hits.append(f"{p}: dated record changed since base")
    if hits:
        print("\n".join(hits))
        return 1
    print("renames-clean")
    return 0


if __name__ == "__main__":
    sys.exit(main())
