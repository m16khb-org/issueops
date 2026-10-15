#!/usr/bin/env python3
"""Measure `issueops remote score --judge none` label selection on this repository's issues.

Reads every issue of the current GitHub repository (`gh issue list`, read only),
keeps the issues that carry exactly one label inside --min-number/--max-number,
strips the noise a reader would not see when the label is chosen (leading
`[tag]` title prefixes, `#123` references, GitHub issue/PR URLs), and scores each
one with the repository's labels (`gh label list`) as candidates at the default
threshold.

Reported on one JSON line:
  n_total     issues in range with exactly one label
  n_selected  issues where the scorer selected at least one label
  n_correct   selected issues whose selected labels are exactly the GitHub label
  accuracy    n_correct / n_selected (0 when nothing was selected)

--check prints the JSON line and then a final `PASS` line when n_selected and
accuracy meet --min-selected and --min-accuracy; otherwise a final `FAIL` line
and exit code 1. Nothing is written to the repository or to GitHub; request
files live in a temporary directory that is removed afterwards.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path

TITLE_TAG_RE = re.compile(r"^\s*(\[[^\]]+\]\s*)+")
ISSUE_URL_RE = re.compile(r"https?://github\.com/\S+/(issues|pull)/\d+")
ISSUE_REF_RE = re.compile(r"(?<![\w/])#\d+")


def run_gh(*args: str) -> list[dict]:
    proc = subprocess.run(["gh", *args], capture_output=True, text=True)
    if proc.returncode != 0:
        raise SystemExit(f"gh {' '.join(args)} failed: {proc.stderr.strip()}")
    return json.loads(proc.stdout)


def clean_issue(issue: dict) -> tuple[str, str]:
    title = ISSUE_REF_RE.sub("#?", TITLE_TAG_RE.sub("", issue["title"]))
    body = ISSUE_REF_RE.sub("#?", ISSUE_URL_RE.sub("[링크]", issue.get("body") or ""))
    return title, body


def score_labels(binary: str, workdir: Path, number: int, title: str, body: str, labels: list[dict]) -> list[str]:
    request = {
        "issue": {"title": title, "body": body},
        "label_candidates": [{"name": l["name"], "description": l.get("description", "")} for l in labels],
    }
    path = workdir / f"{number}.json"
    path.write_text(json.dumps(request, ensure_ascii=False), encoding="utf-8")
    proc = subprocess.run(
        [binary, "remote", "score", "--judge", "none", "--json", "--input", str(path)],
        capture_output=True,
        text=True,
    )
    try:
        result = json.loads(proc.stdout)
    except json.JSONDecodeError:
        raise SystemExit(f"remote score #{number} produced no JSON (exit {proc.returncode}): {proc.stderr.strip()}")
    return sorted(item["name"] for item in result.get("selected_labels") or [])


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--binary", default="bin/issueops", help="issueops binary to run")
    parser.add_argument("--min-number", type=int, default=0)
    parser.add_argument("--max-number", type=int, default=10**9)
    parser.add_argument("--check", action="store_true", help="print PASS/FAIL against the thresholds below")
    parser.add_argument("--min-selected", type=int, default=50)
    parser.add_argument("--min-accuracy", type=float, default=0.85)
    args = parser.parse_args()

    labels = run_gh("label", "list", "--limit", "200", "--json", "name,description")
    issues = run_gh("issue", "list", "--state", "all", "--limit", "1000", "--json", "number,title,body,labels")
    targets = [
        i
        for i in issues
        if args.min_number <= i["number"] <= args.max_number and len(i["labels"]) == 1
    ]

    n_selected = n_correct = 0
    with tempfile.TemporaryDirectory(prefix="label-score-eval-") as tmp:
        workdir = Path(tmp)
        for issue in sorted(targets, key=lambda i: i["number"]):
            title, body = clean_issue(issue)
            selected = score_labels(args.binary, workdir, issue["number"], title, body, labels)
            if selected:
                n_selected += 1
                if selected == [issue["labels"][0]["name"]]:
                    n_correct += 1

    accuracy = n_correct / n_selected if n_selected else 0.0
    print(
        json.dumps(
            {
                "min_number": args.min_number,
                "max_number": args.max_number,
                "n_total": len(targets),
                "n_selected": n_selected,
                "n_correct": n_correct,
                "accuracy": round(accuracy, 4),
            },
            ensure_ascii=False,
        )
    )
    if args.check:
        ok = n_selected >= args.min_selected and accuracy >= args.min_accuracy
        print("PASS" if ok else "FAIL")
        return 0 if ok else 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
