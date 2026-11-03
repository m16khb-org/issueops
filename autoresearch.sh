#!/usr/bin/env bash
# Deterministic issueops performance + quality benchmark.
# Primary: cli_cpu_ms = sum over a fixed read-only CLI workload of the median
# CPU time (user+sys, children) per command. CPU time, not wall time, because
# the host runs other agents and wall time swings 2-5x.
# Secondary: complexity/duplication counts, policy correctness, route-docs
# recall/precision on a fixed citation set, binary size.
# No network: analyzers are built from the local module cache through a file
# GOPROXY; issueops commands run with a temp ISSUEOPS_STATE_DIR.
set -euo pipefail
cd "$(dirname "$0")"
ROOT=$PWD
WORK=$(mktemp -d "${TMPDIR:-/tmp}/issueops-ar.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

TOOLS=${AR_TOOLS_DIR:-/tmp/ar-tools}
if [ ! -x "$TOOLS/gocyclo" ] || [ ! -x "$TOOLS/dupl" ]; then
  mkdir -p "$TOOLS"
  GOPROXY="file://$(go env GOMODCACHE)/cache/download" GOSUMDB=off GOBIN="$TOOLS" \
    go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0 github.com/mibk/dupl@v1.0.0 >/dev/null
fi

go build -o "$WORK/issueops" ./cmd/issueops

ROOT="$ROOT" WORK="$WORK" TOOLS="$TOOLS" python3 - <<'PY'
import json, os, resource, statistics, subprocess, tempfile

root, work, tools = os.environ["ROOT"], os.environ["WORK"], os.environ["TOOLS"]
binary = os.path.join(work, "issueops")
state = tempfile.mkdtemp(dir=work)
env = dict(os.environ, ISSUEOPS_STATE_DIR=state, NO_COLOR="1")

def run(argv, cwd=root, check=True):
    before = resource.getrusage(resource.RUSAGE_CHILDREN)
    proc = subprocess.run(argv, cwd=cwd, env=env, capture_output=True, text=True)
    after = resource.getrusage(resource.RUSAGE_CHILDREN)
    cpu = (after.ru_utime - before.ru_utime) + (after.ru_stime - before.ru_stime)
    if check and proc.returncode != 0:
        raise SystemExit(f"command failed rc={proc.returncode}: {argv}\n{proc.stderr[-2000:]}")
    return cpu, proc

workload = {
    "version": ["version"],
    "inspect": ["inspect", "--json"],
    "docs": ["docs", "--json"],
    "next": ["next", "--json"],
    "list": ["list", "--repo", ".", "--json"],
    "route_en": ["project", "route-docs", "--repo", ".", "--task", "create-pr caller crashes before the response", "--json"],
    "route_ko": ["project", "route-docs", "--repo", ".", "--task", "SQLite span lock에 busy 재시도 추가", "--json"],
    "policy": ["policy", "check", "--workspace-root", root, "--cwd", root, "--json", "--", "git", "status", "--short"],
    "doctor_static": ["doctor", "--static-only", "--repo", ".", "--json"],
}
reps = int(os.environ.get("AR_REPS", "15"))
for argv in workload.values():  # warm page cache once
    run([binary] + argv, check=False)
medians = {}
for name, argv in workload.items():
    samples = [run([binary] + argv, check=name not in ("next",))[0] for _ in range(reps)]
    medians[name] = statistics.median(samples) * 1000
total = sum(medians.values())
print(f"METRIC cli_cpu_ms={total:.1f}")
for name, value in medians.items():
    print(f"METRIC cpu_{name}_ms={value:.1f}")

# Policy correctness: write-capable argv must be denied in the read-only tier,
# plain reads must stay allowed.
ws = tempfile.mkdtemp(dir=work)
subprocess.run(["git", "init", "-q"], cwd=ws, check=True)
open(os.path.join(ws, "g.txt"), "w").write("keep\n")
dangerous = [
    "find ./g.txt -delete", "find . -exec rm {} ;", "sed -i .bak s/k/x/ g.txt", "awk BEGIN{print}",
    "git branch -D main", "git config user.name x --get", "git config core.pager x", "git remote add o u",
    "rg --pre ./x foo", "go env -w GOFLAGS=-x", "go list -mod=mod ./...",
]
safe = [
    "find . -name g.txt", "rg keep", "git status --short", "git branch", "git config --get user.name",
    "git config --global user.name", "git remote -v", "git log --oneline", "go env GOPATH", "go list ./...", "cat g.txt",
]
def allowed(line):
    _, proc = run([binary, "policy", "check", "--workspace-root", ws, "--cwd", ws, "--json", "--"] + line.split(" "), cwd=ws, check=False)
    return json.loads(proc.stdout)["allowed"]
print(f"METRIC policy_dangerous_allowed={sum(allowed(l) for l in dangerous)}")
print(f"METRIC policy_safe_denied={sum(not allowed(l) for l in safe)}")

# route-docs recall/precision against records the plans actually cited.
tasks = json.load(open(os.path.join(root, "testdata/autoresearch/route_docs_tasks.json")))
hit = returned = cited = 0
for task in tasks:
    _, proc = run([binary, "project", "route-docs", "--repo", ".", "--task", task["title"], "--json"])
    got = [d["rel_path"] for d in json.loads(proc.stdout).get("docs", []) if "/cautions/2" in d["rel_path"] or "/adr/2" in d["rel_path"]]
    want = set(task["cited"])
    hit += sum(g in want for g in got); returned += len(got); cited += len(want)
print(f"METRIC route_recall={hit / cited:.4f}")
print(f"METRIC route_precision={(hit / returned if returned else 0):.4f}")

# Structural quality of non-test production code.
cyclo = subprocess.run([os.path.join(tools, "gocyclo"), "-ignore", "_test\\.go$", "cmd", "internal"], cwd=root, capture_output=True, text=True).stdout.split("\n")
scores = [int(line.split()[0]) for line in cyclo if line.strip()]
print(f"METRIC cyclo_over12={sum(s > 12 for s in scores)}")
print(f"METRIC cyclo_over20={sum(s > 20 for s in scores)}")
print(f"METRIC cyclo_excess={sum(max(0, s - 12) for s in scores)}")
dupl = subprocess.run([os.path.join(tools, "dupl"), "-t", "100", "-plumbing", "cmd", "internal"], cwd=root, capture_output=True, text=True).stdout.split("\n")
pairs = 0
for line in dupl:
    parts = line.split(": duplicate of ")
    if len(parts) == 2 and "_test.go" not in parts[0] and "_test.go" not in parts[1]:
        pairs += 1
print(f"METRIC dupl_prod_pairs={pairs}")
print(f"METRIC binary_kb={os.path.getsize(binary) / 1024:.0f}")
PY
