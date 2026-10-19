# IssueOps v1 execution verification

[← TESTING.md](../TESTING.md) owns the test-strategy index. This module owns
the IssueOps execution vertical contract tests. CLI/MCP/Codex/Claude adapter
parity that is not
IssueOps-execution-specific lives in
[cli-mcp-and-hosts.md](cli-mcp-and-hosts.md); race package sets are cross-cut
with [concurrency-and-race.md](concurrency-and-race.md).

`execution release` production vertical은 `internal/adapter/issueops` differential
test로 schema v1·missing/zero legacy schema·rich sidecar·holder index와 denial
atomicity를 비교한다. 변경 시 core focused race, outbound focused race,
architecture ratchet, CLI/MCP contract, golden, scoped vet와 build를 실행하며,
전체 suite는 PR CI가 회귀 증거로 담당한다.

`issueopspublication` vertical은 fixed operation ID와 fixed clock을 쓰는 frozen
legacy oracle/new vertical differential로 create·reconcile의 result JSON, error
text, record row, `external_intent_v1` row를 byte-for-byte 비교한다. Provider
create·inventory·live verification 중에는 cycle lock이 해제되어 동시 read와
replacement preview가 완료되어야 한다. CLI text, MCP `isError`, production
provider resolver caller-zero, non-test legacy full-flow 부재를 각각 adapter 및
AST ratchet 테스트로 고정한다.

Normal tests and self-verification must remain green without any external
runtime. Use injected workspace, provider, and process adapters for the default suite. The
current execution contract is `issueops_v1` with `schema_version=1`; legacy
rows and files are reset through the explicit fingerprint-CAS maintenance path,
never migrated or dual-read by normal execution.

Execution tests must cover:

- the session handoff chain as the only takeover path: the holder's `release`,
  the new session's `execution replace --id ID --expected-generation N --preview`
  under its own native actor, the emitted `--reseed` command, and `execution claim
  --claim-current-token`. `issueops next` must route a claimable execution to
  `claim --claim-current-token`, a released execution to `replace --preview`,
  and a revoking execution to `replace --finalize-preview`.
- explicit cmux handoff as a separate released path. Deterministic tests
  must cover exact binary/version/socket/window fences, endpoint-incarnation
  stability, stage-before-create ordering, target enrichment, one-shot send,
  receiver PID/start/executable correlation, and the absence of native-turn or
  claim promotion from raw input. Prompt reads must use one bounded no-follow
  handle-relative read and reject namespace replacement, symlinks, content over
  the shared 64 KiB single-argv bound, unsafe mode, a leaf owner other than the
  current effective UID, owner drift, and NUL bytes before external calls. The
  handler must pin the first canonical-root identity, reread the durable record,
  and recheck the request/process cwd, worktree, Git top-level, generation, and
  root identity immediately before Preflight, CreateWorkspace, and Send. Tests
  must replace state or the root namespace after the preceding call and prove
  that the next call is not made. They must also cover Codex,
  Claude, native Omo, omp, and opencode argv; multiline/quoted shell input; missing/denied
  sockets; malformed and duplicate responses; wrong cwd/target; runtime
  endpoint replacement; create and send response loss; same-generation
  duplicate attempts across changed targets or payloads; actual process cwd
  mismatch; and recovery artifact/orphan
  boundaries without retry, fallback, or automatic workspace cleanup.
- `issueops next` stage classification as a table test over the rule order, and an
  assertion that the read path performs no fetch: the local readiness surface must
  run without `git fetch`, and the strict surface must still run it.
- `cleanup abandon` remote effects (`--close-pr`, `--close-issue`,
  `--delete-remote-branch`): the applied order (`close_pr` → `close_issue` →
  `remote_branch_delete` → local removal), idempotence when an artifact is already
  closed or the remote branch is already absent, and partial failure leaving the
  record and worktree intact with a resumable failure receipt. A run without the
  flags must produce byte-identical output and the same fingerprint as before the
  flags existed.
- the implementation review gate before every publication.
- claimable → active → revoking/released generation transitions, the
  `lease_holder_v1` reverse index, exact native process identity, and stale
  holder/generation rejection across CLI and MCP. Host hooks no longer take
  part in this boundary (2026-08-27); every denial is emitted by the `issueops`
  command itself as `code`, `lifecycle_id`, `expected_root`,
  `current_generation`, and `next_command`.
- replacement preview, revoke, and finalize with PID-reuse-safe process
  observation. A live holder process blocks finalization.
