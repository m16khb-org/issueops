#!/usr/bin/env bash
# issueops usability benchmark (autoresearch harness).
#
# Workload (offline, deterministic):
#   1. gate: gofmt, go vet ./..., go test ./... (cached; any failure exits non-zero)
#   2. build a fresh issueops binary
#   3. drive a scripted IssueOps cycle (start -> ... -> pr-readiness) in a throwaway
#      git repo under a fake `claude` parent process, with isolated HOME/state
#   4. measure session-fixed model-facing payloads (hook, MCP tools/list,
#      MCP instructions, skill descriptions)
#   5. run usability contract probes (error envelopes, exit codes, stderr noise)
#   6. time a few commands (median of N runs)
#
# Primary metric: agent_kb = KB of model-facing text an agent receives for one
# cycle (weighted command outputs) + per-session fixed payloads x SESSIONS.
# Lower is better.
set -euo pipefail

REPO="$(cd "$(dirname "$0")" && pwd)"
BENCH=/tmp/io-ar-bench
rm -rf "$BENCH"
mkdir -p "$BENCH"
cd "$REPO"

# ---- 1. correctness gate -------------------------------------------------
unformatted="$(gofmt -l $(git ls-files '*.go') || true)"
if [ -n "$unformatted" ]; then
  echo "gofmt needed:"; echo "$unformatted"; exit 1
fi
go vet ./... >"$BENCH/vet.log" 2>&1 || { cat "$BENCH/vet.log"; exit 1; }
t0=$(python3 -c 'import time;print(time.time())')
if ! go test ./... >"$BENCH/test.log" 2>&1; then
  grep -E '^(--- FAIL|FAIL|panic)' "$BENCH/test.log" | head -40
  exit 1
fi
t1=$(python3 -c 'import time;print(time.time())')

# ---- 2. build ------------------------------------------------------------
go build -o "$BENCH/issueops" ./cmd/issueops

mkdir -p "$BENCH/fakehost"
cat >"$BENCH/fakehost/main.go" <<'EOF'
package main

import (
	"errors"
	"os"
	"os/exec"
)

