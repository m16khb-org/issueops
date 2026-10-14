#!/usr/bin/env python3
"""Qualitative evaluation of IssueOps stage doc injection with LLM selection and an LLM judge.

The deterministic benchmark (doc_injection_eval.py) proves which docs a stage
*directs* the model to and when. This script checks what a model actually does
with that direction:

  1. actor (`claude -p`, default Sonnet 5.5): plays one stage in a sandbox copy of the
     repo's operating docs. It gets the stage skill, the hook context a real
     session gets, and read-only tools (Read/Glob/Grep plus
     `issueops project route-docs`). It decides for itself which docs and
     modules to read. Every tool call is traced.
  2. judge (`claude -p`, default Opus): sees the full doc catalog and decides,
     without the fixed GOLD list, which docs this stage needs. Then it scores the
     actor's reads and plan against a rubric, with the text of every doc the
     actor read so it can catch invented rules.

The fixed GOLD list is reported only as a cross-check of the judge.
Not deterministic and needs Claude Code auth + network: run on demand, not in
CI or self-verify.
"""

from __future__ import annotations

import argparse
import hashlib
import concurrent.futures
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import traceback
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import doc_injection_eval as det  # noqa: E402

# History and work-in-progress trees are not operating docs; leaving them out keeps
# the sandbox to what a stage is supposed to consult.
SANDBOX_EXCLUDE = {"issues", "plans", "research", "verified-execution", "evidence", "archive"}

RUBRIC = {
    "doc_selection": "Read the docs and modules the stage needs (judge's EXPECTED list) before acting, without detours into irrelevant ones.",
    "depth": "Followed index docs into the owning module or dated record when the rule lives there, instead of stopping at the index.",
    "rule_grounding": "Every cited rule exists in the cited source (the stage skill, AGENTS.md, or a doc whose text the agent saw). Invented rules, rules credited to the wrong file, or a doc's frontmatter description presented as a body rule score 0.",
    "decision_reflection": "Concrete decisions differ because of the cited rules, not boilerplate citations.",
    "recording": "Produces the record the skill requires for applied doc rules, at the required place and with doc paths.",
}

ACTOR_SCHEMA = {
    "type": "object",
    "properties": {
        "consulted_docs": {"type": "array", "items": {"type": "object", "properties": {
            "path": {"type": "string"}, "why": {"type": "string"},
            "rules": {"type": "array", "items": {"type": "string"}}}, "required": ["path", "why", "rules"]}},
        "order_of_actions": {"type": "array", "items": {"type": "string"}},
        # Not every decision comes from a project doc: forcing a doc path on each one
        # would make the actor credit skill rules to docs and inflate fabrication counts.
        "decisions": {"type": "array", "items": {"type": "object", "properties": {
            "decision": {"type": "string"}, "because": {"type": "string"},
            "source": {"type": "string", "description": "file the rule came from: 'stage skill', 'AGENTS.md', or a .issueops doc path"}},
            "required": ["decision", "because", "source"]}},
        "records": {"type": "array", "items": {"type": "string"}},
    },
    "required": ["consulted_docs", "order_of_actions", "decisions", "records"],
}

JUDGE_SCHEMA = {
    "type": "object",
    "properties": {
        "expected_docs": {"type": "array", "items": {"type": "object", "properties": {
            "path": {"type": "string"}, "why": {"type": "string"}}, "required": ["path", "why"]}},
        **{k: {"type": "object", "properties": {"score": {"type": "integer", "minimum": 0, "maximum": 4},
                                                  "evidence": {"type": "string"}}, "required": ["score", "evidence"]}
           for k in RUBRIC},
        "fabricated_rules": {"type": "array", "items": {"type": "string"}},
        "harness_artifacts": {"type": "array", "items": {"type": "string"},
                              "description": "rules taken from the evaluation system prompt or Claude Code's built-in tool descriptions"},
        "missed_rules": {"type": "array", "items": {"type": "string"}},
        "summary": {"type": "string"},
    },
    "required": ["expected_docs", *RUBRIC, "fabricated_rules", "harness_artifacts", "missed_rules", "summary"],
}

