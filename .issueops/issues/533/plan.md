# #533 하위 이슈 생성의 중복 방지와 복구

Lifecycle: `io-339cd3178172`. Issue: https://github.com/m16khb-org/issueops/issues/533.
Source: `$SOURCE_ROOT`. Branch: `533-child-create-recovery`.
Base SHA: `b917fc960b72bca3e616df1162c6b89d40a517ba`. Canonical worktree는 execution prepare가 정한 sibling 경로를 사용한다.

## 목적과 승인된 종료점

사용자가 다섯 이슈를 병렬 native 세션에 인계하고 main 병합·cleanup·io-update까지 승인했다. 이전 조사 전용 제한은 최신 사용자 지시로 대체됐다. 이 작업의 native owner는 #533 구현, 독립 리뷰, 전체 필수 검증, draft PR 게시, 최신 push 및 PR CI 성공, execution complete/released까지 수행한다. main 병합과 cleanup 및 전역 설치는 root coordinator가 수행한다. 다른 네 이슈 파일을 바꾸지 않는다. 새 세션은 gpt-6.1-sol/high이며 GOFLAGS=-p=2 GOMAXPROCS=4를 process-local로 사용한다.

## 현재 근거

- `internal/adapter/provider/github/provider.go:405-412`는 선호 create의 모든 오류에 두 번째 create를 호출한다. URL+오류가 전달돼도 이 첫 URL이 사라진다.
- `internal/application/issueopsremote/child_create.go:76-85`는 원격 생성 뒤 validation과 Link를 호출하며, durable create intent가 없다. Link 실패 후 반복 호출은 다시 생성한다.
- `cmd/issueops/issueopscli/remotecmd/remote_child_pr.go:41-43`은 오류 때 성공한 ChildURL을 버린다.
- `internal/application/issueopsremote/issue_create.go:85-120`, `issue_create_intents.go`, `issue_reconcile.go`는 parent 생성에서 intent-first, URL 보존, marker 조회 및 exact candidate adoption의 선례다. 이를 child에 맞게 확장하되 부모 IssueURL을 덮어쓰지 않는다.
- 재현 입력과 실패 출력은 source의 `.issueops/evidence/audit-20260930/core-repro.log`, `runtime-evidence/runtime-provider-output.txt` 및 해당 overlay test에 보존돼 있다. 실제 원격 fixture는 생성하지 않았다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 정확성·최소 변경: 불확실한 원격 결과를 실패로 간주하여 재생성하지 않는다.
- `.issueops/ARCHITECTURE.md` 의존 방향, `.issueops/architecture/issueops.md` External intent and lock discipline: 순수 상태 전이는 domain, CAS orchestration은 application, provider I/O는 adapter, wiring은 composition root가 맡는다. provider/network 호출 중 SQLite span을 보유하지 않는다.
- `.issueops/CONVENTIONS.md` shared contract: CLI 및 향후 MCP 소비자는 같은 application result/error contract를 사용한다. 현재 없는 원격 MCP 명령을 추가하는 것은 범위 밖이다.
- `.issueops/CAUTIONS.md`와 `cautions/issueops-lifecycle.md`: actor/generation을 우회하지 않고 isolated state 테스트를 사용한다. 현재 schema_version=1 optional typed field 추가는 가능하지만 strict reader/canonical mapping 모두 갱신하며 missing/zero/future schema 자동 변환은 금지한다.
- `.issueops/ADR.md` 및 `adr/2026-09-23-the-lease-contract-decodes-persisted-records-through-the-pro.md`: production record contract가 strict decode 권위다. 새로운 child intent 필드를 release sidecar 등에서 잃지 않아야 한다.
- `.issueops/TESTING.md`, `testing/unit-and-contract.md`: failure injection, architecture, contract golden, race와 전체 Go 검증을 완료한다.

## 재사용하는 기존 구현

Parent `IssueCreateIntents`의 좁은 RecordStore CAS 패턴, operation marker 및 body digest sealing, provider candidate search, metadata live verifier, `IssueProviderCreateError.Invoked`를 재사용한다. 부모 생성 intent 자체와 completion 함수는 자식 생성에 직접 쓰지 않는다. parent IssueURL이 이미 있는 record에서 여러 child operation을 분리해야 하기 때문이다. 새 범용 workflow engine이나 provider abstraction은 만들지 않는다.

## 설계와 구현 순서