func main() {
	c := exec.Command("/bin/sh", "-c", os.Args[1])
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
EOF
printf 'module fakehost\n\ngo 1.26\n' >"$BENCH/fakehost/go.mod"
(cd "$BENCH/fakehost" && GOFLAGS= go build -o "$BENCH/claude" .)

# ---- 3-6. measurements ---------------------------------------------------
# The whole driver runs under ONE fake `claude` process so every issueops call
# shares the same native-session ancestry (as in a real host session).
cat >"$BENCH/driver.py" <<'PY'
import json, os, re, shlex, statistics, subprocess, sys, time
from pathlib import Path

BENCH = Path(os.environ["BENCH"]); REPO = Path(os.environ["REPO"])
IO = str(BENCH / "issueops")
HOME = BENCH / "home"; STATE = HOME / "state"; R = BENCH / "repo"
HOME.mkdir(); STATE.mkdir()
gitcfg = BENCH / "gitconfig"
gitcfg.write_text("[user]\n\temail = bench@example.com\n\tname = bench\n[init]\n\tdefaultBranch = main\n")
ENV = {k: v for k, v in os.environ.items() if k not in (
    "CODEX_THREAD_ID", "PI_SESSION_ID", "ISSUEOPS_OMP_SESSION_ID", "OPENCODE_SESSION_ID",
    "CLAUDECODE", "CLAUDE_CODE_SESSION_ID", "ISSUEOPS_STATE_DIR", "ISSUEOPS_ROOT")}
ENV.update(HOME=str(HOME), ISSUEOPS_STATE_DIR=str(STATE), CLAUDE_CODE_SESSION_ID="abc",
           GIT_CONFIG_GLOBAL=str(gitcfg), GIT_CONFIG_NOSYSTEM="1", LANG="C", LC_ALL="C",
           ISSUEOPS_DISABLE_UPDATE_CHECK="1")

def sh(cmd, cwd):
    subprocess.run(cmd, cwd=cwd, env=ENV, shell=True, check=True, capture_output=True)

def run(args, cwd, stdin=None):
    """Run issueops as a child of the driver; return (rc, stdout, stderr, ms)."""
    t = time.perf_counter()
    p = subprocess.run([IO, *args], cwd=cwd, env=ENV, capture_output=True, input=stdin)
    return p.returncode, p.stdout.decode(errors="replace"), p.stderr.decode(errors="replace"), (time.perf_counter() - t) * 1000

# --- throwaway repo -------------------------------------------------------
R.mkdir()
sh("git init -q && mkdir -p .issueops && printf '# Cautions\\n' > .issueops/CAUTIONS.md && echo a > a.txt && git add -A && git commit -qm init", R)
sha = subprocess.run(["git", "rev-parse", "HEAD"], cwd=R, env=ENV, capture_output=True, text=True).stdout.strip()

outputs = []   # (label, weight, rc, stdout, stderr)
failures = []
cycle_ms = 0.0
def step(label, args, cwd, weight=1, expect_ok=True):
    global cycle_ms
    rc, out, err, ms = run(args, cwd)
    cycle_ms += ms
    outputs.append((label, weight, rc, out, err))
    (BENCH / "out").mkdir(exist_ok=True)
    (BENCH / "out" / f"{len(outputs):02d}-{label}.txt").write_text(f"rc={rc}\n{out}\n--- stderr\n{err}")
    if expect_ok and rc != 0:
        failures.append(f"{label}: rc={rc} {(out + err)[:300]}")
    return rc, out, err

rc, out, err = step("start", ["start", "--repo", str(R), "--new", "--json"], R)
cid = json.loads(out)["id"]
RF = ["--host", "claude", "--session-id", "abc", "--cwd", str(R)]
rc, out, err = step("whoami", ["execution", "whoami", "--json"], R)
AF = shlex.split(json.loads(out)["claim_actor_flags"][0])

step("intent", ["intent", "record", "--id", cid, "--raw-request", "make X faster", "--interpreted-intent", "reduce latency", "--success-criteria", "p50 < 10ms", *RF, "--json"], R)
step("domain", ["domain-review", "record", "--id", cid, "--model-fit", "ok", "--terminology", "t", "--risk", "r", "--uncertainty", "u", *RF, "--json"], R)
step("link-issue", ["link-issue", "--id", cid, "--issue-url", "https://github.com/o/r/issues/1", *RF, "--json"], R)
step("phase-grill", ["phase", "--id", cid, "--to", "grill", *RF, "--json"], R)
step("plan-prep", ["plan-prep", "record", "--id", cid, "--decisions-waive", "w", "--related-waive", "w", "--web-research-waive", "w", "--codebase-survey-waive", "w", *RF, "--json"], R)
step("decision", ["decision", "add", "--id", cid, "--title", "pool buffers", "--body", "use a pooled buffer", "--kind", "architecture", *RF, "--json"], R)
sh("git checkout -q -b 1-make-x-faster && git checkout -q main", R)
step("branch-prepare", ["branch", "prepare", "--id", cid, "--provider", "github", "--issue-url", "https://github.com/o/r/issues/1", "--branch", "1-make-x-faster", "--base-branch", "main", "--base-sha", sha, "--link-verified", *RF, "--json"], R)
(R / "arch.md").write_text("# Arch\n## 요구사항과 규모\nx\n## 데이터와 API 계약\nx\n## 구조와 흐름\nx\n## 병목과 장애\nx\n## 절충점과 기각한 대안\nx\n")
(R / "plan.md").write_text("# plan\n## 적용되는 결정과 주의사항\nx\n## 재사용하는 기존 구현\nx\n## 성능 영향\nx\n## 하위 호환성과 side effect\nx\n")
step("stage-arch", ["artifact", "stage", "--id", cid, "--name", "architecture", "--file", str(R / "arch.md"), "--json"], R)
step("arch-review", ["architecture", "review", "--id", cid, "--problem-summary", "slow X: hot loop allocates per request", "--proposed-architecture", "pooled buffer plus per-request cache", "--verification", "architecture review checked alternatives", "--refactor-plan", "extract buffer pool", "--alternative", "sync.Pool only", "--risk", "stale cache", "--reviewer-context", "subagent", "--approved", *RF, "--json"], R)
step("stage-plan", ["artifact", "stage", "--id", cid, "--name", "plan", "--file", str(R / "plan.md"), "--json"], R)
step("da-review", ["devils-advocate", "review", "--id", cid, "--verdict", "pass", "--reviewer-context", "subagent", "--finding", "none blocking", *RF, "--json"], R)
step("next-plan", ["next", "--id", cid, "--json"], R, weight=2)
step("status-plan", ["status", "--id", cid, "--json"], R, weight=2)
step("exec-prepare-preview", ["execution", "prepare", "--id", cid, *AF, "--json"], R)
step("exec-prepare", ["execution", "prepare", "--id", cid, *AF, "--confirm", "--json"], R)
W = Path(str(R) + ".worktrees") / "1-make-x-faster"
RFW = ["--host", "claude", "--session-id", "abc", "--cwd", str(W)]
(W / ".issueops/issues/1").mkdir(parents=True, exist_ok=True)
(W / ".issueops/issues/1/plan.md").write_text((R / "plan.md").read_text())
step("exec-status", ["execution", "status", "--id", cid, "--json"], W, weight=2)
step("link-plan", ["link-plan", "--id", cid, "--plan-path", str(W / ".issueops/issues/1/plan.md"), *RFW, "--json"], W)
step("compat", ["compatibility", "review", "--id", cid, "--backward-compatibility", "no API change", "--side-effect", "memory +2MB", "--rollback-plan", "revert commit", "--verification", "go test ./...", "--approved", *RFW, "--json"], W)
step("phase-impl", ["phase", "--id", cid, "--to", "implement", *RFW, "--json"], W)
step("next-impl", ["next", "--id", cid, "--json"], W, weight=2)
sh("echo change > b.txt && git add b.txt && git commit -qm 'feat: b'", W)
step("phase-clean", ["phase", "--id", cid, "--to", "ai-slop-clean", *RFW, "--json"], W)
step("slop-record", ["ai-slop-clean", "record", "--id", cid, "--category", "dead code", "--verification", "go test ./...", *RFW, "--json"], W)
step("next-clean", ["next", "--id", cid, "--json"], W, weight=2)
step("docs-review", ["project-docs-review", "record", "--id", cid, "--verdict", "no-change", "--reviewed-doc", ".issueops/CAUTIONS.md", "--evidence", "checked cautions", *RFW, "--json"], W)
step("impl-review", ["implementation-review", "record", "--id", cid, "--verdict", "pass", "--finding", "none", "--evidence", "diff reviewed", *RFW, "--json"], W)
step("status-final", ["status", "--id", cid, "--json"], W, weight=2)
step("next-final", ["next", "--id", cid, "--json"], W, weight=2)
step("pr-readiness", ["pr-readiness", "--id", cid, "--strict", "--json"], W, weight=1, expect_ok=False)
step("list", ["list", "--repo", str(R), "--json"], R)

if failures:
    print("cycle failures:"); print("\n".join(failures)); sys.exit(1)

cycle_bytes = sum(w * (len(o.encode()) + len(e.encode())) for _, w, _, o, e in outputs)
null_fields = sum(len(re.findall(r'":\s*null', o)) for _, _, _, o, _ in outputs)
max_out = max(len(o.encode()) for _, _, _, o, _ in outputs)

# --- session-fixed payloads ---------------------------------------------
rc, hook, _, _ = run(["hook", "session-start", "--repo", str(REPO), "--host", "claude", "--json"], REPO)
rc, sub, _, _ = run(["hook", "subagent-start", "--repo", str(REPO), "--host", "claude", "--json"], REPO)
msgs = [{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "bench", "version": "0"}}},
        {"jsonrpc": "2.0", "method": "notifications/initialized"},
        {"jsonrpc": "2.0", "id": 2, "method": "tools/list"}]
