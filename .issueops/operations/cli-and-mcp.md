---
name: cli-and-mcp.md
description: Direct CLI, shared HTTP and stdio MCP, policy, guard, worker, and command smoke operations.
---

# CLI And MCP Operations

## Direct CLI

```bash
issueops version
issueops inspect --json
issueops system-status --json
issueops preflight --json /path/to/git-repo
issueops docs --json
issueops doctor --repo . --json
issueops guard check --staged --json
issueops verify-work --json -- git status --short
issueops quality inspect --json
```

`inspect --json`의 `integration.hosts`는 Codex·Claude·Omo마다 installed/linked/configured와
discovered/connected/protocol 여섯 관측을 verified/failed/unknown/not_checked로 보고한다.
기본 inspect는 파일과 설정만 읽으므로 뒤 세 관측은 `not_checked/host_receipt_required`다.
`inspect --host-receipts FILE`(MCP `harness_inspect`의 `host_receipts`)은 실제 host 실행 artifact를
가리키는 schema 1 receipt를 읽는다. receipt가 verified를 유지하려면 transport·config_path·
config_sha256·host version이 현재 값과 같고, artifact 내용에 `docs_index` 또는 주장한 revision이
있어야 한다. 하나라도 어긋나면 `receipt_stale_*`·`receipt_artifact_missing_*` 같은 reason과 함께
unknown이 된다. `config_sha256`은 config 파일 전체가 아니라 secret을 지운 issueops entry만
해싱한다. `~/.claude.json`처럼 host가 매번 바꾸는 다른 필드가 receipt를 stale로 만들지 않게
하려는 것이다. 공용 HTTP 서비스의 `harness_inspect`는 supervisor의 `HOME`/`PATH`로 실행되고
`CODEX_HOME`이 없으므로, 사용자 셸에서 실행한 CLI `inspect`와 Codex 설정 경로나 host version
관측(`host_version_unobservable`)이 다를 수 있다.

`quality inspect`는 수집 성공(`collection_status`), repository health
(`health_status`), automation gate(`gate_status`)를 분리한다. Collector 오류는
`ok=false`, `health_status=unknown`, `gate_status=block`이고 repository debt는
`health_status=needs_attention`, `gate_status=report_only`다. 각 finding은 stable
ID, severity, evidence, remediation, verification command를 가지며
`pioneer_coverage`는 canonical 12종 missing name을 그대로 노출한다.
`gate_status=block`은 JSON/text 결과를 먼저 출력한 뒤 process exit를 nonzero로
끝내므로 CI는 payload와 exit code를 함께 사용할 수 있다. `report_only`는
repository debt를 보이되 exit 0을 유지한다.

분기 복잡도와 code-SNR은 같은 제품 Go 파일 목록을 사용한다. Git workspace에서는
tracked 파일과 ignore되지 않은 untracked 파일을 포함한다. tracked 파일은 ignore
패턴과 겹쳐도 포함하고, 삭제된 파일은 제외한다. `_test.go`, `testdata`, `vendor`,
`node_modules`, `.git`, `.codegraph`, `.issueops-runtime`, `bin`, `.issueops` 경로와
표준 `// Code generated ... DO NOT EDIT.` 주석이 있는 파일은 제외하며 symlink를
따라가지 않는다. 일반 generated 문구만 있는 주석은 제외 근거가 아니다.
Git metadata가 없는 일반 디렉터리는 같은 제외 규칙으로 filesystem을 탐색한다.
Git workspace의 관측 실패·deadline·출력 제한·파일 읽기 오류는 collector warning/error로
전달하고, ignored 파일을 포함하는 fallback이나 성공한 빈 결과로 바꾸지 않는다.
하위 디렉터리를 분석 루트로 넘겨도 제외 경로를 유지한다. Git에서는 가장 가까운
저장소 기준 경로를 사용하므로 제외 경로 안에 별도 Git 저장소가 있으면 그 저장소의
제품 파일은 분석한다. non-Git에서는 분석 루트의 제외 구성요소도 검사한다.
이 선정 기준은 coverage 실행 대상이나 cache fingerprint를 변경하지 않는다.

## Command Policy