ACTOR_SYSTEM = (
    "You are the coding agent running exactly one IssueOps stage. The current directory holds the repository's "
    "AGENTS.md and its .issueops operating docs; application source and git history are not present. Assume the stage "
    "preconditions hold (git checkout, active lease, prior stages done) and plan the stage for the given task; do not run "
    "git or probe the environment, and do not stop on missing environment. Read-only tools are available. "
    "`issueops project route-docs --repo . --task \"<summary>\" --json` is the CLI form of project_docs_route. "
    "Finish with the structured result."
)

JUDGE_SYSTEM = (
    "You evaluate whether an agent consulted the right project documents for one IssueOps stage and let them shape "
    "its decisions. First decide from the catalog which docs or modules this stage and task need (expected_docs); do "
    "this independently, before looking at what the agent read. Then score each rubric item 0-4 (0 absent/wrong, 2 "
    "partial, 4 fully met) with short quoted evidence. Check every cited rule against the provided doc text. "
    "The agent also ran under the evaluation system prompt shown below and Claude Code's built-in tool descriptions "
    "(for example the Bash tool's git commit guidance, including a Co-Authored-By trailer). A rule that comes from either "
    "source is an evaluation artifact: list it under harness_artifacts, not fabricated_rules, and do not lower "
    "rule_grounding for it. Dated records (cautions/YYYY-MM-DD-*.md, adr/YYYY-MM-DD-*.md) state that incident-time "
    "commands, paths, and fields are historical evidence; when one conflicts with a living doc (an index such as "
    "CAUTIONS.md or CONVENTIONS.md, a module file, or the stage skill), the living doc is current. Do not count "
    "following the living doc as a miss.\n\nEvaluation system prompt given to the agent:\n" + ACTOR_SYSTEM
)


def build_sandbox(repo: Path, root: Path, bin_path: str) -> None:
    shutil.copy2(repo / "AGENTS.md", root / "AGENTS.md")
    shutil.copytree(repo / ".issueops", root / ".issueops",
                    ignore=lambda d, names: [n for n in names if Path(d) == repo / ".issueops" and n in SANDBOX_EXCLUDE])
    (root / "bin").mkdir()
    shutil.copy2(bin_path, root / "bin" / "issueops")


def catalog(root: Path) -> str:
    lines = []
    for p in sorted((root / ".issueops").rglob("*.md")):
        rel = p.relative_to(root).as_posix()
        text = p.read_text(encoding="utf-8", errors="replace")
        desc = re.search(r"^description:\s*(.+)$", text, re.M)
        title = re.search(r"^# (.+)$", text, re.M)
        lines.append(f"- {rel}: {(desc or title).group(1).strip() if (desc or title) else ''}")
    return "\n".join(lines)


def claude_stream(argv: list[str], prompt: str, cwd: Path, env: dict, timeout: int) -> dict:
    p = subprocess.run(argv, input=prompt, capture_output=True, text=True, timeout=timeout, cwd=cwd, env=env)
    events = []
    for line in p.stdout.splitlines():
        try:
            events.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    result = next((e for e in reversed(events) if e.get("type") == "result"), None)
    if p.returncode != 0 or result is None or result.get("is_error") or result.get("structured_output") is None:
        r = result or {}
        detail = r.get("result") or "; ".join(str(x) for x in (r.get("errors") or [])) or p.stderr.strip()[-300:]
        raise RuntimeError(f"claude exit {p.returncode} subtype={r.get('subtype')} turns={r.get('num_turns')}: {detail}")
    tools, by_id = [], {}
    for e in events:
        message = e.get("message")
        content = message.get("content") if isinstance(message, dict) else None
        for c in content if isinstance(content, list) else []:
            if not isinstance(c, dict):
                continue
            if e.get("type") == "assistant" and c.get("type") == "tool_use" and c["name"] != "StructuredOutput":
                by_id[c["id"]] = {"tool": c["name"], "input": c["input"], "output": ""}
                tools.append(by_id[c["id"]])
            elif e.get("type") == "user" and c.get("type") == "tool_result" and c.get("tool_use_id") in by_id:
                body = c.get("content")
                if isinstance(body, list):
                    body = "\n".join(x.get("text", "") for x in body if isinstance(x, dict))
                by_id[c["tool_use_id"]]["output"] = str(body or "")
    return {"data": result["structured_output"], "tools": tools, "cost": float(result.get("total_cost_usd") or 0)}


