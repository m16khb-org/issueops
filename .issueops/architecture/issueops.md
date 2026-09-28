# IssueOps capability verticals and ownership

> Family index: [`../ARCHITECTURE.md`](../ARCHITECTURE.md). This module owns the
> IssueOps v1 execution state, schema authority, capability verticals, the Orca
> execution boundary, the execution threat model, and the actor model. General
> state topology and lock discipline live in [`runtime.md`](runtime.md);
> component boundaries live in [`hexagonal-core.md`](hexagonal-core.md).

## IssueOps v1 execution state and schema authority

- IssueOps 위치: `~/.local/state/issueops/issueops_v1/issueops.db`, bucket `issueops_v1`. 한 row는 lifecycle evidence와 정확히 하나의 `Execution`을 저장한다. Execution은 canonical workspace, direct/Orca mode, generation-fenced lease, native process receipt, pending external intent, Orca resource identity, generation에 봉인된 issue-body/context-packet/owner-prompt digest identity, completion receipt를 가진다. terminal preparation intent가 삭제된 뒤에는 identity version 1과 이 세 digest가 resume의 trust root이며 현재 prompt template을 다시 렌더링하지 않는다. Version marker 없는 all-empty binding만 과거 record로 복구할 수 있고, versioned all-empty는 새 persistence invariant 위반으로 거부한다. 사용자 요청과 설계 검토 같은 freeform 값은 secret-like 패턴을 redaction한 뒤 저장한다.
- IssueOps v1의 현재 쓰기 버전은 `schema_version=1`이다. Missing/zero/future schema row와 legacy write-authority key(`execution_handoff`, `execution_workspace`, `ownership`, `remote_create_claim`)를 가진 row는 모두 generic `invalid state`로 byte-identical fail-closed다(`internal/adapter/issueops/execution_namespace_test.go`). Legacy namespace와 row/file은 자동 변환하지 않으며, 전용 reset 명령도 없다: 비현행 schema를 만나면 record를 다시 시작한다.

## Capability verticals

- 태스크 게이트 ledger는 PR readiness에 파일 존재 기반 opt-in으로 합성된다.
  신규 IssueOps cycle의 canonical ledger는
  `.issueops/issues/<provider-issue-number>/gates.md`다. 전역 `gates`
  capability는 이 경로를 먼저 찾고 generic `.issueops/gates/*.md`,
  root `GATES.md`, `gates/*.md`를 호환 경로로 읽는다. Linked issue 번호가 있는
  readiness는 자기 번호와 anonymous ledger만 판정하고 다른 번호는 warning으로
  건너뛰며, 같은 번호의 canonical·legacy ledger가 함께 있으면
  `duplicate_issue_artifact:<n>`으로 fail-closed한다. 미충족 게이트(unchecked 또는
  checked-but-EVIDENCE-pending)는 `gates_incomplete:<file>`로 pr 진입을 막고,
  ledger가 없으면 요구를 추가하지 않는다. 조회·평가는 함수 변수로 주입되고
  composition root만 배선한다(`loopgate`와 같은 구조). 상세 계약은
  [ADR 2026-08-22](../adr/decisions/2026-08-22-task-gate-ledger.md)를 참조한다.

