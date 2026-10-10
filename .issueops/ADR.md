---
name: ADR.md
description: Accepted structural decisions and their rationale; read before reversing or extending a design choice.
---
# Architecture Decision Records

작성일: 2026-05-25.

> 이 파일의 날짜별 항목은 append-only 결정 이력이다. 과거 항목의 retired host/schema/command 명칭은 당시 근거를 보존하는 역사 표기이며 현재 지원 계약이 아니다. 현재 운영 표면은 루트 `AGENTS.md`, `ARCHITECTURE.md`, `OPERATIONS.md`와 가장 최근의 명시적 superseding 결정이 정한다.

이 색인은 accepted architecture decision의 정규 입구다. 모든 record는
`adr/YYYY-MM-DD-<slug>.md` 한 곳에 있다. `project_docs_append(kind=adr)`도 이 경로에 쓴다.
2026-10-07에 예전 `adr/decisions/` record를 이 폴더로 올렸다.

## Accepted baseline

- **아키텍처:** 외부 하네스 코어(CLI/MCP/worker)에 얇은 host adapter를 얹는
  hybrid 구조. 근거는 [plugin vs external worker](adr/2026-05-25-plugin-vs-external-worker.md)
  와 [Go 언어 선택](adr/2026-05-25-go-language-selection.md) 결정에 있다.
- **First-party hosts:** Codex, Claude Code, Omo native, omp가 같은 shared skill,
  MCP, lifecycle activation contract를 사용한다. 근거는
  [Omo native first-party host](adr/2026-08-12-omo-native-first-party-host.md)와
  [omp first-party host](adr/2026-10-08-omp-first-party-host.md) 결정에 있다.
- **External integrations:** native activation, readiness, self-verification은
  standalone으로 유지한다. Native activation 뒤 Claude-scoped declarative catalog를
  non-fatal로 provision할 수 있는 좁은 예외는
  [optional upstream provisioning](adr/2026-08-28-optional-upstream-provisioning-preserves-the-standalone-core.md)이 소유한다.
- **구현 로드맵:** phase별 계획, 목표 아키텍처, MVP 범위, 위험, 다음 작업 후보는
  [adr/roadmap.md](adr/roadmap.md) 가 소유한다.
- **결정 규칙:** status model, naming, authoring 규칙과 archived history
  ledger는 [adr/README.md](adr/README.md) 가 소유한다.

## Decision index

Reverse-chronological. Same-day records use a distinguishing slug. Status model
and supersession rules live in [adr/README.md](adr/README.md).

