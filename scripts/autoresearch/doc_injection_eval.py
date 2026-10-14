#!/usr/bin/env python3
"""Deterministic doc-injection quality benchmark for IssueOps stages.

For each scenario (one IssueOps stage moment) it reconstructs what the model
actually gets told to read before acting:

  * the stage skill text (skills/<skill>/SKILL.md): explicit .issueops/<NAME>.md
    mentions and a project_docs_route / route-docs directive;
  * project_docs_route output for the scenario task, when the skill (or a hook)
    tells the agent to route;
  * hook additionalContext for SessionStart, UserPromptSubmit, PreToolUse(Skill)
    and PostToolUse(Bash `issueops next`) probes. The static SessionStart
    catalog block is stripped: listing every doc is awareness, not injection.

Scores per scenario (0-100):
  recall     0.35  gold docs reached
  precision  0.15  reached docs that are gold
  timing     0.30  gold docs directed before the stage action anchor
  reflection 0.20  a paragraph of the stage text (outside bad-example sections)
                   names a project doc and requires recording what was applied
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

WEIGHTS = {"recall": 0.35, "precision": 0.15, "timing": 0.30, "reflection": 0.20}
HOOK_LINE = -1  # hook context is injected before the skill body runs
ROUTE_RE = re.compile(r"project_docs_route|route-docs")
RECORD_RE = re.compile(r"적는다|적고|기록한다|기록하고|기록해|남긴다|남기고|인용|\bcite\b|\brecord\b", re.I)
PROJECT_DOC_RE = re.compile(r"\.issueops/[A-Z_]+\.md|\b(?!SKILL\.md|README\.md)[A-Z][A-Z_]{2,}\.md|프로젝트 문서|운영 문서|문서 경로|[Pp]roject docs?")
BAD_SECTION_RE = re.compile(r"^(#{1,6})\s+.*(나쁜 예|Bad example|Anti-?pattern)", re.I)
HEADING_RE = re.compile(r"^(#{1,6})\s+")
CATALOG_HEADER = "Project docs under .issueops/"


def existing_docs(repo: Path) -> list[str]:
    return sorted(p.name for p in (repo / ".issueops").glob("*.md") if re.fullmatch(r"[A-Z_]+\.md", p.name))


def doc_mentions(lines: list[str], names: list[str]) -> dict[str, int]:
    first: dict[str, int] = {}
    pats = {n: re.compile(r"(?<![A-Za-z_])" + re.escape(n)) for n in names}
    for i, line in enumerate(lines):
        for n, p in pats.items():
            if n not in first and p.search(line):
                first[n] = i
    return first


def strip_bad_sections(lines: list[str]) -> list[str]:
    out, skip_level = [], 0
    for line in lines:
        h = HEADING_RE.match(line)
        if h:
            level = len(h.group(1))
            if skip_level and level <= skip_level:
                skip_level = 0
            if BAD_SECTION_RE.match(line):
                skip_level = level
        out.append("" if skip_level else line)
    return out


LIST_ITEM_RE = re.compile(r"^\s*([-*]|\d+\.)\s+")


def paragraphs(lines: list[str]) -> list[str]:
    """Prose units: blank-line paragraphs, with each list item its own unit."""
    out, cur = [], []
    for line in lines:
        starts_unit = not line.strip() or HEADING_RE.match(line) or LIST_ITEM_RE.match(line)
        if starts_unit and cur:
            out.append(" ".join(cur))
            cur = []
        if line.strip() and not HEADING_RE.match(line):
            cur.append(line.strip())
    if cur:
        out.append(" ".join(cur))
    return out


def reflection_required(lines: list[str]) -> bool:
    return any(PROJECT_DOC_RE.search(p) and RECORD_RE.search(p) for p in paragraphs(strip_bad_sections(lines)))


def strip_catalog(text: str) -> str:
    """Drop the static catalog block: its header and every non-blank line after it
    (usage line, one line per doc, omissions). It is awareness, not stage injection."""
    out, in_catalog = [], False
    for line in text.splitlines():
        if line.startswith(CATALOG_HEADER):
            in_catalog = True
            continue
        if in_catalog and line.strip():
            continue
        in_catalog = False
        out.append(line)
    return "\n".join(out).strip()


def collect_context(obj) -> list[str]:
    found = []
    if isinstance(obj, dict):
        for k, v in obj.items():
            if k == "additionalContext" and isinstance(v, str):
                found.append(v)
            else:
                found.extend(collect_context(v))
    elif isinstance(obj, list):
        for v in obj:
            found.extend(collect_context(v))
    return found


def run(bin_path: str, args: list[str], stdin: str, env: dict) -> tuple[int, str]:
    try:
        p = subprocess.run([bin_path, *args], input=stdin, capture_output=True, text=True, timeout=20, env=env)
    except subprocess.TimeoutExpired:
        return 124, ""
    return p.returncode, p.stdout


def hook_context(bin_path: str, repo: Path, sc: dict, env: dict) -> str:
    next_stdout = json.dumps({"ok": True, "stage": {"key": sc["stage"]}, "selected": {"id": "bench"}})
    probes = [
        ("session-start", {"hook_event_name": "SessionStart", "cwd": str(repo), "source": "startup"}),
        ("user-prompt-submit", {"hook_event_name": "UserPromptSubmit", "cwd": str(repo), "prompt": sc["task"]}),
        ("pre-tool-use", {"hook_event_name": "PreToolUse", "cwd": str(repo), "tool_name": "Skill",
                          "tool_input": {"skill": sc["skill"]}}),
        ("post-tool-use", {"hook_event_name": "PostToolUse", "cwd": str(repo), "tool_name": "Bash",
                           "tool_input": {"command": "issueops next --id bench --json"},
                           "tool_response": {"stdout": next_stdout, "stderr": "", "exit_code": 0}}),
    ]
    parts = []
    for event, payload in probes:
        code, out = run(bin_path, ["hook", event, "--host", "claude", "--repo", str(repo)], json.dumps(payload), env)
        if code != 0 or not out.strip():
            continue
        try:
            obj = json.loads(out)
        except json.JSONDecodeError:
            continue
        for ctx in collect_context(obj):
            ctx = strip_catalog(ctx)
            if ctx:
                parts.append(ctx)
    return "\n".join(parts)


def route_docs(bin_path: str, repo: Path, task: str, env: dict, universe: set[str]) -> tuple[set[str], int]:
    code, out = run(bin_path, ["project", "route-docs", "--repo", str(repo), "--task", task, "--json"], "", env)
    if code != 0:
        raise SystemExit(f"route-docs failed for task {task!r} (exit {code})")
    docs = json.loads(out)["docs"]
    names, records = set(), 0
    for d in docs:
        rel = d["rel_path"]
        m = re.fullmatch(r"\.issueops/([A-Z_]+\.md)", rel)
        if m and m.group(1) in universe:
            names.add(m.group(1))
        elif rel.startswith(".issueops/"):
            records += 1
    return names, records


def evaluate(sc: dict, repo: Path, bin_path: str, env: dict, universe: list[str]) -> dict:
    skill_path = repo / "skills" / sc["skill"] / "SKILL.md"
    lines = skill_path.read_text(encoding="utf-8").splitlines()
    anchor_re = re.compile(sc["anchor"])
    anchor = next((i for i, l in enumerate(lines) if anchor_re.search(l)), None)
    if anchor is None:
        raise SystemExit(f"{sc['id']}: anchor {sc['anchor']!r} not found in {skill_path}")

    hook_text = hook_context(bin_path, repo, sc, env)
    hook_lines = hook_text.splitlines()
    # line index per doc: earliest directive wins; hook text counts as before the skill body
    directive: dict[str, int] = dict(doc_mentions(lines, universe))
    for n in doc_mentions(hook_lines, universe):
        directive[n] = HOOK_LINE

    route_line = next((i for i, l in enumerate(lines) if ROUTE_RE.search(l)), None)
    if any(ROUTE_RE.search(l) for l in hook_lines):
        route_line = HOOK_LINE
    routed, records = set(), 0
    if route_line is not None:
        routed, records = route_docs(bin_path, repo, sc["task"].lower(), env, set(universe))
        for n in routed:
            directive[n] = min(directive.get(n, route_line), route_line)

    gold = set(sc["gold"])
    reached = set(directive)
    hit = reached & gold
    recall = len(hit) / len(gold)
    precision = len(hit) / len(reached) if reached else 0.0
    timely = {n for n in hit if directive[n] < anchor}
    timing = len(timely) / len(gold)
    reflection = 1.0 if reflection_required(lines + hook_lines) else 0.0
    score = 100 * (WEIGHTS["recall"] * recall + WEIGHTS["precision"] * precision
                   + WEIGHTS["timing"] * timing + WEIGHTS["reflection"] * reflection)
    route_only, _ = route_docs(bin_path, repo, sc["task"].lower(), env, set(universe))
    return {
        "id": sc["id"], "stage": sc["stage"], "skill": sc["skill"], "score": score,
        "recall": recall, "precision": precision, "timing": timing, "reflection": reflection,
        "route_used": route_line is not None, "route_recall": len(route_only & gold) / len(gold),
        "missing": sorted(gold - reached), "late": sorted(hit - timely), "noise": sorted(reached - gold),
        "hook_bytes": len(hook_text.encode()), "records": records,
    }


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", required=True)
    ap.add_argument("--repo", required=True)
    ap.add_argument("--scenarios", required=True)
    ap.add_argument("--json-out")
    a = ap.parse_args()
    repo = Path(a.repo).resolve()
    universe = existing_docs(repo)
    scenarios = json.loads(Path(a.scenarios).read_text(encoding="utf-8"))["scenarios"]

    with tempfile.TemporaryDirectory() as state:
        env = {k: v for k, v in os.environ.items() if not k.startswith("ISSUEOPS_")}
        env.update({"ISSUEOPS_STATE_DIR": state, "NO_COLOR": "1", "TZ": "UTC"})
        results = [evaluate(sc, repo, a.bin, env, universe) for sc in scenarios]

    print(f"{'scenario':<22}{'score':>7}{'R':>6}{'P':>6}{'T':>6}{'F':>4}  missing | late | noise")
    for r in results:
        print(f"{r['id']:<22}{r['score']:7.1f}{r['recall']:6.2f}{r['precision']:6.2f}{r['timing']:6.2f}"
              f"{int(r['reflection']):4d}  {','.join(r['missing']) or '-'} | {','.join(r['late']) or '-'}"
              f" | {','.join(r['noise']) or '-'}")

    n = len(results)
    mean = lambda k: sum(float(r[k]) for r in results) / n
    stages = {}
    for r in results:
        stages.setdefault(r["stage"], []).append(r["score"])
    print("per-stage:", ", ".join(f"{k}={sum(v)/len(v):.1f}" for k, v in stages.items()))
    if a.json_out:
        Path(a.json_out).write_text(json.dumps(results, ensure_ascii=False, indent=2), encoding="utf-8")

    print(f"METRIC doc_injection_score={mean('score'):.2f}")
    for k in ("recall", "precision", "timing", "reflection", "route_recall"):
        print(f"METRIC {k}={mean(k):.4f}")
    print(f"METRIC route_used_ratio={mean('route_used'):.4f}")
    print(f"METRIC min_scenario_score={min(r['score'] for r in results):.2f}")
    print(f"METRIC hook_context_bytes={mean('hook_bytes'):.1f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