1. **RED**: audit fixture를 production test 구조로 가져와 fake gh 첫 create가 URL+exit1을 반환할 때 create 1회, local Link 실패 및 restart 뒤 같은 operation의 create 1회 조건을 먼저 실패로 확인한다. GitLab에도 same operation 반복과 hierarchy/metadata 실패를 주입한다.
2. **안전한 GitHub fallback**: `gh issue create --parent` capability를 read-only help 검사로 먼저 판별하고 지원하는 경우 preferred 호출을 한 번만 한다. 미지원이 확인됐을 때만 처음부터 plain create+attach 경로를 사용한다. capability 판별 오류는 create 전에 실패하며 create의 timeout/connection error/URL+error에는 재생성하지 않는다. known URL과 invoked 상태를 result/error에 보존한다.
3. **durable child operation**: record에 별도 optional typed child-create operation collection을 추가한다. operation_id(opaque generated ID), origin(implicit|explicit)와 normalized request fingerprint, provider/project authority/parent URL, title, normalized body digest, labels/assignees, marker, state, observed URL, holder/generation(실행이 있으면), timestamps를 저장한다. Body 원문이나 secrets는 저장하지 않는다. 각 operation은 pending → not_invoked 또는 invoked_unknown/url_observed/verification_failed/receipt_failed → completed로 전이한다. 첫 외부 호출 전 CAS로 pending을 기록한다. 호출 중 crash도 pending으로 남아 재생성을 차단한다. 명확한 not_invoked만 같은 요청 재시도를 허용한다.
4. **요청 identity와 복구**: create-child에 optional `--operation-id`를 제공한다. 최초 미지정 호출은 새 ID를 발급한다. 무ID 요청은 normalized sealed payload fingerprint와 origin=implicit로 최초 implicit operation에 영속 결합한다. 이 결합은 completed 상태 뒤에도 보존하며 explicit operation은 그 결합을 변경하거나 후보에 들어가지 않는다. 동일 fingerprint의 implicit operation이 있으면 completed 상태까지 포함해 그 최초 ID와 URL을 반환하고 재생성하지 않는다. 최초 implicit operation이 없을 때만 새 ID를 발급하고, fingerprint 당 implicit operation은 CAS에서 정확히 하나만 허용한다. 이 판정은 동일 RecordStore CAS 안에서 다시 검사하여 concurrent 무ID 호출도 create 1회를 보장한다. 동일 operation-id replay는 payload가 정확히 같아야 하며 completed이면 동일 child URL을 반환한다. 미해결 operation이 있으면 새 identity로도 추가 create는 차단한다. 완료 뒤 동일 payload를 의도적으로 새로 생성하려면 caller가 명시한 fresh --operation-id가 필수다. 무ID invocation만으로 새 의도를 추정하지 않는다. 다른 payload는 정상 신규 생성으로 처리한다. CLI help와 운영 안내에 Python secrets.token_hex(16)으로 ID를 먼저 한 번 생성해 저장하고 그 값을 최초 요청과 재시도에 재사용하는 예시를 넣는다. 결과에는 항상 operation ID를 제공하지만 첫 응답을 못 받은 무ID retry도 기존 completed URL을 재사용한다.
5. **reconcile-child**: `remote reconcile-child --id ... --operation-id ...` preview/confirm을 추가한다. known URL은 exact provider/project/parent/type와 marker/body/title/labels/assignees를 재조회한다. URL을 모르면 marker candidate search로 정확히 하나를 찾고 zero/many/truncated는 차단한다. hierarchy attach 실패가 있었으면 이미 생성된 child만 대상으로 안전하게 attach/verify하고 create를 호출하지 않는다. local Link와 completed receipt는 하나의 CAS로 원자화한다. 저장 실패 시 intent를 유지하여 다음 복구가 같은 URL을 채택한다. preview는 파일/record/provider mutation이 없다. 현재 actor/generation을 authorize하고 intent actor가 stale면 새 active holder의 명시적 reconcile만 허용하며 자동 create는 차단한다.
6. **표면·문서·회귀**: CLI 오류 JSON에 known URL, operation ID, 정확한 reconcile command를 보존한다. application contract로 동일하게 반환하여 host마다 해석이 갈리지 않게 한다. 명령 목록/golden/help와 운영 문서 및 관련 create-child skill만 실제 변경 계약에 맞춰 갱신한다. retired compatibility path는 남기지 않는다. 다른 이슈의 self-verify/CI 및 unrelated skill 정리는 하지 않는다.

## 성능 영향