- `execution release`는 첫 production vertical이다. CLI/MCP transport facade는 injected release handler만 호출하고, `internal/contract/issueopslease` decode → pure `internal/domain/issueopslease` → capability-local `internal/application/issueopslease` → inbound/outbound adapter 순서로 흐른다. decode는 persisted record를 production record contract(`internal/contract/issueops`)로 엄격하게 읽어 모르는 field를 거부하고, execution만 typed로 다루며 나머지 sidecar는 원문 그대로 보존한다(ADR 2026-09-23). `cmd/issueops/issueopsapp`만 SQLite store, process observation, clock, filesystem path matcher를 조립한다. two-argument `ReleaseExecution` facade는 제거됐고 `TestCurrentIssueOpsVerticalOnly`가 재도입을 막는다.
- `execution reconcile`의 Orca `worktree_create`·`owner_launch`·`dispatch` confirm도 같은 vertical 경계를 사용한다. kind-local router가 injected handler로 보내고, application은 호출당 현재 durable stage 하나만 inventory/adopt 또는 bounded retry/CAS한다. preview와 no-pending은 side effect가 없는 compatibility router에 남는다.
- `issueopsdelegation.ChildGates`가 현재 자식 레코드 조회와 PR·재검토 게이트 조합을 맡는다. 자식 선택·종료 판정은 domain에 두며, 조회 실패는 차단하고 기존 span 안에서도 추가 잠금이나 쓰기를 하지 않는다.
- 원격 PR/MR 생성과 `remote_pr_create` 복구는 `issueopspublication` capability vertical이다. `internal/contract/issueopspublication`의 stable mapping → pure domain decision → shared `CreateService`/`ReconcileService` → inbound/outbound adapter 순서로 흐르며, `cmd/issueops/issueopsapp`만 provider, raw schema v1 CAS bridge, live verifier를 조립한다. CLI create와 CLI/MCP reconcile은 같은 request-scoped handler pair를 사용하고, handler가 없으면 legacy full-flow로 우회하지 않고 fail closed한다.
- 최초 parent issue 생성은 record의 `IssueCreateIntent`가 권위다. Provider 호출
  전에 operation marker, provider, canonical project authority, title/body
  digest, labels/assignees를 원자 기록한다. 호출 시작 전 실패만 같은 sealed
  request의 재시도를 허용하고, 시작 후 timeout/error/malformed output/live
  verification 실패는 자동 재시도 없이 reconciliation을 요구한다. 정확히 한
  marker candidate만 title/body digest와 live label/assignee 검증 뒤
  `IssueURL`+completed receipt 한 CAS로 채택한다.

## IssueOps operational surface

- IssueOps 제공 표면: 기존 lifecycle/domain CLI와 함께 `issueops execution prepare/status/claim/release/replace/reconcile/switch-mode/complete`, generation-fenced `issueops remote create-pr`를 제공한다. 단계 운영 표면으로 `issueops artifact stage/unstage`(prepare 전 스테이징·materialize·orca packet manifest 봉인), `issueops implementation-review record`(publication fail-closed 게이트, execution이 있는 모든 모드에 적용, 변경 집합 fingerprint 바인딩), `issueops next`(read-only 단계 투영 — `issueopsnext` vertical이 소유하며 record와 로컬 관측만 쓰고 fetch·provider·Orca 호출을 하지 않는다. stage key·index·missing·next_command·exits를 돌려주고, 키를 스킬 이름으로 바꾸는 표는 라우터 스킬이 소유한다), `issueops list`(read-only 다중 사이클 집계, 단일 SQLite snapshot의 물리 `scanned_records`, bounded invalid-row diagnostics와 pending/failure/cleanup-failure/issue-create projection), `issueops cleanup abandon`(미머지 사이클 폐기 — `--close-pr`·`--close-issue`·`--delete-remote-branch`로 원격 효과를 opt-in하며, 원격 효과는 record 삭제보다 먼저 실행되고 플래그와 원격 브랜치 OID만 fingerprint에 들어간다), `issueops cleanup finish`(record-backed 머지 후 정리 — orca 회수→git worktree 제거→브랜치 CAS 삭제→감사 라인 멱등 반영→레코드 삭제, resumable), `issueops remote create-issue/reconcile-issue`(intent-first parent issue 생성과 zero/one/many marker reconciliation), `issueops remote reflect-completion/close-issue`(completion 섹션 보존·부모 이슈 close, 원격 readback fail-closed)를 제공한다. execution prepare는 `--owner-model` 미지정 시 host별 implementer 기본값(codex gpt-6-sol/high, claude claude-sonnet-5/high)을 적용하고, owner 프롬프트에 planner급 reviewer 모델(codex gpt-5.6-sol/xhigh, claude claude-opus-5/high)을 렌더한다. Claude의 Fable 5는 자동 기본값이나 폴백으로 쓰지 않고 명시적 수동 지정으로만 사용한다. IssueOps MCP 표면은 정확히 하나인 `issueops_execution`이며 action으로 같은 execution state machine을 호출한다. `execution prepare`가 provider branch의 exact base SHA에서 fixed sibling worktree를 만들거나 top-level·branch·HEAD가 모두 일치하는 기존 워크트리를 채택하고, direct는 caller에게 generation 1을 부여하며 Orca는 sealed packet/prompt/token file과 claimable lease를 만든다. External mutation은 intent-first이고 ambiguity는 reconcile 전까지 fail closed다. `execution complete`는 phase `pr`, active generation, final HEAD, committed verification report, verification, exact verified remote URL을 요구하며 `done` 전이와 lease release를 원자적으로 기록한다.

