# Final-review fixes F4 and F5: evidence report

Base HEAD `92eaa001`, uncommitted working tree. Review date 2026-10-02/03. Scope: `internal/adapter/inspect`, `cmd/issueops/mcpcli/mcp_sdk_server.go` (instruction text only). No commit, no push, no real `$HOME`, no `bin/issueops`, `ddd_responsibility_inventory.json` untouched.

## Files

| File | Change |
|---|---|
| `internal/adapter/inspect/receipts.go` | `artifactEvidence` no longer does substring search. It parses the artifact as JSONL and reads supported host events (`readArtifactFacts`, `artifactScan`). |
| `internal/adapter/inspect/receipts_artifact_test.go` (new) | Table test over 11 real-shaped fixtures. |
| `internal/adapter/inspect/testdata/receipts/*.jsonl` (new, 11 files) | Minimal lines trimmed from the real artifacts, plus error variants. |
| `internal/adapter/inspect/hosts_test.go` | Only the `newReceiptFixture` artifact content changed (see "Existing-test change"). No assertion edited, no test deleted. |
| `cmd/issueops/mcpcli/mcp_sdk_server.go` | `httpServerInstructions` constant; `newSDKServer` sets it when `transport == transportHTTP`. Stdio text is unchanged. |
| `cmd/issueops/mcpcli/mcp_sdk_instructions_test.go` (new) | Initialize instructions per transport. |

No instruction-text golden exists (grep over `cmd/issueops` found none), so no golden was regenerated.

## F4: what the parser accepts

Format is detected per line; supported shapes were learned from `/tmp/kinsp-1790951136/artifacts/*.jsonl`, `/tmp/issueops-ten-improvements-01a0fa31/receipt-art/*.jsonl`, and `claude-stream-real.jsonl`.

| Shape | Discovered (`tools/list`) | Connected (`docs_index`) | Protocol |
|---|---|---|---|
| Claude stream-json | `system/init` `tools` contains `docs_index` or `*__docs_index` | `tool_use` of docs_index, then `tool_result` with the same `tool_use_id` and `is_error` not true | no revision field exists, so unknown |
| Codex app-server (`direction` + `raw` JSON-RPC) | a response `result.data[].tools` map has `docs_index` | the response to the `mcpServer/tool/call` request (request order = JSON-RPC id) has no `isError`/`error`, its text payload has no `ok:false`, and it has a `docs` field (request lines carry no params) | `result.protocolVersion` in a response; the real initialize response has none, so unknown |
| Codex step summary (`step`) | `inventory` server `tools` lists docs_index | `tool` step with `tool:"docs_index"`, `ok:true`, `isError` not true | none, so unknown |
| Omo transport run (`ok`, `docs`) | `ok:true` and `tools` lists docs_index or `has_docs_index` | `ok:true`, no `is_error`/`isError`, numeric `docs` | `revisions` or `mcp_protocol_version_headers` arrays on an `ok:true` line |