p = subprocess.run([IO, "mcp"], cwd=REPO, env=ENV, capture_output=True, timeout=60,
                   input="".join(json.dumps(m) + "\n" for m in msgs).encode())
instructions = tools_list = 0
for line in p.stdout.decode().splitlines():
    if not line.strip():
        continue
    d = json.loads(line)
    if d.get("id") == 1:
        instructions = len(d["result"].get("instructions", "").encode())
    if d.get("id") == 2:
        tools_list = len(json.dumps(d["result"]["tools"], ensure_ascii=False).encode())
if not tools_list:
    print("MCP tools/list failed:", p.stderr.decode()[:500]); sys.exit(1)
skill_desc = 0
stage_skill = 0
for f in sorted((REPO / "skills").glob("*/SKILL.md")):
    text = f.read_text()
    m = re.search(r"^description:\s*(.*)$", text, re.M)
    skill_desc += len(f.parent.name.encode()) + len((m.group(1) if m else "").encode())
    if f.parent.name.startswith("issueops"):
        stage_skill += len(text.encode())

SESSIONS = 4      # median agent sessions per cycle (transcript mining)
SUBAGENTS = 4
session_bytes = SESSIONS * (len(hook.encode()) + instructions + skill_desc) + SUBAGENTS * len(sub.encode()) + tools_list

# --- usability probes -----------------------------------------------------
defects = []
def single_json(text):
    try:
        json.loads(text); return True
    except Exception:
        return False