| Date | Decision | Record |
|---|---|---|
| 2026-10-10 | Completed cycles merge through typed `remote merge-pr` (squash default, head pinned to completion, no branch deletion/admin/auto-merge); blockers reopen the cycle via reseed | [record](adr/2026-10-10-issueops-merges-completed-cycles-through-a-typed-remote-merg.md) |
| 2026-10-08 | State cleanup runs on the write path that owns it: `channel send` prunes 7-day-old messages, install/update removes retired state paths after commit, and the MCP HTTP log is capped in-process | [record](adr/2026-10-08-state-cleanup-runs-on-the-write-path-that-owns-it.md) |
| 2026-10-08 | omp (oh-my-pi) is the fourth first-party host; its lifecycle extension exports the main session id as `ISSUEOPS_OMP_SESSION_ID` | [record](adr/2026-10-08-omp-first-party-host.md) |
| 2026-10-08 | SubagentStart carries the project-doc catalog, and every host gets the same model text; supersedes the "SessionStart only" clause of 2026-08-27 | [record](adr/2026-10-08-subagent-start-catalog-and-unified-model-text.md) |
| 2026-10-08 | Role models resolve from flag, main-worktree local, and XDG global settings and are injected into Orca·cmux owner sessions; supersedes the Claude implement default of 2026-09-24 | [record](adr/2026-10-08-role-models-resolve-from-global-and-local-settings-and-are-i.md) |
| 2026-10-07 | Review revise and regress rounds are capped at five; supersedes the cap number of 2026-07-02 and 2026-09-08 Decision (4) | [record](adr/2026-10-07-review-revise-and-regress-rounds-are-capped-at-five.md) |
| 2026-10-07 | `.issueops` 폴더 구조를 append와 runtime이 쓰는 경로에 맞추고 checker가 강제한다 | [record](adr/2026-10-07-issueops-docs-layout-matches-the-append-and-runtime-paths.md) |
| 2026-10-07 | Readers accept only the current record shape | [record](adr/2026-10-07-readers-accept-only-the-current-record-shape.md) |
| 2026-10-06 | Remove the last legacy compatibility paths and non-release platform code | [record](adr/2026-10-06-remove-the-last-legacy-compatibility-paths-and-non-release-p.md) |
| 2026-10-06 | Remove OpenWiki and the remaining legacy, test-only, and unsupported-platform code | [record](adr/2026-10-06-remove-openwiki-and-the-remaining-legacy-test-only-and-unsup.md) |
| 2026-10-06 | Delete re-export shims and stop naming live code legacy | [record](adr/2026-10-06-delete-re-export-shims-and-stop-naming-live-code-legacy.md) |
| 2026-10-03 | 요청 grant 재검사는 grant root의 span과 모든 data write 트랜잭션에서 한다(다른 root는 통과) | [record](adr/2026-10-03-grant-grant-root-data-write.md) |
| 2026-10-02 | 세 host가 공용 Streamable HTTP MCP 서비스에 직접 연결하고 native 발급 caller capability로 요청 권한을 정한다; 2026-09-23 in-process 결정을 stdio 경로로 좁힌다 | [record](adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md) |
| 2026-10-02 | `rebase-onto-parent` 스킬을 `sync-base`로 바꾸고 merge와 rebase를 증거로 고른다 | [record](adr/2026-10-02-sync-base-skill-chooses-merge-or-rebase-by-evidence.md) |
| 2026-09-30 | 독자 검토를 공개 본문 10건 분석 뒤에도 권장 절차로 유지한다 | [record](adr/2026-09-30-reader-review-remains-recommended-after-ten-public-bodies.md) |
| 2026-09-30 | DDD responsibility ownership across the harness | [record](adr/2026-09-30-ddd-responsibility-ownership-across-the-harness.md) |
| 2026-09-29 | Cleanup ownership binds the operation and exact record revision; supersedes the draft finish-only field | [record](adr/2026-09-29-cleanup-ownership-binds-the-operation-and-exact-record-revis.md) |
| 2026-09-29 | Cleanup finish executor owns observation and attempt-bound finalization | [record](adr/2026-09-29-cleanup-finish-executor-owns-observation-and-attempt-bound-f.md) |
| 2026-09-29 | Cleanup finish ownership is an optional record field with guarded writers | [record](adr/2026-09-29-cleanup-finish-ownership-is-an-optional-record-field-with-gu.md) |
| 2026-09-24 | Claude role models: Opus 5 plans and reviews, Sonnet 5 implements, Fable 5 is manual-only; supersedes the 2026-07-24 Claude defaults; superseded in part — defaults come from the role resolver and Claude implements with Opus since [2026-10-08](adr/2026-10-08-role-models-resolve-from-global-and-local-settings-and-are-i.md) | [record](adr/2026-09-24-claude-role-models-opus-5-plans-and-reviews-sonnet-5-impleme.md) |
| 2026-09-24 | Issue and PR bodies are human documents; implementation materials stay in .issueops/issues/<n>/; partially supersedes 2026-09-09 | [record](adr/2026-09-24-issue-and-pr-bodies-are-human-documents.md) |
| 2026-09-23 | issueops mcp serves in-process; the shared daemon leaves the MCP path (stdio 경로로 축소: 2026-10-02 공용 HTTP 결정; daemon 제거 완료: 2026-10-06) | [record](adr/2026-09-23-issueops-mcp-serves-in-process-the-shared-daemon-leaves-the.md) |
| 2026-09-23 | The lease contract decodes persisted records through the production record contract; supersedes 2026-07-28 | [record](adr/2026-09-23-the-lease-contract-decodes-persisted-records-through-the-pro.md) |
| 2026-09-09 | IssueOps seals the requester intent as a derived artifact next to the plan | [record](adr/2026-09-09-issueops-seals-the-requester-intent-as-a-derived-artifact.md) |
| 2026-09-08 | Pipeline skill routing: companion skills called by name, base drift as a three-surface model, frontend as a next flag | [record](adr/2026-09-08-pipeline-skill-routing-companion-skills-called-by-name-base.md) |
| 2026-09-08 | Adversarial review throughput: executable findings, change tiers, and concurrent read-only verification; superseded in part — the revise cap is five since [2026-10-07](adr/2026-10-07-review-revise-and-regress-rounds-are-capped-at-five.md) | [record](adr/2026-09-08-adversarial-review-throughput-executable-findings-change-tie.md) |
| 2026-09-08 | IssueOps project-doc gates: link-plan checks the four plan sections and no-change needs reviewed docs | [record](adr/2026-09-08-issueops-project-doc-gates-link-plan-checks-the-four-plan-se.md) |
| 2026-09-02 | IssueOps binds artifacts to the code project, not the issue project | [record](adr/2026-09-02-issueops-binds-artifacts-to-the-code-project-not-the-issue-p.md) |
| 2026-09-05 | IssueOps v0.1.0 이름 전환 | [record](adr/2026-09-05-issueops-v0-1-0.md) |
| 2026-09-05 | IssueOps ten-stage skills, CLI-owned stage detection, and the auto execution mode | [record](adr/2026-09-05-issueops-ten-stage-skills-with-auto-execution-mode.md) |
| 2026-09-02 | IssueOps publication evidence gates: project-doc reflection and conditional schema measurement | [record](adr/2026-09-02-issueops-publication-evidence-gates-project-doc-reflection-a.md) |
| 2026-08-28 | IssueOps devil's-advocate verdicts are bound to the reviewed plan digest | [record](adr/2026-08-28-issueops-devils-advocate-plan-binding.md) |
| 2026-08-28 | Optional upstream provisioning preserves the standalone core; partially supersedes the 2026-07-07 blanket prohibition | [record](adr/2026-08-28-optional-upstream-provisioning-preserves-the-standalone-core.md) |
| 2026-08-27 | SessionStart owns compaction context; legacy hook surface removed; superseded in part — default installs also register `SubagentStart` since [2026-10-08](adr/2026-10-08-subagent-start-catalog-and-unified-model-text.md) | [record](adr/2026-08-27-session-start-owns-compaction-context.md) |
| 2026-08-22 | Cross-session channel capability | [record](adr/2026-08-22-cross-session-channel.md) |
| 2026-08-22 | Task gate ledger (unlazy-compatible gates capability) | [record](adr/2026-08-22-task-gate-ledger.md) |
| 2026-08-21 | Bootstrap preserves in-progress repos and transparent plans | [record](adr/2026-08-21-bootstrap-respects-inprogress-repos.md) |
| 2026-08-21 | Engineering standards catalog for project-docs bootstrap and optimize | [record](adr/2026-08-21-engineering-standards-catalog.md) |
| 2026-08-10 | Default hooks are thin static context only | [record](adr/2026-08-10-default-hooks-thin-static-context.md) |
| 2026-08-08 | Dependency ratchet counts capability boundaries only | [record](adr/2026-08-08-dependency-ratchet-capability-boundary.md) |
| 2026-08-08 | Legacy baseline removed; ratchet becomes an invariant | [record](adr/2026-08-08-legacy-baseline-invariant.md) |
| 2026-08-08 | Port speaks in contract vocabulary | [record](adr/2026-08-08-port-contract-vocabulary.md) |
| 2026-08-08 | Contract cross-reference is composition, not implementation | [record](adr/2026-08-08-contract-cross-reference-composition.md) |
| 2026-08-04 | Post-completion base synchronization uses contract-owned authority | [record](adr/2026-08-04-post-completion-base-synchronization.md) |
| 2026-08-04 | Completed reseed requires stamped current completion provenance | [record](adr/2026-08-04-completed-reseed-stamped-provenance.md) |
| 2026-07-29 | Release vertical replaces the lease prototype | [record](adr/2026-07-29-release-vertical-replaces-lease-prototype.md) |
| 2026-07-28 | Lease differential contract owns stable v1 canonicalization; superseded on 2026-09-23 | [record](adr/2026-07-28-lease-differential-v1-canonicalization.md) |
| 2026-07-27 | Architecture dependency fitness ratchet | [record](adr/2026-07-27-architecture-dependency-fitness-ratchet.md) |
| 2026-07-26 | Linked branches are pinned to the sealed base SHA | [record](adr/2026-07-26-linked-branches-pinned-to-sealed-base-sha.md) |
| 2026-07-24 | IssueOps planner/implementer dual structure (#78) | [record](adr/2026-07-24-issueops-planner-implementer-dual-structure.md) |
| 2026-07-24 | Canonical command with managed `io` shorthand | [record](adr/2026-07-24-canonical-command-io-shorthand.md) |
| 2026-07-24 | Workpool removal | [record](adr/2026-07-24-workpool-removal.md) |
| 2026-07-21 | IssueOps uses one ownership handoff contract | [record](adr/2026-07-21-issueops-ownership-handoff-contract.md) |
| 2026-07-19 | One operational-health authority and external one-time reconciliation | [record](adr/2026-07-19-operational-health-authority-and-external-reconciliation.md) |
| 2026-07-15 | Supervised handoff coordinator isolation and bounded self-heal | [record](adr/2026-07-15-supervised-handoff-coordinator-isolation.md) |
| 2026-07-14 | Evidence-first cross-host tool contract hardening | [record](adr/2026-07-14-evidence-first-cross-host-tool-contract.md) |
| 2026-07-09 | Pipe-capture immunity and pipe-capacity doctor check | [record](adr/2026-07-09-pipe-capture-immunity-doctor-check.md) |
| 2026-07-09 | Loop contracts | [record](adr/2026-07-09-loop-contracts.md) |
| 2026-07-08 | SQLite store maintenance policy | [record](adr/2026-07-08-sqlite-store-maintenance-policy.md) |
| 2026-07-07 | State storage moves from JSON files + flock to SQLite (sqlstore) | [record](adr/2026-07-07-sqlite-state-storage-migration.md) |
| 2026-07-07 | Standalone issueops policy; broad upstream wiring removed; blanket prohibition partially superseded on 2026-08-28 | [record](adr/2026-07-07-standalone-harness-policy.md) |
| 2026-07-03 | Codex PreToolUse ask fallback; superseded — `pre-tool-use` hook was unregistered on 2026-08-10 and deleted by [2026-08-27](adr/2026-08-27-session-start-owns-compaction-context.md) | [record](adr/2026-07-03-codex-pretooluse-ask-fallback.md) |
| 2026-07-02 | External LLM calls emit per-call usage observation records; superseded — the externalllm client and usage recorder were deleted on 2026-07-07 under the [standalone policy](adr/2026-07-07-standalone-harness-policy.md) | [record](adr/2026-07-02-external-llm-usage-observation.md) |
| 2026-07-02 | IssueOps regress rounds are capped with a human-decision escalation; superseded in part — the cap is five since [2026-10-07](adr/2026-10-07-review-revise-and-regress-rounds-are-capped-at-five.md) | [record](adr/2026-07-02-issueops-regress-round-cap.md) |
| 2026-07-02 | Self-augment planner consumes Reflexion lessons as score penalty | [record](adr/2026-07-02-self-augment-reflexion-lessons.md) |
| 2026-07-02 | External LLM stays Z.AI-only until a second provider is real; superseded — the harness no longer calls an external LLM; host agents use prompt/result-file contracts ([standalone policy](adr/2026-07-07-standalone-harness-policy.md)) | [record](adr/2026-07-02-external-llm-zai-only.md) |
| 2026-07-01 | Defer harness-side tool-error context injection | [record](adr/2026-07-01-defer-tool-error-context-injection.md) |
| 2026-07-01 | IssueOps phase transition is a pure reducer over the record | [record](adr/2026-07-01-issueops-phase-transition-pure-reducer.md) |
| 2026-07-01 | IssueOps devil's-advocate is a fail-closed loop, not just skill prose | [record](adr/2026-07-01-issueops-devils-advocate-fail-closed-loop.md) |
| 2026-07-01 | MCP transport: adopt go-sdk with a retained legacy JSON-RPC path; superseded in part — the legacy JSON-RPC path was removed on 2026-08-03, go-sdk remains ([2026-10-02 transport](adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md)) | [record](adr/2026-07-01-mcp-transport-go-sdk-legacy-jsonrpc.md) |
| 2026-06-29 | IssueOps phase ledger, grill gate, and Design Review devil's-advocate regression | [record](adr/2026-06-29-issueops-phase-ledger-grill-gate-design-review.md) |
| 2026-06-26 | IssueOps compatibility review phase | [record](adr/2026-06-26-issueops-compatibility-review-phase.md) |
| 2026-06-24 | Skill local background separation | [record](adr/2026-06-24-skill-local-background-separation.md) |
| 2026-06-23 | IssueOps execution decision gate | [record](adr/2026-06-23-issueops-execution-decision-gate.md) |
| 2026-06-23 | IssueOps hook and state-machine boundary | [record](adr/2026-06-23-issueops-hook-state-machine-boundary.md) |
| 2026-06-18 | IssueOps plan-prep evidence gate | [record](adr/2026-06-18-issueops-plan-prep-evidence-gate.md) |
| 2026-06-18 | IssueOps implementation requires durable worktree tool preparation; superseded — `worktree prepare-tools` was removed with the v1 write lease (2026-07-23); `execution prepare` provisions worktrees ([2026-09-05](adr/2026-09-05-issueops-ten-stage-skills-with-auto-execution-mode.md)) | [record](adr/2026-06-18-issueops-worktree-tool-preparation.md) |
| 2026-06-16 | internal/core *_facade.go is the intended public surface; superseded — `internal/core` was removed; see [DDD ownership](adr/2026-09-30-ddd-responsibility-ownership-across-the-harness.md) and [re-export shim removal](adr/2026-10-06-delete-re-export-shims-and-stop-naming-live-code-legacy.md) | [record](adr/2026-06-16-core-facades-intended-public-surface.md) |
| 2026-06-13 | Distribution decision gate | [record](adr/2026-06-13-distribution-decision-gate.md) |
| 2026-06-09 | Expose IssueOps gate contracts through MCP and skills | [record](adr/2026-06-09-expose-issueops-gate-contracts.md) |
| 2026-05-25 | Go language selection | [record](adr/2026-05-25-go-language-selection.md) |
| 2026-05-25 | Plugin vs external worker (external core + thin adapters) | [record](adr/2026-05-25-plugin-vs-external-worker.md) |

## Archived history

Hot reading path에서 벗어난 결정은 [adr/README.md](adr/README.md) 의 archived
ledger에 요약되어 있고, 전문은 `.issueops/archive/adr-history.md` 에 보존된다.