def judge_plain_json(a, prompt: str, cwd: Path, env: dict) -> dict:
    """Judge without --json-schema: the reply's JSON object is parsed and checked here.

    Used only after structured output failed, which happens when the model leaks tool-call
    markup into the StructuredOutput call. The required keys and score range are the schema's."""
    argv = ["claude", "-p", "--model", a.judge_model, "--effort", a.effort, "--strict-mcp-config",
            "--setting-sources", "project", "--no-session-persistence", "--output-format", "json", "--tools", "",
            "--system-prompt", JUDGE_SYSTEM + "\n\nReply with one JSON object only, matching this JSON Schema:\n"
            + json.dumps(JUDGE_SCHEMA)]
    p = subprocess.run(argv, input=prompt, capture_output=True, text=True, timeout=a.timeout, cwd=cwd, env=env)
    try:
        out = json.loads(p.stdout)
        text = out.get("result") or ""
        data = json.loads(text[text.index("{"):text.rindex("}") + 1])
    except (json.JSONDecodeError, ValueError) as e:
        raise RuntimeError(f"judge plain-JSON fallback failed: {e}: {p.stdout[-300:] or p.stderr[-300:]}")
    missing = [k for k in JUDGE_SCHEMA["required"] if k not in data]
    bad = [k for k in RUBRIC if k in data and not (isinstance(data[k], dict) and data[k].get("score") in range(5))]
    if missing or bad:
        raise RuntimeError(f"judge plain-JSON fallback invalid: missing={missing} bad_scores={bad}")
    return {"data": data, "tools": [], "cost": float(out.get("total_cost_usd") or 0)}


def base_argv(model: str, effort: str, system: str, schema: dict) -> list[str]:
    return ["claude", "-p", "--model", model, "--effort", effort, "--strict-mcp-config", "--setting-sources", "project",
            "--no-session-persistence", "--output-format", "stream-json", "--verbose",
            "--system-prompt", system, "--json-schema", json.dumps(schema)]


MD_TOKEN_RE = re.compile(r"[\w./-]+\.md\b")
CD_RE = re.compile(r"(?:^|[;&|]\s*)cd\s+([^\s;&|]+)")


def rel_reads(tools: list[dict], root: Path) -> list[str]:
    """Docs the actor opened, through Read or a shell command that names the file."""
    reads, real = [], str(root.resolve())
    for t in tools:
        if t["tool"] == "Read":
            given = Path((t["input"] if isinstance(t["input"], dict) else {}).get("file_path", ""))
            # The actor runs in the sandbox, so a relative path is relative to it, not to this process.
            fp = str((given if given.is_absolute() else root / given).resolve())
            found = [fp[len(real) + 1:] if fp.startswith(real + "/") else f"OUTSIDE:{fp}"]
        elif t["tool"] == "Bash":
            # A shell command may cd into .issueops (or a module dir) and then name files bare:
            # resolve every *.md token against the sandbox root and the directories the command cd'd into.
            command = (t["input"] if isinstance(t["input"], dict) else {}).get("command", "")
            bases = [root]
            for target in CD_RE.findall(command):
                d = (Path(target) if Path(target).is_absolute() else bases[-1] / target).resolve()
                if str(d).startswith(real):
                    bases.append(d)
            found = []
            for token in MD_TOKEN_RE.findall(command):
                for base in reversed(bases):
                    p = (Path(token) if Path(token).is_absolute() else base / token).resolve()
                    if str(p).startswith(real + "/") and p.is_file():
                        found.append(str(p)[len(real) + 1:])
                        break
        else:
            found = []
        for rel in found:
            if rel not in reads and (rel.startswith("OUTSIDE:") or (root / rel).is_file()):
                reads.append(rel)
    return reads