Semantics: any failed docs_index result in the artifact turns Connected into `unknown/receipt_artifact_tool_failed` even if another call succeeded (conservative). No recognized evidence gives `receipt_artifact_missing_evidence` (Discovered/Connected) or `receipt_artifact_missing_revision` (Protocol). A revision that is only mentioned in prose or in a non-response field (including the reviewer artifact's top-level `protocolVersion`) is not accepted. The existing config_path, config sha, host version, transport, synthetic and source checks are unchanged. Artifact format is not bound to the receipt's host; the host is bound only through the stale checks.

## RED

Command (before the `receipts.go` change; the new fixture and test file already in place):

```
go test ./internal/adapter/inspect -run TestHostsReceipt -count=1     # exit 1
```

Failures observed (one assertion per subtest because `requireStatus` is fatal):

- `claude-stream-error.jsonl`, `codex-app-server-error.jsonl`, `codex-steps-error.jsonl`: `connected = verified/`, want `unknown/receipt_artifact_tool_failed`.
- `codex-app-server-no-catalog.jsonl`: `connected = unknown/receipt_artifact_missing_evidence`, want `verified` (docs_index succeeded; the substring check needed the literal name in the file).
- `omo-transport-error.jsonl`: `discovered = verified/`, want `unknown/receipt_artifact_missing_evidence`.
- `omo-revision-mentioned-only.jsonl`: `protocol = verified/`, want `unknown/receipt_artifact_missing_revision`.
- `failed-call.jsonl` (the reviewer's `{"type":"tool_result","name":"docs_index","isError":true,"error":"connection failed","protocolVersion":"2026-07-28"}`): `discovered = verified/`, want `unknown/receipt_artifact_missing_evidence`.

F5, before the change: `go test ./cmd/issueops/mcpcli -run TestInitializeInstructionsFollowTransport -count=1` exit 1, `HTTP instructions missing "shared local service": "This MCP endpoint runs the issueops harness in-process for the calling host session. ..."`.

## GREEN

```
gofmt -l internal/adapter/inspect cmd/issueops/mcpcli                 # no output
go vet ./internal/adapter/inspect ./cmd/issueops/mcpcli ./cmd/issueops/basiccli   # exit 0
go test -race ./internal/adapter/inspect ./cmd/issueops/mcpcli ./cmd/issueops/basiccli -count=1   # exit 0
go test ./cmd/issueops/... -run 'Golden|Contract' -count=1            # exit 0
```

`go test -race` output: `ok issueops/internal/adapter/inspect`, `ok issueops/cmd/issueops/mcpcli`, `ok issueops/cmd/issueops/basiccli`.

Instruction text now:

- stdio (unchanged): `This MCP endpoint runs the issueops harness in-process for the calling host session. ...`
- HTTP: `This MCP endpoint is the shared local service used by every host session, not a per-host process. Workspace tools need an authority_file issued by 'issueops mcp authorize'. ...`

Note: the first GREEN attempt failed `TestInitializeInstructionsFollowTransport` because my first HTTP wording said "shared local issueops service"; I corrected the production text, not the test.

## Existing-test change

`newReceiptFixture` in `hosts_test.go` wrote a toy artifact (`system` line with a top-level `protocolVersion`, a `tool_use` with no result). That cannot prove a successful call or a response-field revision under the new semantics, so it now writes one real-shaped Omo transport line (`ok:true`, `docs:3`, `tools` listing docs_index, `mcp_protocol_version_headers:["2025-11-25"]`, `is_error:false`). All assertions in the existing tests are unchanged and pass, including the unrelated-file case that still expects `receipt_artifact_missing_evidence` / `receipt_artifact_missing_revision`.

## Real inspect results

Built to `/tmp/f45/bin/issueops` (`go build -o`), isolated HOME `/tmp/f45/home` with Codex/Claude/Omo HTTP configs, host-version stubs on PATH (`codex-cli 0.128.0`, `2.1.287`, `1.0.0`), receipts generated from the observed config sha. Command: `issueops inspect --json --host-receipts <receipts>`.

Without receipts: every discovered/connected/protocol is `not_checked/host_receipt_required`.

**Reviewer's failed artifact (all three hosts point to it): nothing verifies.**

```
codex  discovered unknown/receipt_artifact_missing_evidence  connected unknown/receipt_artifact_missing_evidence  protocol unknown/receipt_artifact_missing_revision
claude (same)
omo    (same)
```

**Real artifacts (`kinsp .../codex-app-server-run.jsonl`, `claude-stream-run.jsonl`, `omo-transport-run.jsonl`):**

```
codex  discovered verified  connected verified  protocol unknown/receipt_artifact_missing_revision
claude discovered verified  connected verified  protocol unknown/receipt_artifact_missing_revision
omo    discovered verified  connected verified  protocol verified
```

**Second real set (`receipt-art/codex-run.jsonl`, `claude-stream-real.jsonl`, `receipt-art/omo-run.jsonl`): same result.** Protocol is verified only for Omo because only its artifact carries the negotiated revision in a response field. The Codex and Claude artifacts carry none, matching the existing `revision_not_observed_by_host_artifact` receipts.

One mistake during checking: my first real run used a shell loop with a bad `\${...}` escape, so only the real `claude` binary matched its version and the Codex/Omo rows showed `host_version_unobservable`/`receipt_stale_host_version`. I recreated the stubs explicitly and reran; the results above are from the rerun.

## Not done / residual

- `go vet ./...` currently fails in `internal/adapter/outbound/sqlstore` and `internal/adapter/outbound/issueopslease` tests (`WithRecordGuard` arity). That is the parallel F1/F3 node's in-progress signature change; I did not touch those files. Vet on the three packages I touched is clean.
- The Codex app-server parser pairs a call with its response by request order (the recorded request lines have no id or params) and identifies docs_index by the `docs` field in the payload. If a recorder adds notifications between requests, the pairing fails safe (Connected stays unknown).
- Artifact format is host-agnostic: a receipt for one host can cite another host's artifact format. The config/version/transport checks still bind the receipt to the current host. This is an evidence-quality check, not an adversarial boundary.