원격 생성은 비빈번 경로다. 최초 호출에 bounded read-only capability 검사가 추가되고 반복 호출은 durable 상태 검사로 원격 재생성을 막는다. collection 조회는 해당 record의 child operations 수에 선형이며 기존 child evidence 규모 안에서 유지한다. 불필요한 polling, full-repo fetch, 전역 캐시는 추가하지 않는다. fake provider call count로 create·조회·attach 횟수를 단언하고 network outside span probe를 포함한다.

## 하위 호환성과 side effect

기존 `create-child` 명령 입력과 정상 출력의 기존 필드를 유지하고 operation/recovery fields를 additive로 넣는다. `--operation-id`와 reconcile-child는 additive이다. 기존 record에 새 필드가 없으면 operation collection은 비어 있는 현행 v1 record다. 부모 생성 intent, IssueURL, lease generation 필드는 바꾸지 않는다. release strict decode 및 schema contract tests를 실행한다. old binary에 새 record를 읽히는 rollback은 허용되지 않으므로 설치 업데이트는 root가 모든 병합 후 실행한다. 생성된 원격 이슈를 rollback 명목으로 삭제하지 않는다. URL 불명/중복 candidates는 fail closed로 남기고 사람이 정확한 결과를 확인할 수 있는 command/evidence를 제공한다.

## 검증과 게이트

검증은 isolated tmp state와 fake gh/glab subprocess fixtures로 수행하며 실제 remote create는 하지 않는다. 명령은 worktree에서 실행하고 전체 배터리는 같은 final HEAD에서 한 번 실행한다.

- G1: 중복 생성 회귀 | CHECK: `go test ./internal/application/issueopsremote ./internal/adapter/provider/github ./internal/adapter/provider/gitlab -count=1` | EXPECT: 모든 package PASS; URL+오류/timeout/disconnect/Link failure/restart 각 create count=1; unsupported capability 경로 성공.
- G2: identity·CAS·재시작 | CHECK: 실제 추가된 domain/record/application/CLI tests를 포함하는 `go test ./internal/domain/issueops ./internal/contract/issueops ./cmd/issueops/issueopscli/... ./cmd/issueops/issueopsapp -count=1` | EXPECT: unknown/pending blocks second create; exact ID replay; completed CAS 뒤 CLI 응답을 버리고 서비스/reader 재시작 후 실제 무ID CLI replay에서 동일 ID/URL과 create count=1(GitHub/GitLab 각각); concurrent 무ID 요청 create count=1; fresh explicit ID의 동일 payload는 의도적 create count=2 허용; implicit A completed 응답 유실 → 같은 payload의 explicit fresh-ID B 생성 → 재시작 → 무ID CLI replay는 A ID/URL 반환하며 create 총 2회 유지; CLI help에 ID 사전 생성·저장·재사용 안내; wrong payload/project/parent/actor rejection; preview zero writes; URL와 recovery command 유지.
- G3: 경계·공유 계약 | CHECK: `go test ./internal/architecture ./cmd/issueops/contractgolden -count=1` 및 `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1` | EXPECT: import direction, CLI/MCP response golden, strict v1 release 보존 PASS.
- G4: 완성 검증 | CHECK: 저장소 TESTING 표준의 gofmt/vet/full Go/race/build/docs/inspect/self-verify 단일 run | EXPECT: PASS 및 self-verify target >=95. GOFLAGS=-p=2 GOMAXPROCS=4로 다른 네 세션과 리소스 공유.
- G5: 게시 | CHECK: latest commit remote readback, PR diff review 및 exact-head push/PR CI | EXPECT: draft PR linked #533, checks success, execution complete 상태 done/released.

## 인계

prepare가 materialize한 이 계획을 link-plan하고 사용자 실행 방식 선택을 new-session/Orca로 기록한다. 실제 holder가 writer 0/자손 종료를 확인하고 release한 뒤 같은 worktree에 native Codex gpt-6.1-sol/high를 한 번 실행한다. 설치 CLI가 지원하는 bypass flag를 사용한다. receiver는 현재 HEAD/plan/material digest를 대조하고 next의 direct released replace chain을 자기 native actor로 수행한다. 원래 session은 구현하지 않는다.

Repo grounding: 위 source files, audit probes, docs, issue #533 body.
Decision-complete plan: intent-first child operation + conservative provider capability preflight + explicit reconciliation; one native owner, root integration.
Assumptions/defaults: no actual remote fixture creation; keep schema v1 optional additive data, isolated state fixtures.
Unresolved questions: none blocking.
Acceptance criteria: G1–G5 and issue completion criteria.