```bash
issueops policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
issueops policy run --read-only --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
issueops policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
```

`policy run --read-only` executes only argv-form allowlisted read-only commands with workspace/cwd policy, timeout, env allowlist, audit metadata, redaction, and bounded stdout/stderr. It does not allow write, network, arbitrary shell, or background execution. Use `policy fake-run` for write-intent planning.

`guard check` is a portable quality gate. It blocks clear anti-patterns, warns on likely quality smells, and marks context-dependent cases for review. Current rules include secret-like paths, test sleeps, real external URLs in tests, ambiguous test names, snapshot/golden review needs, production-only changes, CLI/MCP/adapter contract changes without golden updates, and likely duplicate helpers.

## State

```bash
issueops state write --key checkpoint-1 --value "작업 메모" --json
issueops state read --key checkpoint-1 --json
issueops state list --json
issueops state prune --max-age 720h --json
issueops state prune --max-age 720h --confirm --json
issueops state doctor --json
issueops state maintain --json
```

State commands use user-state storage, not target repo source files.

## Loop Contracts

```bash
issueops loop start --repo PATH --name NAME --goal TEXT [--max-attempts N] [--json] -- [VERIFY_ARGV...]
issueops loop record-attempt --id ID --verdict pass|fail --evidence TEXT [--evidence TEXT...] [--json]
issueops loop status (--id ID | --repo PATH --name NAME) [--json]
issueops loop stop --id ID (--success | --reason TEXT) [--json]
```

`loop` records a durable verify-until-done contract. `start` stores `verify_argv` but never executes it; `record-attempt` requires evidence; `stop --success` requires the latest attempt to be `pass`. Same-repo active or exhausted loops block strict PR readiness with `loop_incomplete:<loop-id>`.

## Task Gate Ledgers

```bash
issueops gates init [--file PATH] --scope TEXT --gate "G1: outcome | CHECK: cmd | EXPECT: expect" [--gate SPEC...] [--json]
issueops gates check [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--timeout-seconds N] [--env NAME,NAME] [--write] [--network] [--json]
issueops gates status [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--json]
issueops gates report [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--json]
issueops gates abandon --gate ID --reason TEXT [--file PATH] [--json]
```

`gates` discovers per-issue ledgers (`.issueops/issues/<n>/gates.md`)
first, then generic task ledgers (`.issueops/gates/*.md`) and compatible
unlazy ledgers (`GATES.md` plus `gates/*.md`). The format is the
unlazy v2 contract: one checkbox per outcome,
`CHECK:` command plus `EXPECT:` substring-or-`/regex/` match, and `EVIDENCE:`
recorded from the deciding output tail. A checkbox is a claim; evidence is the
proof — a checked gate whose evidence still reads `pending` counts as unmet
(`evidence_pending`, worse than `unchecked`). `ABANDON: <id> <reason>` is the
honest exit and resolves the gate for readiness while keeping it visible in
reports. Unlike upstream unlazy, CHECK commands never run through a raw shell:
they are tokenized to argv and executed through the command policy engine
(workspace boundary, env allowlist — default `HOME,PATH`, secret redaction,
timeout, audit log, shell interpreters denied). Exit codes follow unlazy: `0`
all met or abandoned, `1` unmet remain, `2` usage error.

`gates init` no longer writes a shared root file by default. With no `--file`,
it derives generic `.issueops/gates/<scope-slug>.md` from the required
`--scope`. IssueOps uses the stable convention
`.issueops/issues/<provider-issue-number>/gates.md` and passes that path to
`gates abandon`. Distinct issue folders let concurrent worktrees merge without
sharing root `GATES.md` and keep tracked plan/spec/gate artifacts together.
Existing `.issueops/gates/*.md`, `GATES.md`, and `gates/*.md` files remain
read-compatible, but new IssueOps cycles use the per-issue path.

MCP exposes the same operations as `gates_init`, `gates_check`, `gates_status`,
`gates_report`, and `gates_abandon` sharing one contract DTO (schema version 1).

## Cross-Session Channels

```bash
issueops channel send --channel NAME --from SESSION --message TEXT [--json]
issueops channel recv --channel NAME [--since MSG_ID] [--wait] [--timeout-seconds N] [--limit N] [--json]
```

