#!/usr/bin/env python3
"""Measure `issueops project route-docs` ranking against records that past plans cited.

`build-fixture` extracts, from every committed `.issueops/issues/*/plan.md`, the
`.issueops/cautions` and `.issueops/adr` files cited in the section
"## 적용되는 결정과 주의사항". `measure` replays each plan title through the
binary and reports recall@5 and precision. It is a measurement tool, never a
gate: records change often, so absolute numbers drift. Compare the numbers
taken before and after a ranking change (.issueops/testing/unit-and-contract.md).
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from typing import Callable, Sequence

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_FIXTURE = REPO_ROOT / "testdata" / "route-docs" / "plan-citations.json"
SECTION_HEADING = "## 적용되는 결정과 주의사항"
TOP_K = 5
RECORD_DIRS = ("cautions", "adr")
RECORD_PREFIXES = tuple(f".issueops/{name}/" for name in RECORD_DIRS)
# Index files list records but are not records themselves.
NOT_RECORDS = {".issueops/adr/README.md", ".issueops/cautions/README.md"}
CITATION = re.compile(r"(?<![\w./-])((?:\.issueops/)?(?:cautions|adr)/[A-Za-z0-9_./-]+\.md)")

Runner = Callable[[str], Sequence[str]]


def plan_title(text: str) -> str | None:
    """Return the first `# ` heading of a plan, or None."""
    for line in text.splitlines():
        if line.startswith("# "):
            return line[2:].strip()
    return None


def decision_section(text: str) -> str | None:
    """Return the body of the decisions section, up to the next `## ` heading."""
    lines = text.splitlines()
    start = next((i for i, line in enumerate(lines) if line.strip() == SECTION_HEADING), None)
    if start is None:
        return None
    body: list[str] = []
    for line in lines[start + 1 :]:
        if line.startswith("## "):
            break
        body.append(line)
    return "\n".join(body)


def normalize_citation(path: str) -> str:
    return path if path.startswith(".issueops/") else f".issueops/{path}"


def cited_records(section: str, exists: Callable[[str], bool]) -> list[str]:
    """Return sorted unique record paths linked in a section that exist in the repo."""
    found = {normalize_citation(match) for match in CITATION.findall(section)}
    return sorted(path for path in found if path not in NOT_RECORDS and exists(path))


def issue_key(name: str) -> tuple[int, int | str]:
    return (0, int(name)) if name.isdigit() else (1, name)


def build_fixture(repo: Path) -> dict:
    """Build the fixture from committed plans; plans without citations are skipped."""
    tasks = []
    plans = sorted((repo / ".issueops" / "issues").glob("*/plan.md"), key=lambda p: issue_key(p.parent.name))
    for plan in plans:
        text = plan.read_text(encoding="utf-8")
        section = decision_section(text)
        title = plan_title(text)
        if section is None or title is None:
            continue
        cited = cited_records(section, lambda rel: (repo / rel).is_file())
        if not cited:
            continue
        name = plan.parent.name
        tasks.append({"issue": int(name) if name.isdigit() else name, "title": title, "cited": cited})
    return {"source": f"{SECTION_HEADING} of .issueops/issues/*/plan.md", "tasks": tasks}


def load_fixture(path: Path) -> list[dict]:
    data = json.loads(path.read_text(encoding="utf-8"))
    tasks = data.get("tasks") if isinstance(data, dict) else None
    if not isinstance(tasks, list) or not tasks:
        raise ValueError(f"{path}: expected a non-empty 'tasks' list")
    for task in tasks:
        if not isinstance(task.get("title"), str) or not task["title"]:
            raise ValueError(f"{path}: task {task.get('issue')!r} has no title")
        cited = task.get("cited")
        if not isinstance(cited, list) or not cited or not all(isinstance(item, str) for item in cited):
            raise ValueError(f"{path}: task {task.get('issue')!r} has no cited records")
    return tasks


def record_paths(docs: Sequence[dict]) -> list[str]:
    """Keep routed docs that are cautions/adr files, in the router's order."""
    paths = [doc.get("rel_path", "") for doc in docs]
    return [path for path in paths if path.startswith(RECORD_PREFIXES)]