## Generated command authority

- IssueOps가 생성하는 `next_command`는 prepare/status/replace/resume/sync-base/switch-mode preview와 cleanup preview/finish를 포함해 첫 token을 생성 바이너리의 canonical executable literal로 렌더하고 같은 path·SHA-256·lease generation envelope를 포함한다. 실행 authority를 제거하는 switch-mode apply는 shell command를 만들지 않고 non-command `next_action`만 반환한다. 사람이 직접 입력하는 일반 `issueops` PATH UX는 바꾸지 않는다. CLI와 MCP composition은 outbound executable observer를 통해 같은 envelope를 결합하고, IssueOps root는 subcommand mutation 전에 현재 executable과 durable generation을 대조한다. Hook은 absolute token이 envelope와 일치하는 것만으로 신뢰하지 않고 durable worktree 또는 source root의 canonical `bin/issueops`인지 먼저 제한한다. 관측 실패·stale binary·generation drift에는 command를 내보내거나 fallback하지 않으며 structured error로 중단한다. Contract는 DTO·pure bind/validate만 소유하고 port는 contract를 import하지 않는 순수 observation receipt를 소유한다. 실제 observer 생성은 `issueopsapp` composition root에만 두고 executable 관측 I/O는 outbound adapter에만 둔다.

## Actor model

- Actor model: main agent는 safety/reversibility/user-intent judgement와 child result acceptance를 소유한다. IssueOps의 active native holder는 exact lifecycle ID, generation, process receipt, canonical cwd 안에서만 쓴다. Hook은 관찰·차단·relay만 담당하고, phase 진행·workspace 준비·테스트·publication·merge·cleanup을 대신 실행하지 않는다.

## Optional Orca execution boundary

Orca integration is an optional execution adapter, not a native-install
dependency or second scheduler. `issueops execution prepare --mode auto`
probes readiness before mutation. `auto` resolves to direct only when Orca is
absent or unready at that pre-mutation boundary. After a possible Orca mutation,
the durable pending intent and explicit reconciliation path are authoritative.
Provider capability is part of that boundary. GitLab-linked execution accepts
one bounded, exact-identity issue snapshot observed through a host-configured
`glab_api` capability, or reads the same fields through the generic `glab api`
provider adapter when no MCP snapshot is supplied. MCP server namespace,
personal wrapper identity, credential profile, and token remain outside core;
only the normalized provider/source/URL/body/state DTO crosses the port. Exact
base SHA and branch upstream are separate identities; the SHA creates the
worktree and the remote issue branch is restored as upstream after namespace
canonicalization.
엄브렐라 자식 cycle은 branch prepare의 명시적 `parent_worktree`와
`base_branch`에서 canonical 부모 worktree 경로를 봉인한다. 기존 delegation
cycle은 명시값이 없을 때 같은 경로를 계산해 하위 호환한다. Orca adapter는 생성 시 그 경로를
`--parent-worktree`로 명시하고, 응답의 lineage가
`explicit-cli-flag`/`explicit`인지 검증한다. 독립 cycle만 `--no-parent`를
사용하므로 Orca UI 계층과 IssueOps의 provider-native 부모 관계가 일치한다.

## IssueOps execution v1 threat model and invariants

### Adversarial multi-session model

- One record has one `Execution`, one canonical worktree, and one active
  generation at a time.
- The trust boundary is the exact native actor: host, session/agent ID, process
  PID/start/executable receipt, canonical cwd, lifecycle ID, and generation.
- Branch names, source cwd, generic session bindings, terminal handles, and
  stable diffs are not write authority.
- Hooks are default-deny guards for mismatched mutation, not schedulers or lease
  grantors.

### Generation fence and sealed owner context