- plan readiness tests must prove a non-empty staged `plan` before a fresh
  prepare, atomic `plan_path` persistence with the worktree receipt, exact
  staged/durable digest equality on reseed, and zero
  operation/worktree/lease mutations on failure.
- completion only from `pr` with the durable verified PR/MR projection; the
  completion receipt, lease release, reverse-index deletion, and `done` phase
  transition are one atomic write. An identical retry is idempotent only when
  all terminal invariants still hold.
- completed replacement preview and reseed must test parent drift and no-drift
  against the same outbound observer. Drift must preserve the raw record,
  completion/history/ledger/lease, token paths, artifact prepare count, and
  repository commit count.
- current completion generation 0/missing is invalid in preview and reseed even
  when the request supplies a generation. No selected-generation compatibility
  fallback or legacy wording is permitted.
- released-completion sync-base tests must cover matching/missing/wrong/history
  generation, claimable/history-only state, canonical cwd, live/mismatched
  process receipt, pending intent, stale fingerprint, immutable completion, and
  exact apply/finalize/abort/retry commands. Hook matrices must run the exact
  forms for Codex and Claude and block duplicates, wrappers, shell expansion,
  wrong cwd/lifecycle, stale history generation, multiple modes, and unknown
  flags.
- architecture tests must scan every production Go file in
  `internal/port/issueopsbasesync` and allow only Request, Receipt, Inspector,
  and the `context` import.
- typed-error
  `next_command` and conflict `abort_command` cannot escape generated-command
  provenance binding. GREEN requires canonical executable, hash, and generation
  provenance on both fields, or a tested conversion to non-executable guidance.
- released sync-base production reachability must start from a claimable fixture
  and use the public execution dispatcher with the production claim and complete
  handlers before preview/apply/finalize. Direct completion/active record writes
  are allowed only for isolated gate tests and cannot serve as vertical evidence.
- generated `next_command` tests must cover every production-reachable adapter
  path. CLI covers prepare/status/replace/sync-base preview;
  MCP covers its advertised prepare/status/replace/reconcile/complete
  surface plus typed base-sync-required errors from replace. Because MCP
  has no sync-base action, a success-result sync-base binder/test is dead code.
  Both adapters also cover
  execution reseed preview, cleanup finish preview/apply, exact current-binary path/hash binding,
  observation failure with no command fallback, and stale installed-binary versus
  newer worktree-binary rejection before a mutation handler is entered.
- A current-generation absolute generated IssueOps mutation may proceed only when its
  exact `--cwd` selects the canonical worktree and the CLI independently proves that
  `os.Getwd()` matches it before mutation. Bare commands, mismatched process cwd,
  stale provenance, and delegation commands whose `--parent` identity is missing
  must remain fail-closed.

Use this focused package set before the full repository gates:

```bash
go test ./internal/adapter/issueops/... ./internal/adapter/codex ./internal/adapter/claude ./cmd/issueops/issueopscli ./cmd/issueops/hookcli ./cmd/issueops/hookcli/hookinput ./cmd/issueops/mcpcli -count=1
go test -race ./internal/adapter/issueops/... ./cmd/issueops/issueopscli ./cmd/issueops/hookcli ./cmd/issueops/hookcli/hookinput ./cmd/issueops/mcpcli -count=1
```

For cmux changes, add the deterministic boundary and H0 projection checks. Do
not launch the app or perform a live workspace mutation as part of the default
suite:

```bash
go test ./internal/adapter/cmux ./internal/domain/nativehost ./internal/domain/issueops ./cmd/issueops/issueopsapp ./cmd/issueops/issueopscli/executioncmd -count=1
go test -race ./internal/adapter/cmux ./internal/domain/nativehost ./internal/domain/issueops ./cmd/issueops/issueopsapp ./cmd/issueops/issueopscli/executioncmd -count=1
go test ./internal/contract/issueops -run 'H0|H5' -count=1
```

The deterministic H0 fixture contains the three cmux × native-host same-host
rows and six directed cross-host material-transfer rows. Installed/version
evidence with a disconnected socket remains `unavailable`; runtime and live
cells remain `not-run`, and no cmux × host combination is certified without a
separate completed live episode.

Native activation tests use isolated temporary homes. They require same-
directory staged build, smoke, atomic rename, strict Codex/Claude semantic and
raw-file readback, and a sealed activation receipt written last. Every injected
crash before that final receipt must leave destructive legacy reset blocked.

After native installation, exercise the Codex and Claude `SessionStart` fixtures
(including `source:"compact"`) through the common hook input boundary in
`./cmd/issueops/hookcli/hookinput` and require the same project-doc catalog
projection for both hosts; there are no allow/block hook projections any more.