probe_bytes = 0
# P1: --json errors must be exactly one JSON document even with 2>&1.
rc, out, err, _ = run(["status", "--id", "io-000000000000", "--json"], R)
probe_bytes += len(out) + len(err)
if not single_json(out + err): defects.append("status-json-error-not-single-document")
# P2: state read --json for a missing key must answer with JSON on stdout.
rc, out, err, _ = run(["state", "read", "--key", "missing-key", "--json"], R)
probe_bytes += len(out) + len(err)
if not single_json(out + err): defects.append("state-read-json-error-not-json")
# P3: unknown command must not dump the whole usage (>4KB).
rc, out, err, _ = run(["no-such-command"], R)
probe_bytes += len(out) + len(err)
if len(out) + len(err) > 4096: defects.append("unknown-command-dumps-usage")
# P4: mutation failures with --json under 2>&1 parse as one document.
rc, out, err, _ = run(["decision", "add", "--id", cid, "--title", "t", "--body", "b", "--kind", "architecture", "--host", "claude", "--session-id", "abc", "--cwd", str(R), "--json"], R)
probe_bytes += len(out) + len(err)
if not single_json(out + err): defects.append("mutation-json-error-not-single-document")
# P5: record commands accept the claim actor flags printed by whoami.
rc, out, err, _ = run(["decision", "add", "--id", cid, "--title", "t", "--body", "b", "--kind", "architecture", *AF, "--json"], W)
probe_bytes += len(out) + len(err)
if "flag provided but not defined" in out + err: defects.append("record-command-rejects-claim-actor-flags")
# P6: no internal telemetry on stderr during the cycle.
if any('"event":"issueops_record_span"' in e for _, _, _, _, e in outputs): defects.append("telemetry-on-stderr")
# P7: a not-ready readiness verdict is not a process failure.
pr = [o for o in outputs if o[0] == "pr-readiness"][0]
if pr[2] != 0: defects.append("readiness-verdict-exit-nonzero")
# P8: cycle JSON has no null where a list is expected (count reported separately).
# P9: help on a subcommand goes to stdout so `--help | grep` works.
rc, out, err, _ = run(["decision", "add", "--help"], R)
probe_bytes += len(out) + len(err)
if not out.strip(): defects.append("subcommand-help-on-stderr")

# --- latency --------------------------------------------------------------
def med(args, cwd, n=7):
    ts = []
    for _ in range(n):
        ts.append(run(args, cwd)[3])
    return statistics.median(ts[1:])
lat_status = med(["status", "--id", cid, "--json"], W)
lat_next = med(["next", "--id", cid, "--json"], W)
lat_version = med(["version"], R)
lat_sysstat = med(["system-status", "--repo", str(R), "--json"], R, n=4)
rc, docs, _, _ = run(["docs", "--json"], REPO)
rc, inspect, _, _ = run(["inspect", "--json"], REPO)

agent_bytes = cycle_bytes + session_bytes + probe_bytes
def M(k, v): print(f"METRIC {k}={v}")
M("agent_kb", round(agent_bytes / 1000, 2))
M("cycle_out_kb", round(cycle_bytes / 1000, 2))
M("session_fixed_kb", round(session_bytes / 1000, 2))
M("tools_list_kb", round(tools_list / 1000, 2))
M("skill_desc_kb", round(skill_desc / 1000, 2))
M("hook_kb", round(len(hook.encode()) / 1000, 2))
M("stage_skill_kb", round(stage_skill / 1000, 2))
M("max_cmd_out_kb", round(max_out / 1000, 2))
M("null_fields", null_fields)
M("usability_defects", len(defects))
M("docs_kb", round(len(docs.encode()) / 1000, 1))
M("inspect_kb", round(len(inspect.encode()) / 1000, 1))
M("cycle_ms", round(cycle_ms))
M("status_ms", round(lat_status, 1))
M("next_ms", round(lat_next, 1))
M("version_ms", round(lat_version, 1))
M("system_status_ms", round(lat_sysstat, 1))
M("test_s", os.environ["TEST_SECONDS"])
print("ASI defects=" + ",".join(defects))
for label, w, rc, o, e in outputs:
    print(f"# {label:22s} w={w} rc={rc} out={len(o.encode())} err={len(e.encode())}")
PY
export TEST_SECONDS BENCH REPO
TEST_SECONDS=$(python3 -c "print(round($t1-$t0,1))")
BENCH="$BENCH" REPO="$REPO"
"$BENCH/claude" "python3 '$BENCH/driver.py'"