`channel` is a durable shared mailbox over issueops state: Codex, Claude Code,
and Omo sessions sharing the same state exchange messages through it. In the
front/server coordination pattern, the server session `send`s the API
contract, the front session `recv --wait` blocks for it, and both sides keep
a `--since <msg-id>` cursor from the last seen message to continue the dialog.
Messages are append-only with nanosecond-ordered IDs, so key order is arrival
order. `recv --wait` returns exit 0 with messages or exit 1 on timeout
(`timed_out: true` in JSON). Channels are a trust boundary only between
sessions sharing the same issueops state — no cross-machine semantics.

IssueOps integration is opt-in through file presence. A linked cycle judges
its own `.issueops/issues/<n>/gates.md`, anonymous ledgers, and compatible
legacy paths; other numbered issue ledgers are skipped with one warning. A
canonical and legacy ledger for the same issue fails closed as
`duplicate_issue_artifact:<n>`. Unmet gates add `gates_incomplete:<file>` and
block entering `pr` until every gate has evidence or an honest `ABANDON`.
Because the ledger lives in the worktree, real cycles commit it before strict
readiness checks `worktree_clean`.

## Daemon And MCP

```bash
issueops daemon start --json
issueops daemon status --json
issueops daemon stop --json
issueops mcp
issueops mcp cleanup --json
issueops mcp cleanup --apply --json
```

기본 HTTP 설치에서 세 host는 `issueops mcp --http`(서비스 제어는 `issueops mcp service start|stop|status --json`)에
연결하고, HTTP로 workspace 도구를 부르기 전에 native 세션에서 `issueops mcp authorize`를 실행해
`authority_file`을 받는다. stdio `issueops mcp`는 host 세션 안에서 in-process로 동작하며 daemon을
시작하지 않는다. 두 경로 모두 daemon을 쓰지 않는다. 아래 daemon 명령과
admission 설정은 이전 binary로 떠 있는 MCP proxy가 붙는 legacy daemon에만 적용된다.

daemon admission은 기본 256개 동시 MCP 연결을 허용한다. 장기 실행 multi-session
host에서 더 큰 bounded pool이 필요하면 daemon 시작 전에
`ISSUEOPS_DAEMON_MAX_CONNECTIONS`를 `1..4096` 범위로 설정하고 daemon을
재시작한다. 범위를 벗어나거나 해석할 수 없는 값은 기본 256으로 fail-safe
복귀한다. `daemon status --json`의 `active_connections`,
`max_connections`, `accepting`으로 실제 admission 상태를 확인한다.

`mcp cleanup`은 기본 dry-run이다. Darwin의 `--apply`만 현재 checkout의 exact `issueops mcp` 명령, `PPID=1`, 확인된 executable/start time을 모두 만족하고 signal 직전 동일 identity가 다시 확인된 고아를 종료한다. Linux 컨테이너처럼 `PPID=1`이 살아 있는 host일 수 있는 플랫폼은 `skip-unsupported-platform`으로 거부한다. 살아 있는 host proxy, 다른 checkout, 외부 MCP, identity 미확정 프로세스는 건드리지 않는다.

MCP smoke:

```bash
tmp_state="$(mktemp -d)"
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  '{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"issueops://commit-policy"}}' \
  | ISSUEOPS_STATE_DIR="$tmp_state" issueops mcp
rm -rf "$tmp_state"
```

## Worker

```bash
issueops worker enqueue --kind smoke --payload "TOKEN=redacted" --json
issueops worker status --id "$JOB_ID" --json
issueops worker list --json
issueops worker cleanup-stuck --json
issueops worker cancel --id "$JOB_ID" --json
issueops worker run --read-only --kind smoke --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
```

`worker` currently records lifecycle jobs and can run policy-gated read-only evidence commands. It is not a general writable shell runner. Future process execution must pass command policy, audit logging, timeout/cancellation, and redaction checks.

## Contract And Audit

```bash
issueops contract schema --json
issueops contract check --json
ISSUEOPS_AUDIT_LOG="$(mktemp)" issueops policy audit --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
```

`policy audit` appends redacted JSONL policy decisions and does not execute the command.
