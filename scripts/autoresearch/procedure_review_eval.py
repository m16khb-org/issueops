#!/usr/bin/env python3
"""Procedure compliance and review judgment quality for IssueOps stage skills.

Each scenario in procedure_review_spec.json runs its stage skill (Sonnet actor, `claude -p`) in a
per-scenario sandbox that copies the worktree a stage would see (AGENTS.md, the .issueops operating docs,
cmd/, internal/, configs/, skills/, go.mod, the issueops binary) and the scenario fixture under in/. The
actor may read, write files, and delegate to a subagent (the review skill requires a fresh-context reviewer).

Up to two scores per run:
  compliance  deterministic checks over the files the actor wrote and the tools it used
              (inputs untouched, unrelated changes not staged, no ledger writes in verify, ...).
  review      for review scenarios, an Opus judge with read access to the same sandbox matches the
              review's findings against the planted defects (recall) and checks every blocking finding
              for validity; on the clean plan every blocking finding must be valid.
  plan        for plan-writing scenarios, the judge checks fixed requirements and lists false claims
              the plan makes about the current repository.

The scenario score is the mean of the components it has; METRIC procedure_review_score is the mean over
all runs. Not deterministic (LLM actor and judge); `--reps` repeats each scenario.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import json
import os
import re
import shutil
import subprocess
import statistics
import sys
import tempfile
import time
import traceback
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import doc_injection_judge as dj  # noqa: E402

REVIEW_JUDGE_SCHEMA = {
    "type": "object",
    "properties": {
        "verdict": {"type": "string", "enum": ["pass", "revise", "stop", "none"]},
        "expected": {"type": "array", "items": {"type": "object", "properties": {
            "id": {"type": "string"}, "found": {"type": "boolean"}, "evidence": {"type": "string"}},
            "required": ["id", "found", "evidence"]}},
        "blocking_findings": {"type": "array", "items": {"type": "object", "properties": {
            "finding": {"type": "string"}, "matches_expected": {"type": "string"},
            "valid": {"type": "boolean"}, "why": {"type": "string"}},
            "required": ["finding", "matches_expected", "valid", "why"]}},
        "summary": {"type": "string"},
    },
    "required": ["verdict", "expected", "blocking_findings", "summary"],
}

REVIEW_JUDGE_SYSTEM = (
    "You grade one IssueOps review. The review text is what the reviewer (usually a delegated subagent) returned, plus "
    "any files the calling session wrote. List every finding the review treats as blocking/required (not optional "
    "suggestions) under blocking_findings. For each: matches_expected is the id of the planted defect it identifies, or "
    "an empty string; valid is true only if the defect really exists in the reviewed input and breaks a rule you can "
    "find in the repository docs or configs in the current directory (use the read tools to check), or is a concrete "
    "correctness bug. For each planted defect, found is true only if a finding (blocking or not) identifies that same "
    "defect. verdict is the review's final verdict, or none if it gave none."
)

PLAN_JUDGE_SCHEMA = {
    "type": "object",
    "properties": {
        "requirements": {"type": "array", "items": {"type": "object", "properties": {
            "id": {"type": "string"}, "met": {"type": "boolean"}, "evidence": {"type": "string"}},
            "required": ["id", "met", "evidence"]}},
        "false_claims": {"type": "array", "items": {"type": "object", "properties": {
            "claim": {"type": "string"}, "why": {"type": "string"}},
            "required": ["claim", "why"]}},
        "summary": {"type": "string"},
    },
    "required": ["requirements", "false_claims", "summary"],
}

PLAN_JUDGE_SYSTEM = (
    "You grade one IssueOps plan the session wrote or revised for the given inputs. The plan is in the files the session wrote (usually "
    "out/plan.md) plus its structured output. For each requirement, met is true only if the final plan itself "
    "satisfies it (a later revision in the same output counts; an intention mentioned only in the session's notes "
    "does not). false_claims lists statements the plan makes about the current repository (what code, docs, configs "
    "or commands do now) that are wrong; check each claim against the files in the current directory with the read "
    "tools before listing it. Proposals and unverified assumptions marked as such are not claims. A requirement about "
    "what the session did (for example a delta review it ran) is judged from the subagent results and written files."
)

# Stage scenarios whose result is what the session did (stop, record a blocker), not a plan it wrote.
OUTCOME_JUDGE_SYSTEM = (
    "You grade what one IssueOps stage session did for the given inputs: the files it wrote or changed, the commands it "
    "wrote to out/commands.sh, its subagent results, and its structured output. For each requirement, met is true only if "
    "the session's actions and final artifacts satisfy it; stated intentions that the artifacts contradict do not count. "
    "false_claims lists statements the session makes about the current repository (what code, docs, configs or "
    "commands do now) that are wrong; check each against the files in the current directory with the read tools."
)

# doc_injection_judge's actor prompt says the source is absent; this sandbox has it (see SOURCE_DIRS).
ACTOR_SYSTEM = (
    "You are the coding agent running exactly one IssueOps stage. The current directory is a copy of the repository "
    "worktree: AGENTS.md, the .issueops operating docs, the application source (cmd/, internal/), configs/, skills/ "
    "and go.mod. Git history is not present and the go toolchain cannot run here. Assume the stage preconditions hold "
    "(git checkout, active lease, prior stages done) and carry out the stage for the given task; do not run git or "
    "probe the environment, and do not stop on missing environment. Read tools, Write/Edit, and the Task delegation "
    "tool are available. "
    "`issueops project route-docs --repo . --task \"<summary>\" --json` (the CLI form of project_docs_route) and "
    "`issueops gates lint` and `issueops gates status` run here; other issueops commands do not. "
    "Finish with the structured result."
)

SOURCE_DIRS = ("cmd", "internal", "configs", "skills")
SHELL_FENCE_RE = re.compile(r"```(?:bash|sh|shell|zsh)\n(.*?)```", re.S)
INLINE_COMMENT_RE = re.compile(r"\s+#\s.*$")
# An echo/printf segment prints a message; its quoted text (for example "do not use git add -A") is not a command.
MESSAGE_SEGMENT_RE = re.compile(r"(^|&&|\|\||;)\s*(?:echo|printf)\b(?:\"[^\"]*\"|'[^']*'|[^;&|\"'])*")


def command_lines(root: Path, rels: list[str]) -> str:
    """Executable command text the actor wrote: .sh files and shell code fences in other files,
    without comment lines, trailing ` # ...` comments, or echo/printf messages. Prose that quotes a
    forbidden command (for example "did not run `git push --delete`") is not an action and must not
    trip a check."""
    chunks = []
    for rel in rels:
        text = (root / rel).read_text(encoding="utf-8", errors="replace")
        chunks.extend([text] if rel.endswith(".sh") else SHELL_FENCE_RE.findall(text))
    lines = []
    for chunk in chunks:
        for line in chunk.splitlines():
            if line.lstrip().startswith("#"):
                continue
            lines.append(MESSAGE_SEGMENT_RE.sub(r"\1", INLINE_COMMENT_RE.sub("", line)))
    return "\n".join(lines)


def check_compliance(checks: list[dict], root: Path, before: dict[str, str], tools: list[dict]) -> dict[str, bool]:
    after = dj.snapshot(root)
    changed = {rel for rel, digest in after.items() if before.get(rel) != digest} | (set(before) - set(after))
    out_rels = [rel for rel in sorted(after) if rel.startswith("out/")]
    out_text = "\n".join((root / rel).read_text(encoding="utf-8", errors="replace") for rel in out_rels)
    no_comments = command_lines(root, out_rels)
    used = {t["tool"] for t in tools}
    results = {}
    for c in checks:
        kind = c["type"]
        if kind == "unchanged":
            ok = not any(rel.startswith(c["prefix"]) for rel in changed)
        elif kind == "writes_only_under":
            ok = all(rel.startswith(c["prefix"]) for rel in changed)
        elif kind == "tool_used":
            ok = bool(used & set(c["tools"]))
        elif kind == "absent":
            ok = not re.search(c["regex"], no_comments, re.M)
        elif kind == "present":
            ok = bool(re.search(c["regex"], out_text, re.M))
        elif kind == "absent_unless":
            ok = not re.search(c["regex"], no_comments, re.M) or bool(re.search(c["unless"], out_text, re.M))
        elif kind == "file_contains":
            p = root / c["path"]
            ok = p.is_file() and c["text"] in p.read_text(encoding="utf-8", errors="replace")
        elif kind == "gates_lint_clean":
            # The written plan's gates pass `issueops gates lint` (the sandbox's own binary) and there is at least one.
            p = root / c["path"]
            ok = False
            if p.is_file():
                run = subprocess.run([str(root / "bin" / "issueops"), "gates", "lint", "--plan", str(p), "--cwd", str(root),
                                      "--json"], capture_output=True, text=True, timeout=120)
                try:
                    lint = json.loads(run.stdout)
                    ok = bool(lint.get("clean")) and lint.get("total_gates", 0) > 0
                except json.JSONDecodeError:
                    ok = False
        else:
            raise SystemExit(f"unknown check type {kind!r}")
        results[c["id"]] = ok
    return results


def review_text(actor: dict, written: str) -> str:
    delegated = "\n\n".join(f"<subagent_result>\n{t['output'][:30000]}\n</subagent_result>"
                            for t in actor["tools"] if t["tool"] in ("Agent", "Task"))
    return (f"{delegated or '(no subagent was used)'}\n\n# Files the calling session wrote\n{written or '(none)'}\n\n"
            f"# Calling session's structured output\n{json.dumps(actor['data'], ensure_ascii=False, indent=1)}")


def grade_review(sc: dict, inputs: str, text: str, root: Path, env: dict, a) -> dict:
    expected = sc.get("expected_findings", [])
    prompt = (f"# Stage\n{sc['stage']} (skill {sc['skill']})\n\n# Reviewed input\n{inputs}\n\n"
              f"# Planted defects (ground truth)\n{json.dumps(expected, ensure_ascii=False, indent=1) if expected else '(none: this input is a clean plan)'}\n\n"
              f"# Review output\n{text}\n")
    argv = dj.base_argv(a.judge_model, a.effort, REVIEW_JUDGE_SYSTEM, REVIEW_JUDGE_SCHEMA) + [
        "--tools", "Read,Glob,Grep", "--allowedTools", "Read", "Glob", "Grep"]
    try:
        g = dj.claude_stream(argv, prompt, root, env, a.timeout)["data"]
    except RuntimeError as e:
        if "error_max_structured_output_retries" not in str(e):
            raise
        g = dj.claude_stream(argv, prompt, root, env, a.timeout)["data"]
    found = {e["id"] for e in g["expected"] if e.get("found")} & {e["id"] for e in expected}
    invalid = [b for b in g["blocking_findings"] if not b.get("valid")]
    if sc.get("clean"):
        score = max(0.0, 100 - 25 * len(invalid) - (25 if g["verdict"] == "stop" else 0))
    else:
        score = max(0.0, 100 * len(found) / len(expected) - 15 * len(invalid))
    return {"score": score, "found": sorted(found), "missed": sorted({e["id"] for e in expected} - found),
            "invalid_blocking": [b["finding"][:200] for b in invalid], "verdict": g["verdict"], "judge": g}


def grade_plan(sc: dict, inputs: str, text: str, root: Path, env: dict, a) -> dict:
    reqs = sc["requirements"]
    prompt = (f"# Stage inputs (the issue, or the plan and review the session revised)\n{inputs}\n\n# Requirements\n{json.dumps(reqs, ensure_ascii=False, indent=1)}\n\n"
              f"# Plan output\n{text}\n")
    system = OUTCOME_JUDGE_SYSTEM if sc.get("outcome") else PLAN_JUDGE_SYSTEM
    argv = dj.base_argv(a.judge_model, a.effort, system, PLAN_JUDGE_SCHEMA) + [
        "--tools", "Read,Glob,Grep", "--allowedTools", "Read", "Glob", "Grep"]
    def ask() -> dict:
        try:
            return dj.claude_stream(argv, prompt, root, env, a.timeout)["data"]
        except RuntimeError as e:
            if "error_max_structured_output_retries" not in str(e):
                raise
            return dj.claude_stream(argv, prompt, root, env, a.timeout)["data"]

    # The judge sometimes returns entries for only some requirements; an unjudged requirement
    # is a judge failure, not a miss, so ask again (bounded) before scoring it as missed.
    want = {r["id"] for r in reqs}
    g = ask()
    for _ in range(2):
        if want <= {r.get("id") for r in g["requirements"]}:
            break
        g = ask()
    met = {r["id"] for r in g["requirements"] if r.get("met")} & {r["id"] for r in reqs}
    score = max(0.0, 100 * len(met) / len(reqs) - 15 * len(g["false_claims"]))
    # Reported in the review fields so the per-run line and aggregates read the same way.
    return {"score": score, "found": sorted(met), "missed": sorted({r["id"] for r in reqs} - met),
            "invalid_blocking": [c["claim"][:200] for c in g["false_claims"]], "verdict": "none",
            "judge": dict(g, blocking_findings=[{"finding": c["claim"], "valid": False, "why": c["why"]}
                                                for c in g["false_claims"]])}


def run_once(sc: dict, rep: int, shared_root: Path, repo: Path, env: dict, a) -> dict:
    fixture = HERE / sc["fixture"]
    scenario_dir = tempfile.mkdtemp(prefix=f"pr-{sc['id']}-{rep}-")
    root = Path(scenario_dir) / "repo"
    try:
        shutil.copytree(shared_root, root)
        shutil.copytree(fixture, root / "in")
        inputs = dj.fixture_block(fixture)
        hook = dj.det_hook(a.bin, root, {"id": sc["id"], "stage": sc["stage"], "task": sc["task"], "skill": sc["skill"]}, env)
        skill = (repo / "skills" / sc["skill"] / "SKILL.md").read_text(encoding="utf-8")
        agents = (root / "AGENTS.md").read_text(encoding="utf-8")
        prompt = (
            f"# Project instructions (AGENTS.md, auto-loaded)\n<agents>\n{agents}\n</agents>\n\n"
            f"# Session context injected by hooks\n{hook or '(none)'}\n\n"
            f"# Stage\n{sc['stage']} (skill {sc['skill']})\n\n# Task\n{sc['task']}\n\n"
            f"# Stage skill\n<skill>\n{skill}\n</skill>\n\n"
            f"# Stage inputs (also on disk under in/)\nThe plan, diff, worktree state, or ledger this stage works on. "
            f"Treat them as the cycle's real artifacts.\n{inputs}\n\n"
            "Run this stage for the task: read what you need, then return which docs you consulted and the rules that "
            "apply, the order of your actions, each concrete decision with the rule and the file it came from (the stage "
            "skill, AGENTS.md, or a doc path), and the records the skill requires." + dj.ARTIFACT_PROMPT)
        argv = dj.base_argv(a.actor_model, a.effort, ACTOR_SYSTEM, dj.ACTOR_SCHEMA) + [
            "--tools", "Read,Glob,Grep,Bash,Write,Edit,Task", "--permission-mode", "acceptEdits",
            "--allowedTools", "Read", "Glob", "Grep", "Bash(issueops project route-docs:*)", "Bash(issueops gates lint:*)", "Bash(issueops gates status:*)",
            "Task", "Agent"]
        before = dj.snapshot(root)
        actor_started = time.time()
        actor = dj.claude_stream(argv, prompt, root, env, a.timeout)
        actor_seconds = time.time() - actor_started
        written = dj.written_files(root, before)
        compliance = check_compliance(sc.get("checks", []), root, before, actor["tools"])
        parts = [100 * sum(compliance.values()) / len(compliance)] if compliance else []
        review = None
        if sc.get("expected_findings") or sc.get("clean") or sc.get("requirements"):
            judge_root = Path(scenario_dir) / "judge"
            shutil.copytree(shared_root, judge_root)
            shutil.copytree(fixture, judge_root / "in")
            grade = grade_plan if sc.get("requirements") else grade_review
            review = grade(sc, inputs, review_text(actor, written), judge_root, env, a)
            parts.append(review["score"])
        return {"id": sc["id"], "rep": rep, "score": statistics.mean(parts), "compliance": compliance,
                "actor_seconds": round(actor_seconds, 1),
                "review_text": review_text(actor, written) if review else "",
                "review": review, "written": written, "actor": actor["data"],
                "trace": [f"{t['tool']} {json.dumps(t['input'], ensure_ascii=False)[:200]}" for t in actor["tools"]]}
    finally:
        shutil.rmtree(scenario_dir, ignore_errors=True)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", required=True)
    ap.add_argument("--repo", required=True)
    ap.add_argument("--spec", default=str(HERE / "procedure_review_spec.json"))
    ap.add_argument("--only", help="comma-separated scenario ids")
    ap.add_argument("--reps", type=int, default=2)
    ap.add_argument("--jobs", type=int, default=5)
    ap.add_argument("--actor-model", default="claude-sonnet-5-5")
    ap.add_argument("--judge-model", default="claude-opus-5-5")
    ap.add_argument("--effort", default="medium")
    ap.add_argument("--timeout", type=int, default=1200)
    ap.add_argument("--out", help="JSON report path")
    a = ap.parse_args()

    repo = Path(a.repo).resolve()
    scenarios = json.loads(Path(a.spec).read_text(encoding="utf-8"))["scenarios"]
    if a.only:
        scenarios = [s for s in scenarios if s["id"] in set(a.only.split(","))]
    started = time.time()
    results, failures = [], {}
    with tempfile.TemporaryDirectory() as state, tempfile.TemporaryDirectory() as sandbox:
        shared_root = Path(sandbox) / "repo"
        shared_root.mkdir()
        dj.build_sandbox(repo, shared_root, a.bin)
        # Real reviews and commits run in a worktree that has the source and host configs. Without them a
        # reviewer can only guess about code paths and timeouts, and the judge cannot check those guesses.
        # scripts/ holds this benchmark's spec and fixtures, so it stays out.
        for name in SOURCE_DIRS:
            shutil.copytree(repo / name, shared_root / name)
        for name in ("go.mod", "go.sum"):
            shutil.copy2(repo / name, shared_root / name)
        env = {k: v for k, v in os.environ.items() if not k.startswith("ISSUEOPS_")}
        env.update({"ISSUEOPS_STATE_DIR": state, "NO_COLOR": "1", "TZ": "UTC"})
        jobs = [(sc, rep) for sc in scenarios for rep in range(1, a.reps + 1)]
        with concurrent.futures.ThreadPoolExecutor(max_workers=a.jobs) as pool:
            futures = {}
            for sc, rep in jobs:
                # Each run's bin/ is its own copy, so PATH is set per run inside the sandbox copy.
                run_env = dict(env, PATH=f"{shared_root / 'bin'}{os.pathsep}{env.get('PATH', '')}")
                futures[pool.submit(run_once, sc, rep, shared_root, repo, run_env, a)] = f"{sc['id']}#{rep}"
            for f in concurrent.futures.as_completed(futures):
                try:
                    results.append(f.result())
                except Exception as e:  # keep the rest of the batch
                    failures[futures[f]] = f"{type(e).__name__}: {e} @ {traceback.extract_tb(e.__traceback__)[-1].lineno}"

    order = {s["id"]: i for i, s in enumerate(scenarios)}
    results.sort(key=lambda r: (order[r["id"]], r["rep"]))
    for r in results:
        failed = [k for k, v in r["compliance"].items() if not v]
        rv = r["review"]
        rv_txt = (f" review={rv['score']:.0f} found={','.join(rv['found']) or '-'} missed={','.join(rv['missed']) or '-'} "
                  f"invalid={len(rv['invalid_blocking'])} verdict={rv['verdict']}") if rv else ""
        print(f"{r['id']:<22}#{r['rep']} score={r['score']:5.1f} t={r['actor_seconds']:.0f}s "
              f"compliance_failed={','.join(failed) or '-'}{rv_txt}")
    for k, err in failures.items():
        print(f"{k:<24} FAILED: {err[:300]}")
    if a.out:
        Path(a.out).write_text(json.dumps({"results": results, "failures": failures}, ensure_ascii=False, indent=2),
                               encoding="utf-8")
    if not results:
        print("no results")
        return 1
    checks = [v for r in results for v in r["compliance"].values()]
    clean_ids = {s["id"] for s in scenarios if s.get("clean")}
    delta_ids = {s["id"] for s in scenarios if s.get("delta")}
    outcome_ids = {s["id"] for s in scenarios if s.get("outcome")}
    plan_ids = {s["id"] for s in scenarios if s.get("requirements")} - delta_ids - outcome_ids
    reviews = [r["review"] for r in results if r["review"] and r["id"] not in plan_ids | delta_ids | outcome_ids]
    defect_reviews = [r["review"] for r in results
                      if r["review"] and r["id"] not in clean_ids | plan_ids | delta_ids | outcome_ids]
    clean_reviews = [r["review"] for r in results if r["review"] and r["id"] in clean_ids]
    plans = [r["review"] for r in results if r["review"] and r["id"] in plan_ids]
    deltas = [r for r in results if r["id"] in delta_ids]
    outcomes = [r for r in results if r["id"] in outcome_ids]
    print(f"cost/time: wall={time.time() - started:.0f}s runs={len(results)} failed={len(failures)}")
    print(f"METRIC procedure_review_score={statistics.mean(r['score'] for r in results):.2f}")
    print(f"METRIC compliance_rate={sum(checks) / len(checks) if checks else 0:.4f}")
    if defect_reviews:
        print(f"METRIC review_recall={statistics.mean(len(rv['found']) / (len(rv['found']) + len(rv['missed'])) for rv in defect_reviews):.4f}")
    if reviews:
        print(f"METRIC invalid_blocking_per_review={statistics.mean(len(rv['invalid_blocking']) for rv in reviews):.3f}")
    if plans:
        print(f"METRIC plan_score={statistics.mean(p['score'] for p in plans):.2f}")
        print(f"METRIC plan_false_claims={statistics.mean(len(p['invalid_blocking']) for p in plans):.3f}")
    if clean_reviews:
        print(f"METRIC clean_plan_score={statistics.mean(rv['score'] for rv in clean_reviews):.2f}")
    if deltas:
        print(f"METRIC delta_score={statistics.mean(r['score'] for r in deltas):.2f}")
        print(f"METRIC delta_seconds={statistics.mean(r['actor_seconds'] for r in deltas):.1f}")
    if outcomes:
        print(f"METRIC stage_outcome_score={statistics.mean(r['score'] for r in outcomes):.2f}")
    print(f"METRIC failed_runs={len(failures)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