- Every mutating transition requires the active generation and matching native
  actor/cwd. Stale generations fail before CAS. This includes the evidence
  records that gate publication (`implementation-review`, `project-docs-review`,
  `schema-evidence record`): the CLI passes the parsed actor flags to the core,
  and `cmd/issueops/issueopscli/issueops_actor_flags_test.go` rejects an actor
  flag registration whose result is discarded.
- Direct preparation grants generation 1 to the caller. Orca preparation stores
  a claimable generation and seals the remote issue digest, private context
  packet, fully rendered prompt, owner host/model/effort, and stable Orca
  resource IDs.
- Orca claim resolves and consumes the current-generation private token exactly once and requires both
  sealed SHA-256 values. Token contents never enter state, prompts, logs, or
  responses.

`issueopspreparation` domain은 준비·재개·폐기 조회에 공통인 이슈와 workspace 신원을 검증하며, 검증된 자체 호스팅 GitLab URL도 허용한다. `IntentRequestBuilder`의 조회는 봉인된 메타데이터만 사용하고, 실행은 현재 pending·generation과 토큰·프롬프트·컨텍스트 파일의 digest까지 검증한다. `LaunchHydrator`는 현재 레코드와 intent를 다시 읽으며, adapter는 SQL·파일 읽기와 DTO 변환만 맡는다.

### External intent and lock discipline

- Workspace and remote PR/MR creation persist intent before calling the adapter.
  Timeout or error is ambiguity, not absence; retry and mode fallback remain
  blocked until `execution reconcile` proves one exact outcome.
- Parent issue creation follows the same intent-first rule independently of the
  execution lease. Only a proven `not_invoked` outcome is retryable.
  `invoked_unknown`, observed URL, verification failure, and receipt failure are
  Doctor findings until `remote reconcile-issue` adopts one exact candidate.
- sqlstore `BEGIN IMMEDIATE` spans serialize record CAS. A span covers the whole
  state root, not one cycle (ADR 2026-07-07), so a call that stalls inside one
  span stalls every cycle's writes. No network call (`git fetch`, `ls-remote`,
  `push`, provider CLI or API readback, Orca) runs while a span is held.
  Observations that feed a write run before the span: the upstream fetch for
  `pr` entry, the retarget readback and origin lookup, and the change-set
  fingerprint of evidence records. The span re-reads the record and uses an
  observation only if its inputs (git root, prepared base, remote artifact,
  requested branch) are unchanged; otherwise the command fails and is retried.
  Local Git reads that re-validate a CAS precondition may still run inside the
  span, such as the HEAD re-check in `cautions/issueops-orchestration.md`.
  `internal/adapter/issueops/span_network_guard_test.go` probes the span lock
  from a fake `git` for `pr` entry, retarget, and the evidence records; a new
  network call in another span needs its own probe case.
- Remote intent stores generation and native actor. Finish/reconcile rejects a
  changed generation, holder, cwd, branch, or provider result.

### Replacement and completion

- Replacement is preview → revoke → finalize-preview → finalize. Inventory and
  quiescence fingerprints, expected generation, actor, cwd, and explicit
  confirm are required; there is no unsafe override.
- Completion requires phase `pr`, active generation, exact final HEAD, committed
  verification report, verification evidence, and a durable verified remote artifact
  at the exact URL. The completion receipt, `done` transition, and lease release
  are one atomic state mutation.
- Completion never merges or deletes local/remote resources. Cleanup remains a
  separate human-authorized operation based on current merge and cleanliness
  evidence.
- A completed replacement first observes the fetched parent base through the
  `issueopsbasesync` port. Parent drift returns the typed
  `post_completion_sync_base_required` contract before owner inventory,
  artifact preparation, token creation, completion archival, or record CAS.
- Post-completion sync authority is the released lease plus its current stamped
  completion generation, canonical cwd, live native actor, and preview
  fingerprint. Completion history never restores current authority. The only
  recovery sequence is released current completion + drift → sync-base preview
  → generation/fingerprint-fenced sync-base apply → replacement preview →
  reseed/claim → verify and re-complete.
- `internal/port/issueopsbasesync` owns only request, receipt, and interface.
  The public typed error and exact next-command projection belong to
  `internal/contract/issueops`; Git/network observation belongs to the outbound
  adapter.
