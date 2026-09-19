# Task 9 Report — H0 Capability And Host Compatibility Baseline

## Outcome

Implemented H0 as a deterministic typed contract plus versioned fixture under the existing IssueOps contract package. The fix round tightened two reviewer-identified contract gaps without adding a daemon, scheduler, network dependency, runtime cache, or broad CLI surface.

The baseline covers Orca, Herdr, cmux, and launcher-free direct handoff against Codex, Claude, and Omo. Each of the 12 cells has an explicit status (`supported`, `unsupported`, `unavailable`, or `not-run`) and separate installed, connected, capable, and runtime-verified evidence dimensions. Evidence now carries an explicit result (`positive`, `negative`, or `not-run`) so negative capability/connection evidence is not inferred from an absent or not-run observation.

## Files Changed

- `internal/contract/issueops/capability_baseline.go` — H0 schema, evidence dimensions, explicit observation results, status/evidence validation, six directed cross-host handoff validation, no-fallback rule, and recovery invariant validation.
- `internal/contract/issueops/capability_baseline_test.go` — contract tests for the 12 matrix cells, invalid status/kind/result rejection, silent fallback rejection, exact cross-host handoff coverage, installed/mock/live/not-run evidence separation, explicit negative evidence requirements, live supported attempt/runtime binding, and recovery invariants.
- `internal/contract/issueops/testdata/h0-baseline.json` — deterministic fixture for Tasks 10–13. It sets `kind: deterministic-fixture`, uses synthetic exact identities, and does not persist local runtime IDs, sockets, permissions, or process details.
- `.superpowers/sdd/review-verification-efficiency/task-9-report.md` — this report.

## Design Notes

- The canonical surface is `internal/contract/issueops` because H0 is an IssueOps contract baseline, not a runtime action. Existing execution recovery logic remains owned by the IssueOps lease/replacement/resume packages.
- `Observation.result` is the explicit-negative representation. `positive` and `negative` require an observed claim with version, executable path, runtime identity, timestamp, and attempt. `not-run` requires `claim: not-run` and `observed: false`.
- `supported` is only valid for `kind: live-observation` with positive live `runtime_verified` evidence bound to the baseline attempt ID and runtime identity. Deterministic fixture evidence cannot become supported E2E.
- `unavailable` requires explicit negative connected or capable evidence. `unsupported` requires explicit negative capable evidence. Non-supported cells cannot carry runtime-verified E2E evidence.
- Cross-host handoff must be exactly the six directed Codex/Claude/Omo material-transfer pairs. Missing pairs, duplicates, self handoffs, and native session migration are invalid.
- Silent fallback is invalid: selected, observed, and fallback launcher fields cannot encode “asked for X, used Y.”
- The recovery invariant model records direct released → replace preview → returned recovery chain → claimable → claim, and Orca released → replace → reseed → resume → sealed owner claim. It also requires failure coverage for released-immediate claim, stale generation, duplicate Orca owner, and manual Orca owner.

## Read-Only Environment Evidence

- `codex --version`: `codex-cli 0.155.1`; executable from `command -v codex` was `/Users/m16khb/Library/pnpm/bin/codex`.
- `claude --version`: `2.1.272 (Claude Code)`; executable from `command -v claude` was `/Users/m16khb/.local/bin/claude`.
- `omo --version`: `omo 5.0.0-0.beta.22 (engine: senpi 2026.8.26-2)`; executable from `command -v omo` was `/opt/homebrew/bin/omo`.
- `orca --version`: `1.4.200`; `orca status --json` returned `ok=true`, runtime `state=ready`, `reachable=true`, `connectionState=connected`, and capability `terminal.prompt-delivery.v1`.
- Runtime inventory note from the coordinator: Orca process inventory included multiple identities and an app-version mismatch. The fixture therefore requires exact runtime identity per attempt and does not collapse observations by product name or version.
- `herdr --version`: `herdr 0.9.0`; `herdr status` returned server `running`, protocol `22`, endpoint compatible `yes`, private protocol compatible `yes`.
- `herdr agent start --help`: supported kinds include `codex` and `claude`, but not `omo`; Omo remains a pane-run/native CLI path rather than a Herdr agent kind.
- `omo --help`: confirms initial prompt/session/resume/fork style controls and native Omo CLI identity. This was not substituted with OpenCode/OMP.
- cmux: PATH lookup did not find `cmux`; `/Applications/cmux.app/Contents/MacOS/cmux` exists, but a bounded help probe did not return and only printed notification authorization status. No app startup, socket permission change, prompt send, or session creation was performed.

## Project Docs SHA-CAS

No `.issueops` canonical document was changed. The existing plan section H0 remains the prose owner, and this task added a typed contract/fixture to prevent drift for downstream work. Because no `.issueops` document was revised, `project_docs_revise` SHA-CAS was not used.

## Verification

Initial implementation evidence:

- RED: `go test ./internal/contract/capabilitybaseline -count=1` failed before implementation with `no Go files` for the initial new package.
- GREEN: `go test ./internal/contract/issueops -run 'Test(H0BaselineFixture|Validate)' -count=1` passed after moving the contract into the existing IssueOps contract package and adding status/kind validation.
- `go test ./... -count=1`, `go test -race ./... -count=1`, `go build -o bin/issueops ./cmd/issueops`, `./bin/issueops docs --json`, and `./bin/issueops inspect --json` passed before the fix round.

Fix round evidence:

- RED: `go test ./internal/contract/issueops -run 'TestValidateRequiresExactlySixDirectedCrossHostHandoffs|TestValidateEvidenceResultsAreExplicit' -count=1` initially failed to compile because `ObservationResult` and baseline runtime binding did not exist yet.
- `gofmt -l internal/contract/issueops/capability_baseline.go internal/contract/issueops/capability_baseline_test.go` passed with no output.
- `git diff --check` passed.
- `go test ./internal/contract/issueops -count=1` passed after adding explicit observation results, exact six directed handoff validation, and live supported attempt/runtime binding.
- `go test ./internal/contract/... -count=1` passed.
- `go test ./internal/architecture -run TestProductionPackagesHaveImporters -count=1` passed.
- `go vet ./...` passed.
- `python3 -m json.tool internal/contract/issueops/testdata/h0-baseline.json` passed.

## Known Limits

- No live launcher-to-host E2E was run. The fixture records those cells as `not-run` or `unavailable` and does not claim runtime support.
- No speed improvement is claimed. H0 only establishes fixed input/contract evidence that later P0-style measurements can compare against.

## Fix Round 2 — Recovery FailureCases exact-set validation

Reviewer found that recovery `FailureCases` used membership-only validation, which accepted extra values, duplicate values, and unknown string values. The fix replaces membership validation with exact-set validation for both direct and Orca recovery modes while preserving step ordering validation unchanged.

Verification for fix round 2:

- RED: `go test ./internal/contract/issueops -run TestValidateRecoveryFailureCasesAreExactSets -count=1` failed before the fix because direct/orca extra and duplicate failure cases were accepted, and unknown values only failed through the older membership message.
- GREEN: `go test ./internal/contract/issueops -run 'TestValidateRecoveryFailureCasesAreExactSets|TestValidateRecoveryModeInvariants' -count=1` passed after adding exact-set validation.
