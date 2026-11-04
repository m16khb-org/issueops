"""Gate G16 for issue #572: importers of the three renamed-clause packages use one spelling.

Collects the alias (or lack of one) of every import of the three packages in all
tracked Go files. Prints "aliases-clean" when every import is unaliased, except
the files listed in ALLOWED, where the same file already uses the package's name
for something else. Each package may carry at most one alias name.
"""

import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[3]
PACKAGES = {
    "issueops/internal/domain/issueopsremote": "issueopsremote",
    "issueops/internal/domain/guardpattern": "guardpattern",
    "issueops/internal/domain/traceclassification": "traceclassification",
}
# file -> (package path, alias, reason). Empty when no file collides.
ALLOWED = {}
IMPORT = re.compile(r'^\s*(?:import\s+)?(?:([A-Za-z_][A-Za-z0-9_]*|\.)\s+)?"([^"]+)"', re.M)


def main():
    listed = subprocess.run(["git", "ls-files", "--cached", "--others", "--exclude-standard", "*.go"], cwd=ROOT, capture_output=True, text=True, check=True).stdout.split()
    files = sorted({p for p in listed if (ROOT / p).is_file()})
    problems = []
    aliases = {path: set() for path in PACKAGES}
    for rel in files:
        text = (ROOT / rel).read_text(encoding="utf-8", errors="replace")
        for alias, path in IMPORT.findall(text):
            if path not in PACKAGES:
                continue
            if not alias:
                continue
            allowed = ALLOWED.get(rel)
            if allowed is None or allowed[0] != path or allowed[1] != alias:
                problems.append(f"{rel}: {path} imported as {alias}")
            else:
                aliases[path].add(alias)
    for path, names in aliases.items():
        if len(names) > 1:
            problems.append(f"{path}: more than one alias name {sorted(names)}")
    if problems:
        print("\n".join(problems))
        return 1
    print("aliases-clean")
    return 0


if __name__ == "__main__":
    sys.exit(main())