- The #303 provenance boundary binds typed-error `next_command` on the reachable
  CLI and MCP error paths. CLI conflict `next_command` and `abort_command` share
  one observed canonical executable/hash/generation receipt; observation
  failure exposes no unbound fallback. As #326 records, the public MCP action
  enum excludes sync-base and therefore has no sync-base success action/result;
  AC-06 host parity is the exact CLI command plus Codex/Claude hook classifier,
  while MCP binds only reachable resume/replace `BaseSyncRequiredError` output.

## Execution boundary

Workspace provisioning and lease grant are one execution transaction. The
source main worktree remains available before, during, and after direct or Orca
execution for unrelated work. A generic session binding is routing metadata
only. The fence selects the exact lifecycle ID, generation, native process
receipt, canonical worktree, and persisted Orca identity.

One active execution exists per record, not per source repository. Exact-ID
routing therefore keeps parallel cycles independent. The active holder performs
the remaining gates, implementation, publication, and completion in its
canonical worktree. Completion records `done` and releases the generation;
later merge and cleanup require separate current evidence and authority.

Post-merge cleanup ordering is a contract: `reflect-completion`(completion
섹션에 최종 head·PR URL·검증 요약·artifact 본문 보존) → `close-issue` →
`cleanup finish`. finish는 preview 게이트(원격 readback fail-closed·요청자
보호·head OID CAS·fingerprint) 뒤에만 파괴 단계를 수행하고 마지막에
레코드를 삭제한다. 워크트리를 점유한 프로세스와 그 워크트리에 매인 Orca 터미널은
차단 사유가 아니라 apply ①′의 종료 대상이다: preview가 receipt(pid·시작 시각·실행
파일)와 터미널 handle을 fingerprint에 결속하고, apply가 fingerprint된 handle마다
`orca terminal close --terminal`(same-handle·`ptyKilled=true` receipt 필수)를
호출한 뒤 HUP+TERM → KILL → 최종 점유·터미널 재관측(둘 다 0 증명) 순서로 닫은
뒤 orca 회수로 넘어간다. bulk `terminal stop --worktree`는 fingerprint 밖 동시
생성 터미널까지 닫을 수 있어 쓰지 않는다. 터미널 close 실패는 fail-closed다
(`workspace_processes_stop`, 시그널 없음). 요청자 자신(pid 조상, `ORCA_PANE_KEY`/`ORCA_TERMINAL_HANDLE`로 확정한
요청자 터미널)과 소스 체크아웃은 종료·삭제 대상이 될 수 없어 preview가 거부한다
(#477) — 결정적 ID(`sha256(repo+branch)`) 재사용과 충돌하지 않는
유일한 수명 종료다. 각 파괴 단계는 멱등이며, 실패 시 레코드가 보존되고 재실행
전 preview 재발급이 요구된다. prune은 completion 미반영 + RemoteArtifact 보유
레코드를 나이와 무관하게 보존한다(보존 불변식). staged artifact의 수명은
레코드와 같다(deleteIssueOps가 스테이지 버킷을 동반 삭제).

Cleanup `finish`, `remote-branch`, `abandon`은 같은 cycle의 실행 잠금과 `cleanup_attempt`를
공유한다. attempt의 operation·token과 관측 당시의 원본 레코드를 CAS로 결속하며,
일반 writer는 attempt가 있는 레코드를 변경할 수 없다. 각 외부 효과 직전에
소유권을 확인하고, provider 감사 반영 뒤에도 같은 레코드에만 receipt를 기록한다.
상속된 자식 프로세스가 모두 종료됐음을 drain으로 확인한 뒤 finish는 레코드를
삭제하고 remote-branch는 attempt만 해제한다. abandon은 소유한 intent 행·staged artifact·
레코드를 한 트랜잭션으로 삭제한다. 다른 operation의 attempt는 인계받지
않으며, 해당 정리 명령으로 복구해야 한다. 원격 ref가 이미 없으면 preview는
레코드를 쓰지 않는다. 같은 operation의 중단된 attempt가 남아 있을 때는 명시적인
apply가 새 token으로 인계받아 drain·해제하며, 삭제나 감사 반영을 했다고 기록하지
않는다. [공용 cleanup 소유권 결정](../adr/2026-09-29-cleanup-ownership-binds-the-operation-and-exact-record-revis.md)이
이 경계의 정규 근거다.

`abandon`의 대상 선택·사유 검증·폐기 허용 조건·자식 미완료 판정·실패 후 재시도
규칙은 `internal/domain/issueops`가 소유한다. 파일·Git·프로세스 관측은 외부에서
전달하며, 관측 실패를 자원 부재로 바꾸지 않는다. 승인 inventory는 contract DTO로,
fingerprint와 실패 기록의 봉인은 `internal/application/issueopscleanup`에서 만든다.
`close_pr`, `close_issue`, `remote_branch_delete` 실패는 로컬 삭제 전이므로,
봉인된 로컬 자원의 존재 여부와 OID가 그대로일 때만 새 preview로 재시도할 수 있다.

`AbandonExecutor`는 provider 선택과 artifact 조회 전에 공용 실행 잠금을 얻고,
그 뒤 읽은 원본 레코드에 관측 결과와 attempt를 CAS로 결속한다. CLI가 전달한
`ArtifactUnmerged` 값은 승인 근거로 쓰지 않는다. 원격 효과 → 점유 프로세스 종료 →
워크트리 제거 → 브랜치 CAS 삭제 → drain → 레코드 삭제 순서를 application이 소유하며,
각 외부 효과 전에 같은 레코드의 소유권을 확인한다. 봉인된 과거 `applying` 실패 기록은
복구 판단의 근거일 뿐, 실행 중인 다른 abandon을 통과시키는 권한이 아니다.
취소된 로컬 Git 명령은 실제 삭제를 끝냈을 수 있으므로 `applying` 실패 기록을 보존하고,
새 preview가 남은 자원을 다시 관측하게 한다. 취소된 조회의 exit code를 부재로 해석하지 않는다.

원격 폐기에서는 domain이 요청별 조회 전제 조건과 preview 효과 목록을 결정하고,
`AbandonRemoteObserver`가 허용된 provider·Git 조회를 조율한다. 어댑터는 기존
`LinkedBranchRemoteRef`를 재사용해 `ls-remote`가 반환한 ref가 요청한 브랜치와 정확히
일치하는 단일 행인지 확인한다. 성공한 빈 결과만 부재이며, 다른 ref·여러 행·불완전한
행은 `remote_branch_readable`로 거부해 fingerprint와 삭제 권한을 발급하지 않는다.

`AbandonPreviewer`가 로컬 자원·자식 cycle·pending intent·Orca owner 관측을 조합한다.
워크트리·브랜치·DB 읽기와 경로 해석은 adapter에 두고, 자식의 완료 여부와 Orca 잔여물
허용 조건은 domain에서 판정한다. preparation domain은 봉인된 단계까지 확인할 자원
순서를 결정하고, application은 그 결과를 lifecycle domain의 신원·부재 판정과 조합한다.
앞선 단계의 자원이 남았거나 부재 관측에 권위가 없으면 폐기를 거부한다. 실제 터미널은
점유 종료 단계에서 도달 가능한 경우에만 허용하며, 런타임 전환 관측 권한은 holderless일 때만 연다.

`cleanup orphan`의 요청 검증·삭제 허용 조건은 `internal/domain/issueopsorphancleanup`이,
preview·fingerprint·삭제 순서는 `OrphanCleaner`가 소유한다. 원격 병합과 Orca 관측은
잠금 밖에서 끝내고, apply는 sqlstore 쓰기 배제 잠금 아래에서 레코드·lease index와
로컬 Git 상태를 다시 읽는다. 새 소유자·잘못된 행·HEAD 변경이 있으면 삭제하지 않는다.
대상이 상태 저장소와 잠금 파일을 포함하면 symlink 경로도 정규화해 preview와 apply에서
거부한다. Git 자식은 배제 잠금을 상속하며, 취소 후 다음 삭제를 시작하지 않는다.
워크트리 제거 후 브랜치 삭제가 실패하면 부분 완료 필드를 보존한다. 성공은 drain 뒤에만
반환하며 임시 lifecycle이나 attempt를 만들지 않는다. CLI/MCP DTO와 정상 preview의
fingerprint 형식은 유지하고, 기존 orphan adapter 실행 함수·DTO 별칭·전역 Git 실행기는 제거했다.