def index_of(rel: str) -> str | None:
    """Map a read path to the index doc name used by the fixed GOLD list."""
    m = re.fullmatch(r"\.issueops/([A-Z_]+\.md)", rel)
    if m:
        return m.group(1)
    m = re.fullmatch(r"\.issueops/([a-z_-]+)/.+\.md", rel)
    if not m:
        return None
    family = {"cautions": "CAUTIONS.md", "adr": "ADR.md", "testing": "TESTING.md", "conventions": "CONVENTIONS.md",
              "architecture": "ARCHITECTURE.md", "operations": "OPERATIONS.md"}
    return family.get(m.group(1))


ARTIFACT_PROMPT = (
    " Also produce the files this stage writes, in this directory: the plan file, verified-execution report, gate "
    "ledger, PR/issue body file, confirmation text shown to the user, or edits to existing .issueops docs, as the skill "
    "requires. Create and change files only with the Write and Edit tools (Bash here is read-only and cannot write). "
    "Put new files under out/. Commands that need git, a provider, or an IssueOps record cannot run here: write each "
    "exact command you would run, with its flag values filled in, to out/commands.sh."
)


def snapshot(root: Path) -> dict[str, str]:
    return {p.relative_to(root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in root.rglob("*") if p.is_file() and p.relative_to(root).parts[0] != "bin"}


def written_files(root: Path, before: dict[str, str]) -> str:
    after = snapshot(root)
    changed = sorted(rel for rel, digest in after.items() if before.get(rel) != digest)
    return "\n".join(
        f"<file path=\"{rel}\" state=\"{'modified' if rel in before else 'new'}\">\n"
        f"{(root / rel).read_text(encoding='utf-8', errors='replace')[:20000]}\n</file>" for rel in changed)


FIXTURES_DIR = Path(__file__).resolve().parent / "doc_injection_fixtures"


def fixture_block(fixture: Path) -> str:
    return "\n".join(f"<file path=\"in/{p.relative_to(fixture).as_posix()}\">\n{p.read_text(encoding='utf-8')}\n</file>"
                     for p in sorted(fixture.rglob("*")) if p.is_file())


FILE_BLOCK_RE = re.compile(r'<file path="([^"]+)" state="\w+">\n(.*?)\n</file>', re.S)


def chain_inputs(report: Path, source: str, dest: Path) -> Path:
    """Write the out/ files that scenario `source` produced in `report` into `dest`."""
    results = {r["id"]: r for r in json.loads(report.read_text(encoding="utf-8"))["results"]}
    if source not in results or not results[source].get("written"):
        raise SystemExit(f"--chain: {report} has no written files for scenario {source!r}")
    count = 0
    for rel, body in FILE_BLOCK_RE.findall(results[source]["written"]):
        if rel.startswith("out/"):
            path = dest / rel.removeprefix("out/")
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(body, encoding="utf-8")
            count += 1
    if not count:
        raise SystemExit(f"--chain: scenario {source!r} in {report} wrote nothing under out/")
    return dest


def run_scenario(sc: dict, shared_root: Path, repo: Path, bin_path: str, env: dict, cat: str, a) -> dict:
    fixture = a.chain_dirs.get(sc["id"]) or FIXTURES_DIR / sc["id"]
    isolated = a.write_artifacts or fixture.is_dir()
    if isolated:
        # A scenario that writes or carries inputs gets its own copy, so nothing leaks into another scenario.
        scenario_dir = tempfile.mkdtemp(prefix=f"di-{sc['id']}-")
        root = Path(scenario_dir) / "repo"
        shutil.copytree(shared_root, root)
        if fixture.is_dir():
            shutil.copytree(fixture, root / "in")
    else:
        root = shared_root
    inputs = fixture_block(fixture) if fixture.is_dir() else ""
    hook = det_hook(bin_path, root, sc, env)  # sandbox root: hook output must not reveal the real repo path
    skill = (repo / "skills" / sc["skill"] / "SKILL.md").read_text(encoding="utf-8")
    agents = (root / "AGENTS.md").read_text(encoding="utf-8")
    actor_prompt = (
        f"# Project instructions (AGENTS.md, auto-loaded)\n<agents>\n{agents}\n</agents>\n\n"
        f"# Session context injected by hooks\n{hook or '(none)'}\n\n"
        f"# Stage\n{sc['stage']} (skill {sc['skill']})\n\n# Task\n{sc['task']}\n\n"
        f"# Stage skill\n<skill>\n{skill}\n</skill>\n\n"
        + (f"# Stage inputs (also on disk under in/)\nThe plan, diff, or gate ledger this stage works on. "
           f"Treat them as the cycle's real artifacts.\n{inputs}\n\n" if inputs else "")
        + "Run this stage for the task up to its decisions: read what you need, then return which docs you consulted "
        "and the rules that apply, the order of your actions, each concrete decision with the rule and the file it came from "
        "(the stage skill, AGENTS.md, or a doc path), and the records the skill requires."
        + (ARTIFACT_PROMPT if a.write_artifacts else "")
    )
    tools, allowed, mode = "Read,Glob,Grep,Bash", ["Read", "Glob", "Grep", "Bash(issueops project route-docs:*)"], []
    if a.write_artifacts:
        tools += ",Write,Edit"
        # acceptEdits auto-approves file edits inside the working directory (the per-scenario sandbox) only.
        mode = ["--permission-mode", "acceptEdits"]
    actor_argv = base_argv(a.actor_model, a.effort, ACTOR_SYSTEM, ACTOR_SCHEMA) + [
        "--tools", tools, *mode, "--allowedTools", *allowed]
    before = snapshot(root)
    try:
        actor = claude_stream(actor_argv, actor_prompt, root, env, a.timeout)
        written = written_files(root, before) if a.write_artifacts else ""
        reads = rel_reads(actor["tools"], root)
    finally:
        if isolated:
            shutil.rmtree(scenario_dir, ignore_errors=True)
    trace = [f"{t['tool']} {json.dumps(t['input'], ensure_ascii=False)[:200]}" for t in actor["tools"]]
    # The judge sees exactly what each tool call returned, so a grep that showed three lines
    # does not count as having read the whole doc.
    observed = "\n".join(
        f"<tool_call n=\"{i}\" call={json.dumps(trace[i], ensure_ascii=False)}>\n{t['output'][:40000]}\n</tool_call>"
        for i, t in enumerate(actor["tools"]))
    judge_prompt = (
        f"# Stage\n{sc['stage']} (skill {sc['skill']})\n\n# Task\n{sc['task']}\n\n"
        f"# AGENTS.md (auto-loaded into the agent's context; citing it needs no tool call)\n<agents>\n{agents}\n</agents>\n\n"
        f"# Stage skill (also in the agent's context)\n<skill>\n{skill}\n</skill>\n\n"
        f"# Doc catalog available to the agent\n{cat}\n\n# Rubric\n{json.dumps(RUBRIC, indent=1)}\n\n"
        f"# What each agent tool call returned (all doc text the agent saw beyond AGENTS.md and the skill)\n"
        f"{observed or '(no tool calls)'}\n\n"
        + (f"# Stage inputs the agent was given (the plan/diff/ledger it had to judge)\n{inputs}\n\n"
           "For decision_reflection, check whether the agent found the problems in these inputs that the project "
           "docs forbid, and whether each finding cites the doc rule it breaks.\n\n" if inputs else "")
        + (f"# Files the agent created or modified (judge recording from these, not from promises in the output)\n"
           f"{written or '(none)'}\n\n" if a.write_artifacts else "")
        + f"# Agent output\n{json.dumps(actor['data'], ensure_ascii=False, indent=1)}\n"
    )
    judge_argv = base_argv(a.judge_model, a.effort, JUDGE_SYSTEM, JUDGE_SCHEMA) + ["--tools", ""]
    with tempfile.TemporaryDirectory() as jcwd:
        try:
            judge = claude_stream(judge_argv, judge_prompt, Path(jcwd), env, a.timeout)
        except RuntimeError as e:
            # The judge is stateless, so a malformed structured output can be retried without
            # rerunning the actor; any other failure still fails the scenario.
            if "error_max_structured_output_retries" not in str(e):
                raise
            judge = judge_plain_json(a, judge_prompt, Path(jcwd), env)
    j = judge["data"]
    scores = {k: j[k]["score"] for k in RUBRIC}
    expected = {(e["path"] if isinstance(e, dict) else str(e)).removeprefix("./") for e in j["expected_docs"]}
    read_set = set(reads)
    expected_index = {i for i in map(index_of, expected) if i}
    read_index = {i for i in map(index_of, reads) if i}
    gold = set(sc["gold"])
    return {
        "id": sc["id"], "stage": sc["stage"], "score": 100 * sum(scores.values()) / (4 * len(RUBRIC)),
        "scores": scores, "reads": reads, "expected": sorted(expected),
        "read_recall": len(expected & read_set) / len(expected) if expected else 1.0,
        "index_recall": len(expected_index & read_index) / len(expected_index) if expected_index else 1.0,
        "route_used": any(t["tool"] == "Bash" and "route-docs" in json.dumps(t["input"]) for t in actor["tools"]),
        "gold_vs_judge": {"gold_only": sorted(gold - expected_index), "judge_only": sorted(expected_index - gold)},
        "fabricated_rules": j["fabricated_rules"], "harness_artifacts": j.get("harness_artifacts", []),
        "missed_rules": j["missed_rules"], "summary": j["summary"],
        "trace": trace, "denied": [t["output"][:300] for t in actor["tools"] if t["tool"] in ("Write", "Edit") and "denied" in t["output"].lower()],
        "actor": actor["data"], "judge": j, "written": written, "cost_usd": actor["cost"] + judge["cost"],
    }


def det_hook(bin_path: str, repo: Path, sc: dict, env: dict) -> str:
    """Full hook context (catalog included): what a real session sees."""
    out = []
    payload = {"hook_event_name": "SessionStart", "cwd": str(repo), "source": "startup"}
    code, stdout = det.run(bin_path, ["hook", "session-start", "--host", "claude", "--repo", str(repo)],
                           json.dumps(payload), env)
    if code == 0 and stdout.strip():
        out.extend(det.collect_context(json.loads(stdout)))
    extra = det.hook_context(bin_path, repo, sc, env)  # non-catalog context from other probed events
    if extra:
        out.append(extra)
    return "\n".join(out)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", required=True)
    ap.add_argument("--repo", required=True)
    ap.add_argument("--scenarios", required=True)
    ap.add_argument("--only", help="comma-separated scenario ids")
    ap.add_argument("--actor-model", default="claude-sonnet-5-5")
    ap.add_argument("--judge-model", default="claude-opus-5-5")
    ap.add_argument("--effort", default="medium")
    ap.add_argument("--jobs", type=int, default=6)
    ap.add_argument("--timeout", type=int, default=900)
    ap.add_argument("--write-artifacts", action="store_true",
                    help="let the actor write the stage's files in a per-scenario sandbox and judge recording from them")
    ap.add_argument("--chain", action="append", default=[], metavar="TARGET=REPORT.json:SOURCE",
                    help="give scenario TARGET the out/ files that scenario SOURCE wrote in an earlier "
                         "--write-artifacts report, as its in/ inputs (replaces any fixture)")
    ap.add_argument("--out", required=True, help="JSON report path")
    a = ap.parse_args()

    repo = Path(a.repo).resolve()
    scenarios = json.loads(Path(a.scenarios).read_text(encoding="utf-8"))["scenarios"]
    if a.only:
        wanted = set(a.only.split(","))
        scenarios = [s for s in scenarios if s["id"] in wanted]
    chain_root = Path(tempfile.mkdtemp(prefix="di-chain-"))
    a.chain_dirs = {}
    for spec in a.chain:
        target, _, rest = spec.partition("=")
        report, _, source = rest.rpartition(":")
        a.chain_dirs[target] = chain_inputs(Path(report), source, chain_root / target)

    started = time.time()
    results, failures = [], {}
    with tempfile.TemporaryDirectory() as state, tempfile.TemporaryDirectory() as sandbox:
        root = Path(sandbox)
        build_sandbox(repo, root, a.bin)
        cat = catalog(root)
        env = {k: v for k, v in os.environ.items() if not k.startswith("ISSUEOPS_")}
        env.update({"ISSUEOPS_STATE_DIR": state, "NO_COLOR": "1", "TZ": "UTC",
                    "PATH": f"{root / 'bin'}{os.pathsep}{env.get('PATH', '')}"})
        with concurrent.futures.ThreadPoolExecutor(max_workers=a.jobs) as pool:
            futures = {pool.submit(run_scenario, sc, root, repo, a.bin, env, cat, a): sc["id"] for sc in scenarios}
            for f in concurrent.futures.as_completed(futures):
                try:
                    results.append(f.result())
                except Exception as e:  # report and keep the rest of the batch
                    failures[futures[f]] = f"{type(e).__name__}: {e} @ {traceback.extract_tb(e.__traceback__)[-1].lineno}"

    order = {s["id"]: i for i, s in enumerate(scenarios)}
    results.sort(key=lambda r: order[r["id"]])
    keys = list(RUBRIC)
    print(f"{'scenario':<20}{'score':>6} " + " ".join(f"{k[:6]:>6}" for k in keys) + f"{'readR':>7}{'idxR':>6} route fab")
    for r in results:
        print(f"{r['id']:<20}{r['score']:6.1f} " + " ".join(f"{r['scores'][k]:>6}" for k in keys)
              + f"{r['read_recall']:7.2f}{r['index_recall']:6.2f} {'y' if r['route_used'] else '-':>5} {len(r['fabricated_rules']):>3}")
    for sid, err in failures.items():
        print(f"{sid:<20} FAILED: {err}")
    if results:
        n = len(results)
        mean = lambda f: sum(f(r) for r in results) / n
        print(f"mean score={mean(lambda r: r['score']):.1f} " + " ".join(
            f"{k}={mean(lambda r, k=k: r['scores'][k]):.2f}" for k in keys)
            + f" read_recall={mean(lambda r: r['read_recall']):.2f} index_recall={mean(lambda r: r['index_recall']):.2f}"
            + f" route_used={mean(lambda r: r['route_used']):.2f} fabricated={sum(len(r['fabricated_rules']) for r in results)}")
    cost = sum(r["cost_usd"] for r in results)
    print(f"actor={a.actor_model} judge={a.judge_model} effort={a.effort} cost=${cost:.2f} wall={time.time() - started:.0f}s")
    Path(a.out).write_text(json.dumps({"actor_model": a.actor_model, "judge_model": a.judge_model, "effort": a.effort,
                                       "results": results, "failures": failures}, ensure_ascii=False, indent=2),
                           encoding="utf-8")
    shutil.rmtree(chain_root, ignore_errors=True)
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