def binary_runner(binary: str, repo: Path) -> Runner:
    def run(title: str) -> list[str]:
        command = [binary, "project", "route-docs", "--repo", str(repo), "--task", title, "--json"]
        completed = subprocess.run(command, capture_output=True, text=True, check=False)
        if completed.returncode != 0:
            raise RuntimeError(f"route-docs failed for task {title!r}: {completed.stderr.strip() or completed.stdout.strip()}")
        return record_paths(json.loads(completed.stdout).get("docs", []))

    return run


def score(tasks: Sequence[dict], runner: Runner, jobs: int = 1, k: int = TOP_K) -> dict:
    """Score routed records against cited ones.

    recall_at_k is micro-averaged (hits over all cited records); precision is hits
    over all returned records within the top k. mean_task_recall is the per-task mean.
    """
    with ThreadPoolExecutor(max_workers=max(1, jobs)) as pool:
        routed = list(pool.map(lambda task: list(runner(task["title"]))[:k], tasks))
    per_task = []
    for task, returned in zip(tasks, routed):
        cited = set(task["cited"])
        hits = [path for path in returned if path in cited]
        per_task.append(
            {
                "issue": task.get("issue"),
                "title": task["title"],
                "cited": len(cited),
                "returned": len(returned),
                "hits": len(hits),
                "recall": len(hits) / len(cited),
                "misses": sorted(cited - set(returned)),
            }
        )
    cited_total = sum(item["cited"] for item in per_task)
    returned_total = sum(item["returned"] for item in per_task)
    hits_total = sum(item["hits"] for item in per_task)
    return {
        "k": k,
        "tasks": len(per_task),
        "cited": cited_total,
        "returned": returned_total,
        "hits": hits_total,
        "recall_at_k": hits_total / cited_total if cited_total else 0.0,
        "precision": hits_total / returned_total if returned_total else 0.0,
        "mean_task_recall": sum(item["recall"] for item in per_task) / len(per_task) if per_task else 0.0,
        "per_task": per_task,
    }


def render_human(result: dict) -> str:
    k = result["k"]
    lines = [
        f"tasks={result['tasks']} cited={result['cited']} returned={result['returned']} hits={result['hits']}",
        f"recall@{k}={result['recall_at_k']:.3f} precision={result['precision']:.3f} mean_task_recall={result['mean_task_recall']:.3f}",
    ]
    missed = [item for item in result["per_task"] if item["misses"]]
    lines.append(f"tasks with misses: {len(missed)}/{result['tasks']}")
    for item in missed:
        lines.append(f"- issue {item['issue']}: {item['title']} (recall {item['recall']:.2f}, returned {item['returned']})")
        lines.extend(f"    missed {path}" for path in item["misses"])
    return "\n".join(lines)


def cmd_build_fixture(args: argparse.Namespace) -> int:
    fixture = build_fixture(Path(args.repo))
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(fixture, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {output} ({len(fixture['tasks'])} tasks)")
    return 0


def cmd_measure(args: argparse.Namespace) -> int:
    tasks = load_fixture(Path(args.fixture))
    result = score(tasks, binary_runner(args.binary, Path(args.repo)), jobs=args.jobs)
    print(json.dumps(result, ensure_ascii=False, indent=2) if args.json else render_human(result))
    return 0


def parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = parser.add_subparsers(dest="command", required=True)
    build = sub.add_parser("build-fixture", help="extract cited records from committed plans")
    build.add_argument("--repo", default=str(REPO_ROOT))
    build.add_argument("--output", default=str(DEFAULT_FIXTURE))
    build.set_defaults(func=cmd_build_fixture)
    measure = sub.add_parser("measure", help="report recall@5 and precision of `project route-docs`")
    measure.add_argument("--binary", required=True, help="issueops binary to run")
    measure.add_argument("--repo", default=".", help="repository passed to route-docs (default: .)")
    measure.add_argument("--fixture", default=str(DEFAULT_FIXTURE))
    measure.add_argument("--jobs", type=int, default=4, help="parallel route-docs runs (default: 4)")
    measure.add_argument("--json", action="store_true", help="print JSON instead of the human summary")
    measure.set_defaults(func=cmd_measure)
    return parser.parse_args(argv)


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    try:
        return args.func(args)
    except (OSError, ValueError, RuntimeError) as exc:
        print(f"route_docs_eval: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
