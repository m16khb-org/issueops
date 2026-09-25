# 프로젝트 전체 DDD 책임 분리 실행 계획

> 실행자는 `superpowers:executing-plans`로 작업 단위별 구현·검증을 수행한다. 메인 에이전트가 설계와 통합을 소유하며, 독립 탐색과 적대 리뷰만 저장소의 sub-agent 규칙을 따른다.

**Goal:** 프로젝트 전체에서 업무 규칙과 상태 전이는 domain, use case와 트랜잭션 조율은 application, 외부 효과는 adapter, 공개 데이터 형식은 contract가 책임지게 만든다.

**Architecture:** 기존 Go modular monolith와 capability별 contract/domain/application/port/adapter 구조를 유지한다. 논리적 업무 경계를 정의하되 폴더 이름만 일괄 변경하지 않는다. 각 기능을 호출 진입점부터 저장·외부 효과까지 함께 이전한다.

**Tech Stack:** Go 1.26.3, 기존 SQLite/sqlstore, 현재 CLI·MCP SDK. 새 framework·ORM·event bus를 추가하지 않는다.

**Spec:** 이 문서의 §1–§4가 설계 명세이며, §5 이후가 실행 계획이다. 현재 사용자 지시와 `.issueops/CONSTITUTION.md`, `.issueops/architecture/hexagonal-core.md`, `.issueops/conventions/go-and-packages.md`를 적용한다.

**기준:** 2026-09-24, HEAD `fbf2c55317cf360e8cf514607054466fe44fbd4d`. 계획 작성 당시 `rg --files` 기준 비테스트 Go 파일은 1,057개(cmd 437, internal 620)다. 구현 시작 시 T01에서 추적 파일과 production package 기준으로 다시 고정한다.

## 1. 요청, 범위, 기본값

- 사용자 요청: “DDD 관점의 책임분리를 전체적인 리팩토링하기 위한 구체적인 계획을 세워줘”. 추가 지시: “범위는 프로젝트 전체야”, “구현진행”, “계속진행해”.
- 현재는 이 계획을 구현 중이다. 후속 사용자 지시로 구현 완료 후 커밋·PR 생성·머지·정리까지 승인되었다. 설치와 배포는 범위에 포함되지 않는다.
- 구현 대상은 `cmd/`, `internal/`의 전체 production 기능과 `scripts/`, `configs/`, `skills/`의 실행 경계다. `.issueops/` 운영 문서와 architecture test도 함께 정합화한다.
- 기본값은 **기존 관찰 가능한 동작 보존**이다. CLI 인자·exit code·JSON·오류 코드, MCP tool/schema, schema v1, SQLite bucket/key/record, 판정 순서와 오류 우선순위를 유지한다.
- 호환성을 깨는 재설계, 기존 버그 수정, 안전 정책 강화는 발견 항목으로 분리한다. 구조 이동 중 조용히 정책을 변경하지 않는다.
- Entity·Value Object·Aggregate는 불변식을 보호하는 데 필요한 경우에만 쓴다. Go의 순수 함수/reducer도 정상적인 도메인 구현이다. 모든 struct에 메서드를 붙이는 작업은 하지 않는다.
- 예상 규모는 XL이다. 기간을 소스 파일 수만으로 확정하지 않는다. T01 이후 capability별 영향 함수와 검증 비용을 기준으로 실행 일정을 산정한다.

### Repo grounding

- `internal/domain/issueopslease/claim.go:ValidateClaim`, `internal/domain/issueopspreparation/decision.go:Decide`는 순수 규칙의 기존 사례다.
- `internal/adapter/issueops/issueops_regress.go:regressIssueOpsForReplanLocked`는 판정·상태 변경·저장이 섞인 사례다.
- `internal/adapter/issueops/devilsadvocate/devils_advocate.go:Validate/checkReviseRoundCap`와 `internal/contract/issueopspreparation/planner_gates.go:MissingPlannerGates`에 업무 판정이 남아 있다.
- `cmd/issueops/riskqa/risk_qa_plan.go:PlanFromPaths`, `cmd/issueops/selfworkflow/summary/self_verify_summary_score.go:ScoreSelfVerificationGoals`에도 업무 규칙이 있다.
- `cmd/issueops/installcli/install.go:executeInstall`은 설치 트랜잭션 조율, `cmd/issueops/apidoc/api_doc_static.go:runAPIDocStaticCheckWithOptions`는 조회·판정·출력을 함께 맡는다.
- 아키텍처의 현재 금지 의존성은 `internal/architecture/dependency_test.go`, `ownership_manifest_test.go`가 검사한다. import만 지킨다고 규칙의 소유권까지 맞는 것은 아니다.

### Gap Analysis

1. 전체 범위를 몇 가지 발견 사례로 대체하지 않는다. T01이 모든 production 파일·함수와 비-Go 실행 파일을 소유자에 배정하고, T22가 미배정 0을 검증한다.
2. 기능별 package와 업무 경계는 같지 않다. 하나의 IssueOps record를 여러 package가 사용해도 한 저장 원자성 경계를 유지한다.
3. JSON shape 검증과 업무 불변식을 구분한다. malformed JSON/schema version/필수 wire 필드 검사는 codec에 남기고, 현재 상태에서 행위를 허용하는지는 domain으로 옮긴다.
4. lock 안에서 하던 검증을 밖으로 빼면 TOCTOU가 생긴다. 읽기→검증→전이→쓰기 전체가 기존 span/CAS를 유지하게 한다.
5. 자기 검증 도구 자체도 이동 대상이다. 검증기와 기준을 같은 변경에서 약화하지 않도록 원본 fixture와 별도 baseline binary를 사용한다.
6. 코드 재배치로 기존 package를 고정한 architecture test는 수정할 수 있지만, CLI/MCP/저장 결과를 고정한 golden을 편의상 갱신하지 않는다.
7. 코드 정리와 동작 개선을 분리한다. 예: 명령 실행기를 통일하면서 새 timeout·allowlist를 적용하는 것은 별도 동작 변경이다.

## 2. 목표 책임과 의존성

| 계층 | 소유 | 소유하지 않음 |
|---|---|---|
| domain | 행위 허용/거부, 불변식, 상태 전이, 점수·선정·보존·진행 규칙 | 파일·DB·Git·HTTP·process·환경변수 조회, wall clock 읽기, CLI 출력 |
| application | 관측 자료 수집 순서, domain 호출, 원자적 변경 조율, retry/reconcile 절차, port 호출 | SQL·OS 구현, CLI flag 파싱, 업무 규칙의 재구현 |
| contract | 공개 DTO, schema/version, 오류 vocabulary, 순수 codec/shape 검증, 데이터 복사 | 상태 기반 승인, 정책 판정, 다음 행동 선택 |
| port | capability별 외부 작업 interface, 좁은 snapshot/commit 계약 | concrete 구현, 범용 service locator |
| inbound/CLI/MCP | parse/shape validation, application 호출, render/exit mapping, schema 광고 | 저장 조율, 단계/점수/권한 판정 |
| outbound/host adapter | filesystem·SQL·process·network, 외부 형식 정규화, 기술적 안전성 | 업무 승인, phase 결정, 반복 횟수 정책 |
| composition root | concrete 생성·의존성 연결 | 업무 분기와 default concrete fallback을 다른 package로 숨기는 행위 |

기존 `domain/<cap> -> contract/<cap>`와 순수 domain helper 의존 원칙을 유지한다. 새 cross-capability DTO import 예외를 만들지 않는다. 다른 업무의 관측값이 필요하면 application이 해당 domain의 좁은 입력으로 변환한다. 기존 예외를 제거할 때도 한꺼번에 public DTO를 바꾸지 않는다.

### 논리적 업무 경계

| 경계 | 업무 용어·불변식 | 기존 capability와 범위 |
|---|---|---|
| 작업 사이클 | 의도, 단계, 계획, 리뷰, 게이트, 회귀, 부모·자식 | issueops, issueopsintent/decision/next/routing/artifact, preparation |
| 실행 권한 | actor, generation, lease, canonical workspace, claim/release/reseed | issueopslease/authorization/provenance, native actor 관측 |
| 외부 발행·정리 | remote intent, provider receipt, publication, completion, retention | issueopspublication/completion/remote/bodysync, cleanup/linking |
| 실행 정책 | command, workspace, policy tier, permit/deny, audit | policy, guard, gates, preflight, command execution |
| 호스트 설치 | install plan, activation, seal/abort, managed path, upstream | install, nativeactivation, update, codex/claude/omo/agy |
| 문서·품질 | document, revision, evidence, score, candidate, verification run | projectdocs/projectbootstrap, apidoc, quality, selfworkflow |
| 로컬 실행·관측 | job, loop, lifecycle, channel, health, trace | worker/looprun/lifecycle/channel/state/doctor/trace/toolconformance |

이 표는 새로운 microservice나 새 root 디렉터리 목록이 아니다. 같은 record에 저장된 cycle·execution·publication은 같은 commit에서 함께 검증한다. 기술적 MCP schema/argv/OS 호환성은 독립적인 도메인 모델로 과장하지 않는다.

### 상태 모델과 트랜잭션 결정

- schema v1 DTO와 저장 codec은 유지한다. 업무 모델은 해당 규칙에 필요한 typed snapshot으로 시작하며, 전체 record 복사 모델을 새로 만들지 않는다.
- 전이 함수는 관측 snapshot, 명령, 주입된 시각을 받고 결정/변경분 또는 typed error를 돌려준다. 실패 시 입력을 변경하지 않는다.
- application의 typed transaction interface는 **각 capability의 `application/<cap>/ports.go`**에 둔다. snapshot/changeset DTO는 같은 capability의 contract에 정의한다. publication/completion/preparation은 기존 architecture 규칙상 공용 `internal/port` import도 금지되므로 이 규칙을 그대로 지킨다.
- `internal/port/transactional_record_store.go`의 WithSpan/CompareAndApply는 **outbound 구현 내부에서만** 재사용한다. 이 기존 API는 ctx-only span과 raw rows이므로 typed domain callback인 것처럼 application에 노출하지 않는다.
- application-local callback은 context와 해당 capability의 typed snapshot을 받아 결정된 변경분을 반환한다. adapter는 load/decode/CAS/encode/persist를, application callback은 domain 호출과 결과 조합을 맡는다. 기존 `application/issueopsdecision/ports.go`의 Update처럼 typed callback이 있는 기능은 그 interface를 확장하고, lease/preparation/publication의 operation-specific repository에는 명시적인 typed transition callback을 추가한다.
- 외부 network 호출은 기존 lock 밖 preflight 위치를 유지한다. mutation 전 pending intent를 저장하고, 응답 모호성은 reconcile로 처리한다. create 자동 재시도나 direct fallback을 새로 넣지 않는다.
- DB가 여러 record를 한 번에 갱신하던 경우 batch expected records와 mutations를 유지한다. 하나씩 저장하는 Repository 메서드로 분해하지 않는다.

## 3. 공통 검증 계약

### 변경 순서

각 task는 다음 순서로 수행하며, 같은 task 안에서 구현과 테스트를 완료한다.

1. 기존 public/application 경로에 성공·거부·중복 호출 characterization test를 고정한다. 기존 구현에서 먼저 통과해야 한다.
2. 새 domain/application 경계 테스트를 작성하고 새 seam이 없어서 실패하는 것을 확인한다. 기존 테스트를 억지로 RED로 바꾸지 않는다.
3. 순수 판정과 effect orchestration을 분리하고 production wiring을 변경한다.
4. 원본 fixture를 새 경계와 public 경로에서 실행해 결과·오류·저장 상태·효과 횟수가 같은지 확인한다.
5. 기존 경로에 남은 정책과 무용한 facade를 제거하고 대상 테스트 및 architecture test를 실행한다.

**검증 증거:** `.issueops/evidence/ddd-refactor/TNN-<case>.txt` 및 구조화된 결과. 실제 사용 state/credential은 fixture로 복사하지 않는다. 실패 출력은 전문을 보존한다.

### Review Focus

- 동시에 같은 generation을 claim/release/reseed할 때 한 번만 성공하고 실패가 record를 변경하지 않는가(T05/T06).
- 공개 오류의 순서·nil/빈 배열·timestamp precision, 정의된 sidecar 보존 및 미정의 JSON field의 기존 reject/accept 정책이 보존되는가(T01/T02/T22).
- remote create 성공 뒤 응답이 끊긴 상황에서 중복 생성하지 않고 같은 intent를 reconcile하는가(T07).
- 설치 중 두 번째 host 실패, seal 실패, 프로세스 신원 변경 때 정확한 rollback과 신호 대상이 보존되는가(T11/T12).
- self-verify의 누락 증거·score 경계·부분 성공을 거짓 성공으로 바꾸지 않는가(T17/T18).

### 자동 검사와 의미 검토의 분리

- architecture test는 금지 import/직접 effect API, codec allowlist, 이전 완료한 정책의 구 경로 재등장을 검사한다.
- 의미적 책임은 T01의 **정책 소유권 원장**과 public→application→domain 테스트가 증명한다. 함수에 if가 있다는 이유만으로 domain 로직으로 분류하지 않는다.
- 정책마다 실제 application과 domain을 함께 사용하는 공개 경로 integration test를 연결한다. mock은 외부 port에만 두고, 정상 조건과 domain 거부 조건에서 공개 오류·저장 결과·effect 0회를 확인한다. CLI와 MCP 양쪽에 노출된 업무는 양쪽 경로를 연결한다. application spy는 transport dispatch 보조 증거로만 쓴다.
- 대표 정책(lease claim, review waiver, policy deny, self-verify score)의 domain 호출 제거/결과 무시 mutation을 격리된 임시 source copy에서 한 번씩 적용한다. 해당 공개 경로 test가 실패해야 한다. mutation은 원본 후보나 baseline을 수정하지 않고, 검증 뒤 임시 copy만 제거한다. compiler 실패만으로 mutation 검증 성공을 인정하지 않는다.
- 원장의 항목은 `ID, source symbol, source file, responsibility, target, owning task, public entrypoints, evidence tests, status`다. `status`는 migrate/retain/migrated 중 하나이며 retain에는 구체적인 기술 책임 근거가 필요하다.
- 새 production 파일이 원장에 없거나 migrate 항목이 남아 있으면 완료할 수 없다. 현존 위반만 유한한 이행 목록으로 두며 새 예외 추가는 금지한다. 완료한 항목은 이행 목록에서 삭제한다.

## 4. 실행 전략과 범위 원장

메인이 순차적으로 구현한다. 표의 독립 작업은 나중에 명시적으로 격리 구현을 선택할 경우에만 병렬화한다. 하나의 변경 집합에서 공용 DTO·store·composition root를 동시에 여러 사람이 수정하지 않는다.

| 단계 | 작업 | 선행 |
|---|---|---|
| 기반 | T01 inventory/baseline, T02 contract·domain 경계 | T01 → T02 |
| 작업 사이클 | T03 리뷰/회귀, T04 단계/의도/아티팩트 | T02 → T03 → T04 |
| 실행과 발행 | T05 lease/preparation, T06 저장 조율/복구, T07 발행/완료, T08 연결/자식/정리 | T04 → T05 → T06 → T07 → T08 |
| 정책과 설치 | T09 policy/preflight, T10 guard/gates, T11 install/activation, T12 update/upstream | T02 → T09 → T10; T02 → T11 → T12 |
| 문서와 runtime | T13 문서, T14 worker/loop/channel/state, T15 health/trace/tools | T09 → T13/T14; T14 → T15 |
| 품질·transport | T16 API/quality, T17 scoring/planning, T18 검증 실행, T19 catalog/host adapter | T13/T15 → T16 → T17 → T18 → T19 |
| 통합 | T20 모든 진입점 wiring, T21 문서·회귀 방지, T22 전체 검증 | T03–T19 → T20 → T21 → T22 |

최종 범위에는 이미 잘 분리된 package의 retain 판정도 포함된다. 기술 adapter의 경로를 옮기지 않았다는 이유로 범위 누락으로 보지 않는다. 반대로 원장에 기록하지 않은 capability를 조용히 제외할 수 없다.

## 5. 작업 목록

각 task의 담당은 메인 에이전트이며 권장 실행 등급은 별도 표기한다. 아래 신규 경로는 **생성 예정**이고 기존 경로는 근거다. 각 task는 해당 capability의 기존 모든 production 소비자를 `go list`와 `rg`로 찾아 같은 task에서 갱신한다. 내부 type alias는 이행 중에만 허용하고 제거 task를 명시한다. commit은 이번 계획 작업에서는 하지 않으며 실행 시에도 별도 승인 범위를 따른다.

### Task 1: 전체 소유권 원장과 동작 기준 고정 (T01)

- [ ] 완료 — 전수 목록과 기준선은 작성했으나 심볼별 의미적 책임 판정이 남음
- **담당/등급:** 메인 / deep. **선행:** 없음. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `cmd/`, `internal/`, `scripts/`, `configs/`, `skills/`; `internal/architecture/package_inventory_test.go`; `.github/workflows/ci.yml`
- **변경/신규 파일:** 신규 `internal/architecture/testdata/ddd_responsibility_inventory.json`, `internal/architecture/ddd_responsibility_test.go`, `.issueops/evidence/ddd-refactor/baseline/`.
- **구현:** 추적된 비테스트 Go 파일 전체를 먼저 AST로 열거해 모든 top-level 함수/메서드와 타입을 배정한다. build tag 때문에 현재 GOOS에서 제외되는 파일도 빠뜨리지 않는다. Linux/Darwin/Windows 및 실제 build tag 지원 조합의 `go list` production inventory와 교차 확인한다. 비-Go 파일은 실행/host 설정/스킬 지침/fixture로 분류한다. 순수 규칙, use case, codec, I/O, presentation, composition 중 하나와 T02–T21 담당을 지정한다. 스킬의 사람 판단 지침까지 Go 정책으로 옮기지 않는다. 새 파일 자동 탐지와 이행 항목 감소 검사를 만든다. source count를 목표 지표로 사용하지 않는다.
- **경계·보존:** 현재 CLI/MCP/record fixture를 고정하고 임시 디렉터리에 baseline binary를 빌드한다. baseline과 candidate는 별도 state/root에서 실행한다. 동적 값은 fixture clock/ID로 고정하며 불가피한 run ID/절대 임시 경로만 명시적인 normalization 목록에 둔다. error/status/schema/lease generation은 normalization 금지.
- **CHECK:** `go test ./internal/architecture -count=1`; `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1`.
- **EXPECT / QA:** 정상: 모든 production 파일과 executable script가 정확히 한 owner/task로 배정됨. 실패: 임시 synthetic AST에 미등록 정책 함수를 추가하면 inventory 검사 실패. 현재 full test/race의 실패는 기준 결함으로 기록하고 숨기지 않음.
- **Evidence:** `.issueops/evidence/ddd-refactor/T01-success.txt`, `T01-failure.txt`, `T01-ownership.json`.

### Task 2: Contract의 업무 규칙과 shape 검증 분리 (T02)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T01. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/contract/issueops/issue_create.go:ValidateIssueCreateTransition`; `internal/contract/issueopspreparation/{planner_gates,prepare,intent}.go`; `internal/contract/issueopslease/record.go`; 모든 contract package
- **변경/신규 파일:** 확장 `internal/domain/issueopspreparation/`, `internal/domain/issueopspublication/`; 신규 `internal/domain/issueopsreview/`; 신규 domain tests. contract codec 파일은 기존 위치 유지.
- **구현:** MissingPlannerGates, issue-create 상태 전이, intent의 record authority/verified-link 예외, host implementer defaults를 분리한다. 모델 기본값은 기존 값을 그대로 preparation policy에 둔다. 전체 contract의 함수 본문을 원장으로 분류해 상태 기반 판단은 해당 capability domain으로 이동한다. JSON decode/encode, clone, enum/size/schema validation은 contract에 남긴다.
- **경계·보존:** 여러 업무 검증이 섞인 Decode는 shape decode 결과와 별도 domain validation 호출로 분리하되 기존 reader의 generic invalid state를 유지한다. capability가 해석하지 않는 **정의된 sidecar**는 보존한다. schema에 정의되지 않은 JSON 필드는 각 기존 decoder의 reject/accept 정책을 유지한다. 예를 들어 lease Decode의 DisallowUnknownFields 거부를 허용으로 바꾸지 않는다. domain을 contract에서 import하는 순환을 만들지 않는다. 소비자는 같은 task에서 새 domain 함수를 호출한다.
- **CHECK:** `go test ./internal/contract/... ./internal/domain/... ./internal/architecture -count=1`.
- **EXPECT / QA:** 정상: 현재 허용된 intent/issue-create 전이 행렬 전체와 encode→decode 값/정의된 sidecar 동일. 실패: lease JSON에 미정의 필드를 추가한 fixture, schema missing/zero/future, stale generation, 허용되지 않은 전이는 종전 오류로 거부하며 raw record 불변. 다른 decoder의 미정의 필드 동작은 각각 baseline과 비교한다. contract의 migrated 정책 재등장 fixture는 검사 실패.
- **Evidence:** `.issueops/evidence/ddd-refactor/T02-success.txt`, `T02-failure.txt`, `T02-ownership.json`.

### Task 3: 리뷰·증거 기록·재계획 규칙 이전 (T03)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T02. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/issueops/devilsadvocate/devils_advocate.go`; `intentdesign/`; `compatibilityreview/`; `issueops_regress.go`; `issueops_{implementation_review,project_docs_review,schema_evidence,ledger_recorders,feedback}.go`
- **변경/신규 파일:** 신규 `internal/contract/issueopsreview/{types,snapshot}.go`, `internal/domain/issueopsreview/{review,regress,evidence}.go`, `internal/application/issueopsreview/{service,ports}.go`, 대응 outbound record adapter 및 root wiring. application이 기존 record를 이 capability의 snapshot으로 매핑한다.
- **구현:** waiver 이유, 리뷰 증거, 반복 상한, 변경 집합 applicability, feedback 해소, stop→replan 규칙을 typed 입력으로 옮긴다. 기록 시간은 주입한다. domain 전이는 approval 취소, ledger stale, review clear, phase 변경을 함께 반환한다. application은 change-set 관측을 span 밖에서 하고 locked snapshot의 authority와 증거 일치를 재검사한다.
- **경계·보존:** regress의 plan/compatibility 범위, stop/reflection 요구, revise 거부, regress 3회 제한, active children 거부와 오류 순서를 유지한다. snapshot 매핑은 application 내부에 두고 cross-capability contract 예외를 추가하지 않는다.
- **CHECK:** `go test ./internal/domain/issueopsreview ./internal/application/issueopsreview ./internal/adapter/issueops/... -count=1`.
- **EXPECT / QA:** 정상: stop+reflection+자식 없음에서 grill로 전이하고 audit를 보존. 실패: 네 번째 미면제 revise, 네 번째 regress, 근거 없는 pass, stale digest, foreign actor는 저장 0회. `TestEvidenceRecordersObserveTheChangeSetOutsideTheSpan`, regress cap/ledger 테스트 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T03-success.txt`, `T03-failure.txt`, `T03-ownership.json`.

### Task 4: 사이클 단계·readiness·아티팩트 조율 이전 (T04)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T03. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/issueops/issueops_phase.go`; `issueops_{readiness,pr_readiness,pr_readiness_strict,phase_ledger,phase_refresh}.go`; `gatesgate/`, `loopgate/`; `internal/application/issueopsartifact/`; 기존 issueopsnext/status/inventory/routing/retention vertical
- **변경/신규 파일:** 확장 `internal/domain/issueops/`; 신규 `internal/application/issueopscycle/{service,ports}.go`; 기존 artifact/next/status 등의 service와 root wiring.
- **구현:** validate/apply phase, phase ledger, readiness와 artifact binding을 domain으로 모은다. application은 gates/loop/provider/Git의 관측값을 조합한다. next/status 같은 projection은 기존 분리를 유지하되 Completion/WriterlessCommand 같은 callback 뒤에 숨은 legacy 판단까지 추적한다.
- **경계·보존:** phase 자체와 실제 artifact readiness의 차이를 유지한다. 생성 next_command와 missing key/정렬은 이전과 같게 한다. 읽기 경로가 missing ledger를 저장하거나 state를 초기화하지 않게 한다.
- **CHECK:** `go test ./internal/domain/issueops* ./internal/application/issueops* ./internal/adapter/issueops/... ./cmd/issueops/issueopsapp -count=1`.
- **EXPECT / QA:** 정상: 완전한 gate로 단계 진입 시 entered/completed ledger와 결과 일치. 실패: intent/plan/review 중 하나 누락, stale artifact, broken worktree는 같은 missing/error. `TestPRPhaseEntryFetchesUpstreamOutsideTheSpan`과 readonly status tests 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T04-success.txt`, `T04-failure.txt`, `T04-ownership.json`.

### Task 5: Lease·실행 준비의 실제 정책 소유권 완성 (T05)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T04. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/application/issueopslease/{claim,release}.go`; `internal/adapter/outbound/issueopslease/sqlite.go:claimWithinSpan`; `internal/application/issueopspreparation/prepare.go`; `internal/adapter/outbound/issueopspreparation/repository.go`; `internal/port/transactional_record_store.go`
- **변경/신규 파일:** 기존 lease/preparation domain/application/ports를 확장하고 outbound의 policy orchestration을 제거한다. 별도 중복 lease aggregate는 만들지 않는다.
- **구현:** application이 retry, cleanup applying fence, actor/generation/CWD/token 검증 순서를 소유한다. repository의 typed transaction callback 안에서 domain을 호출한다. token file의 mode/inode/hash 및 process identity는 adapter가 관측하고 동일 span의 current generation과 묶는다.
- **경계·보존:** claim 결과는 저장한 같은 transaction의 execution을 반환한다. record와 reverse-holder index를 원자적으로 갱신한다. prepared existing/mismatch/direct/Orca fallback 규칙과 readiness fingerprint를 유지한다. port에 default permissive 구현을 넣지 않는다.
- **CHECK:** `go test ./internal/domain/issueopslease ./internal/application/issueopslease ./internal/adapter/outbound/issueopslease ./internal/domain/issueopspreparation ./internal/application/issueopspreparation ./internal/adapter/outbound/issueopspreparation -count=1`.
- **EXPECT / QA:** 정상: current token claim과 동일 actor/generation retry의 결과·효과 수 동일. 실패: stale generation, ambiguous token selector, cleanup applying, apply failure에서 record/index/token authority 불변. `TestSQLiteClaimTransaction`, `TestSQLiteClaimRejectsCleanupAbandonFence` 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T05-success.txt`, `T05-failure.txt`, `T05-ownership.json`.

### Task 6: Resume·reconcile·reseed·base sync·mode switch 이전 (T06)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T05. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/issueops/execution_resume*.go`, `execution_reconcile*.go`, `execution_orca_intent.go`, `execution_sync_base.go`, `execution_mode_switch.go`; `internal/adapter/orca/execution_validation.go`; 기존 lease resume/reseed domain
- **변경/신규 파일:** 확장 lease/preparation application; 신규 `internal/domain/issueopsbasesync/`, `internal/application/issueopsbasesync/`; 대응 Git/intent port와 outbound 구현.
- **구현:** action eligibility와 receipt stage 전이를 domain으로, observation/pending checkpoint/effect/finalize 순서를 application으로 이전한다. invocation과 inspection의 receipt 요구가 다른 점을 별도 decision mode로 보존한다. 기존 resume/reconcile Effects bridge와 legacy 실행 경로를 제거한다.
- **경계·보존:** state-root BEGIN IMMEDIATE 범위·중첩 금지·CAS expected bytes를 보존한다. fetch/merge/push는 span 밖에 둔다. 완료 영수증의 immutable HEAD와 completion_history를 그대로 유지한다. mode switch의 workspace 제거와 provenance reset 순서도 기존 계약으로 고정한다.
- **CHECK:** `go test ./internal/application/issueopslease ./internal/adapter/outbound/issueopslease ./internal/adapter/issueops/... ./internal/adapter/orca/... -count=1`; 관련 package에 `-race`.
- **EXPECT / QA:** 정상: released resume과 holderless reseed에서 generation 증가/현재 receipt 처리 동일. 실패: stale raw snapshot/CAS, merge conflict, push failure, missing receipt는 rollback 또는 기존 pending 상태 보존. conflict에서 push 0회, ambiguous launch에서 create 0회 재시도. 기존 sync-base test matrix 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T06-success.txt`, `T06-failure.txt`, `T06-ownership.json`.

### Task 7: 원격 발행·본문 동기화·완료 이전 (T07)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T06. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/application/issueopspublication/{create,reconcile}.go`; `internal/adapter/outbound/issueopspublication/repository.go`; `internal/adapter/issueops/{execution_remote,execution_remote_bridge,issue_create_intent,issueops_remote_body_sync,issueops_remote_sync,issueops_completion_remote,issueops_devilsadvocate_reflect}.go`; `artifactverify/`
- **변경/신규 파일:** 기존 publication/completion vertical 완성; 신규 `internal/application/issueopsbodysync/`와 해당 narrow ports. 기존 domain remote/bodysync policy 재사용.
- **구현:** candidate matching, intent 전이, authority, managed-body 보존 규칙을 domain에 둔다. publication application은 intent 저장→provider→live verification→receipt까지만 수행한다. 명시적인 `execution complete`는 별도 use case로 두고 증거·HEAD·report를 검증한 뒤 completion과 release를 같은 transaction으로 기록한다. outbound repository의 Effects bridge를 제거하고 storage-only 연산으로 바꾼다.
- **경계·보존:** remote 결과 불명확 시 reconcilable intent를 남긴다. 발행과 완료는 merge와 cleanup을 자동 실행하지 않는다. provider parser/auth/HTTP/gh/glab는 기술 adapter에 남긴다. fixture provider로만 로컬 검증한다.
- **CHECK:** `go test ./internal/domain/issueopspublication ./internal/domain/issueopsbodysync ./internal/application/issueopspublication ./internal/application/issueopscompletion ./internal/adapter/outbound/issueopspublication ./internal/adapter/issueops/... -count=1`.
- **EXPECT / QA:** 정상: create readback 직후에는 RemoteArtifact/receipt가 생기고 기존 lease는 active, completion은 없음. 별도 complete 호출에 필요한 증거가 충족된 경우에만 completion+release가 원자 기록. 실패: provider 생성 후 응답 유실은 reconcile 요구, 두 번째 create 0회; 다른 candidate/actor/body SHA/HEAD는 거부. managed section 밖 본문 byte 보존.
- **Evidence:** `.issueops/evidence/ddd-refactor/T07-success.txt`, `T07-failure.txt`, `T07-ownership.json`.

### Task 8: 분기·부모자식·정리 capability 이전 (T08)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T07. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/issueops/{start,branchprepare,linking,delegation,cleanupchildren,cleanupstatus,orphancleanup}/`; `issueops_{umbrella_topology,delegation,child_gate,cleanup_*}.go`; `cleanup_workspace_*.go`; `issueops_linked_branch_observation.go`
- **변경/신규 파일:** 신규 `internal/domain/issueopsbranch/`, `internal/application/issueopsbranch/`, `internal/domain/issueopsdelegation/`, `internal/application/issueopsdelegation/`, `internal/domain/issueopscleanup/`, `internal/application/issueopscleanup/`; 기술 outbound adapters.
- **구현:** 8a: start/branch prepare/retarget/link의 relation/topology 규칙 이전. 8b: parent/child preconditions/profile/acceptance를 이전하고 related rows를 하나의 transaction으로 유지. 8c: cleanup inventory→eligibility→fingerprinted plan을 domain으로, preview/apply/reprobe/effects/failure receipt를 application으로 이전한다. 각 소작업은 독립 검증·통합 지점을 갖는다.
- **경계·보존:** cleanup applying fence와 expected raw state를 유지하고 실제 삭제 효과는 fixture Git/worktree/process에 한정한다. operationalhealth/linkedbranch 판단은 재사용한다. live process identity·inode·digest 검사는 adapter에 남긴다. 새 destructive recovery/cleanup 명령을 추가하지 않는다.
- **CHECK:** `go test ./internal/adapter/issueops/... ./internal/adapter/outbound/issueopsrecord ./internal/architecture -count=1`; 신규 branch/delegation/cleanup domain·application suite와 관련 `-race`.
- **EXPECT / QA:** 정상: remote-only branch 연결, concurrent siblings create/accept, verified cleanup의 stop→remove 순서 동일. 실패: artifact 관측 후 변경, authority CAS drift, 새 terminal 유입 시 삭제 중단. `TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses`, `TestCleanupFinishFinalTerminalObservationBlocksLateTerminal` 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T08-success.txt`, `T08-failure.txt`, `T08-ownership.json`.

### Task 9: Command policy·preflight·audit 조율 이전 (T09)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T02. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/policy/{policy_evaluate,policy_command_classification,policy_catalog,policy_run}.go`; `internal/domain/policy/`; `internal/adapter/preflight/`; `cmd/issueops/policycli/`; `cmd/issueops/commandstep/`
- **변경/신규 파일:** 확장 `internal/domain/policy/`; 신규 `internal/application/policy/{evaluate,run,ports}.go`; process/audit/override snapshot adapters.
- **구현:** Facts+catalog→decision을 domain으로 옮긴다. application이 canonical path 관측·workspace override 로드·evaluation·bounded runner·audit를 조율한다. timeout/env/secret 규칙의 결정과 실제 process 설정을 분리한다. preflight의 기술 검사와 업무 gate를 구분한다.
- **경계·보존:** override는 평가마다 workspace별로 로드하고 parse 경고를 기존 warnings에 보존한다. root/cwd 실체 검증은 string-only 검사로 대체하지 않는다. 다른 capability의 기존 실행 정책을 강화하지 않고 기존 runner 의미를 그대로 주입한다.
- **CHECK:** `go test ./internal/domain/policy ./internal/adapter/policy ./internal/adapter/preflight ./internal/adapter/audit ./cmd/issueops/policycli ./cmd/issueops/commandstep -count=1`; 신규 application/policy suite.
- **EXPECT / QA:** 정상: 두 workspace의 다른 override가 각각 반영. 실패: outside-root/symlink escape/deny command/secret env/override parse 실패에서 종전 verdict와 warnings 동일, denied marker 파일 없음. `TestPolicyOverridesLoadPerEvaluation` 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T09-success.txt`, `T09-failure.txt`, `T09-ownership.json`.

### Task 10: Guard·gate ledger 분리 (T10)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T09. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/guard/{findings,paths,symbols,summary}.go`; `internal/adapter/gates/check.go`; `internal/domain/gates/`; `cmd/issueops/gatescli/`
- **변경/신규 파일:** 신규 `internal/domain/guard/`, `internal/application/guard/`, `internal/application/gates/`; 기존 domain/gates 확장.
- **구현:** content/symbol facts의 finding/severity 규칙과 gate should-run/outcome 판정을 domain으로 옮긴다. application은 ledger 읽기→policy runner→결과 반영→원문 보존 write를 맡는다. 파일 탐색, Git, mode 보존 저장은 adapter에 둔다.
- **경계·보존:** exit=0과 EXPECT 일치를 함께 요구하는 현재 구현을 보존한다. stale 주석을 근거로 구현을 바꾸지 않는다. status-only는 실행·쓰기 없이 끝나야 한다.
- **CHECK:** `go test ./internal/domain/gates ./internal/adapter/gates ./internal/adapter/guard ./cmd/issueops/gatescli -count=1`; 신규 domain/application suite.
- **EXPECT / QA:** 정상: argv check 성공과 정확한 EXPECT에서 met. 실패: 비영 exit+EXPECT 일치, shell separator, timeout, missing evidence는 met 아님. 원본 ledger 비관리 텍스트·권한 보존, status-only bytes 불변.
- **Evidence:** `.issueops/evidence/ddd-refactor/T10-success.txt`, `T10-failure.txt`, `T10-ownership.json`.

### Task 11: 설치와 native activation 분리 (T11)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T02. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/install/install.go`; `cmd/issueops/installcli/{install,install_native_path,install_host_transaction}.go`; `internal/application/nativeactivation/service.go`; `internal/adapter/outbound/nativeactivation/sqlite.go`
- **변경/신규 파일:** 신규 `internal/application/install/`, `internal/domain/nativeactivation/`; 설치 계획 규칙은 기존 `internal/domain/upstream/`와 구분해 `internal/domain/install/`; install outbound/host adapters.
- **구현:** executeInstall과 InstallNative의 공통 절차를 application/install로 합친다. host-neutral 위치/선택 규칙은 domain/install, pending/receipt/idempotency와 7개 host/surface readback 조건은 domain/nativeactivation에 둔다. SQLite는 state mutation, 호스트는 실제 설정 쓰기만 맡는다.
- **경계·보존:** begin→host writes→seal→finalize/abort 순서와 실패 rollback을 유지한다. managed symlink·regular file·mode·inode·binary digest 검사는 adapter가 담당한다. native host 선택값과 agy 포함 readback 계약을 임의로 줄이지 않는다.
- **CHECK:** `go test ./cmd/issueops/installcli ./internal/adapter/install ./internal/application/nativeactivation ./internal/adapter/outbound/nativeactivation ./internal/adapter -count=1`; 신규 install/domain suite.
- **EXPECT / QA:** 정상: user-scope install에서 repo-local skill 생성 0, project-local opt-in의 MCP만 생성. 실패: host write/ seal 실패 시 기존 shell rc·symlink·빈 디렉터리 복원. dry-run은 파일 쓰기·외부 host CLI 실행·activation begin 모두 0. install matrix 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T11-success.txt`, `T11-failure.txt`, `T11-ownership.json`.

### Task 12: 업데이트·bootstrap·upstream·호스트 경계 정리 (T12)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T11. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `cmd/issueops/updatecli/update_bootstrap*.go`; `scripts/install-native.sh`; `internal/application/upstream/service.go`; `internal/adapter/{codex,claude,omo,agy,installutil}/`; `configs/`
- **변경/신규 파일:** 신규 `internal/application/update/` 및 update process adapter; 기존 upstream vertical과 host adapter 유지. scripts는 build/activation bootstrap wrapper 역할로 제한.
- **구현:** update/bootstrap command에서 설치와 daemon refresh 절차를 application으로 옮긴다. process inventory/argv/signal은 adapter, stale/eligible 판단은 normalized identity를 쓰는 domain/install 정책으로 둔다. shell wrapper는 바이너리 교체를 위한 bootstrap만 유지하고 host별 업무 규칙은 Go core 호출로 통일한다.
- **경계·보존:** 현재 활성 MCP는 host 세션 소유이므로 update가 종료/재시작하지 않는다. 별도 cleanup 경로의 PID/start/executable 재확인과 미지원 플랫폼 처리를 유지한다. upstream의 이미 분리된 Plan→Apply를 재작성하지 않는다.
- **CHECK:** `go test ./cmd/issueops/updatecli ./internal/application/upstream ./internal/domain/upstream ./internal/adapter/outbound/upstream ./internal/adapter/codex ./internal/adapter/claude ./internal/adapter/omo ./internal/adapter/agy -count=1`.
- **EXPECT / QA:** 정상: 다른 cwd에서 update가 정확한 source root와 인자를 전달. 실패: installed daemon stop 실패 뒤 후속 refresh 중단, PID identity 변경 대상에 signal 0. dry-run은 ps/host spawn과 설정 변경 0. 기존 `TestRefreshRunningMCPProxiesAfterInstallPreservesAllActiveProcesses` 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T12-success.txt`, `T12-failure.txt`, `T12-ownership.json`.

### Task 13: 프로젝트 문서·bootstrap·수정 use case 이전 (T13)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T09. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/projectdocs/project_docs_{route,revise,append}.go` 및 detection/profile 파일; `internal/adapter/projectbootstrap/project_docs_bootstrap.go`; `internal/domain/projectdoc/`; `cmd/issueops/projectcli/`
- **변경/신규 파일:** 기존 projectdoc domain 확장; 신규 `internal/application/projectdocs/`, `internal/application/projectbootstrap/`; file/template/Git adapters.
- **구현:** task routing, 관측 사실→project profile, revision SHA/summary/content 조건, bootstrap preserve/replace 결정을 domain으로 옮긴다. read/revise/append/bootstrap/sync 순서는 application이 맡는다. Markdown renderer는 presentation/technical helper로 유지한다.
- **경계·보존:** read/hash/write였던 동작을 원자적 CAS로 강화하는 변경은 여기서 하지 않는다. 기존 expected SHA 거부 의미를 유지한다. curated AGENTS와 사용자 작성 섹션, optional docs를 보존한다.
- **CHECK:** `go test ./internal/adapter/projectdocs ./internal/adapter/projectbootstrap ./internal/domain/projectdoc ./cmd/issueops/projectcli -count=1`; 신규 application suite.
- **EXPECT / QA:** 정상: compound task routing이 관련 문서를 누락하지 않고 sync 없는 bootstrap이 기존 본문 보존. 실패: stale SHA/빈 summary/없는 경로는 write 0, dry-run도 bytes 불변. `TestReadAndReviseProjectDocRequireSHAConsensus` 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T13-success.txt`, `T13-failure.txt`, `T13-ownership.json`.

### Task 14: Loop·worker·state·lifecycle·channel 분리 (T14)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T09. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/looprun/{lifecycle,gate}.go`; `internal/adapter/worker/{worker,read_only,store}.go`; `internal/application/state/{prune,doctor}.go`; `internal/adapter/lifecycle/lifecycle_project_state_store.go`; `internal/adapter/channel/store.go`
- **변경/신규 파일:** 신규 domain/application `looprun`, `worker`, `lifecycle`; 신규 application/channel; 기존 domain/state와 application/state 확장. channel에는 불필요한 aggregate를 만들지 않는다.
- **구현:** 14a: loop의 active/terminal/exhausted 전이와 증거 규칙 이전. 14b: worker enqueue/cancel/run/stuck 정책 이전; process 실행 전·후 transaction 분리와 상태 재확인을 유지. 14c: state retention/doctor의 순수 선택, lifecycle namespace/profile, channel send/recv wait 조율을 각 owner로 이전한다.
- **경계·보존:** loop 기본 5/최대 50 attempts와 success proof, worker queued-only cancellation을 유지한다. 실제 잠금·SQL·atomic file create·process liveness·wait timer는 adapters. channel 관측 집합을 전역 캐시로 바꾸지 않는다.
- **CHECK:** `go test ./internal/adapter/looprun ./internal/adapter/worker ./internal/adapter/lifecycle ./internal/adapter/channel ./internal/application/state ./internal/adapter/outbound/state -count=1`; 해당 상태 전이 package `-race`.
- **EXPECT / QA:** 정상: loop 마지막 pass 뒤 succeed, job queued→running→terminal 결과 동일, cross-process channel 가시성 유지. 실패: exhausted retry/terminal restart/running cancel/future schema/namespace mismatch 거부. `TestWorkerConcurrentCancelAndRunDoesNotLoseUpdates`와 readonly no-repair 테스트 유지.
- **Evidence:** `.issueops/evidence/ddd-refactor/T14-success.txt`, `T14-failure.txt`, `T14-ownership.json`.

### Task 15: Health·trace·conformance·분석 도구 분리 (T15)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T14. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/adapter/operationalhealth/collector.go`; `internal/adapter/doctor/{doctor,checks}.go`; `internal/adapter/trace/`; `internal/adapter/toolconformance/benchmark.go`; `internal/adapter/{commitsuggest,lintdiagnose}/`; `internal/adapter/hostprobe/`
- **변경/신규 파일:** 확장 domain/operationalhealth, traceclassification, toolconformance; 신규 application/doctor, trace, toolconformance, commitsuggest, lintdiagnose; 기술 collectors/runners 유지.
- **구현:** pending intent staleness, health severity/admission, trace classification/dedupe, benchmark completed-evidence/resume/gate를 domain으로 이전한다. application은 snapshot 수집·partial error·호스트 실행·분석을 조율한다. hostprobe 격리와 protocol parsing은 adapter에 둔다.
- **경계·보존:** missing/unknown/unavailable을 supported/pass로 바꾸지 않는다. lintdiagnose의 실행 정책·timeout 강화와 projectdoc 동시성 개선은 별도 발견 항목이다. 이번 task는 runner injection과 책임 이동만 수행한다.
- **CHECK:** `go test ./internal/domain/operationalhealth ./internal/adapter/operationalhealth ./internal/adapter/doctor ./internal/adapter/trace ./internal/adapter/toolconformance ./internal/adapter/hostprobe ./internal/adapter/commitsuggest ./internal/adapter/lintdiagnose -count=1`.
- **EXPECT / QA:** 정상: 같은 snapshot+clock에서 health findings와 trace 정렬 동일. 실패: stale signature/중복 identity/미완료 host probe는 hardening gate 불통과, live support로 표기하지 않음. read-only 진단의 state 생성 0, secret 원문 출력 없음.
- **Evidence:** `.issueops/evidence/ddd-refactor/T15-success.txt`, `T15-failure.txt`, `T15-ownership.json`.

### Task 16: API 문서 검사·quality·risk QA 이전 (T16)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T13,T15. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `cmd/issueops/apidoc/{api_doc_static,api_doc_review,staticcheck,reviewfiles,reviewprompt}/`; `cmd/issueops/qualitycli/`; `cmd/issueops/riskqa/{risk_qa_plan,risk_qa_git}.go`
- **변경/신규 파일:** 신규 contract/domain/application `apidoc`, `quality`, `riskqa`; API source readers/review runners/quality collectors는 outbound, CLI render는 기존 cmd에 유지.
- **구현:** API candidate/violation/응답 계약 판정, coverage/SNR health thresholds, PlanFromPaths를 domain으로 옮긴다. repo source/Git/coverage 실행과 evidence bundle 수집은 application+ports로 나눈다. review prompt render는 기술 presentation으로 분리한다.
- **경계·보존:** contract-tests skip, changed-file 기본 범위, no-candidate의 skipped 응답, score 임계값, coverage fingerprint cache key와 partial collector 오류를 보존한다. decorator 검사 개선은 포함하지 않는다.
- **CHECK:** `go test ./cmd/issueops/apidoc/... ./cmd/issueops/qualitycli ./cmd/issueops/riskqa -count=1`; 신규 세 domain/application suite.
- **EXPECT / QA:** 정상: 동일 API fixture의 code/file/line 정렬 일치, Go sensitive path→elevated 명령 동일. 실패: candidate 파일 read 실패, collector 실패, 잘못된 review result는 종전 오류; legacy 전체 Swagger 부채를 새 실패로 포함하지 않음.
- **Evidence:** `.issueops/evidence/ddd-refactor/T16-success.txt`, `T16-failure.txt`, `T16-ownership.json`.

### Task 17: 자기 검증·증강·IssueOps benchmark 정책 이전 (T17)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T16. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `cmd/issueops/selfworkflow/{summary,augmentplan,augmentcatalog,historycompare,candidateexport,llmeval}/`; `internal/adapter/issueops/benchmark/`; `internal/domain/qualitycatalog/`
- **변경/신규 파일:** 신규 contract/domain `selfverify`, `selfaugment`, `issueopsbenchmark`; 해당 application service와 state/judge adapters. 공용 순수 수치 helper는 실제 owner에 둔다.
- **구현:** ScoreSelfVerificationGoals, 후보 우선순위/충족, history retention/compare, lesson penalty, consensus/reliability/critical failure/benchmark gate를 순수 입력으로 이전한다. 저장 DTO는 기존 JSON 형식을 유지한다. exported facade 뒤의 실제 구현까지 이동한다.
- **경계·보존:** 현재 score > target 경계를 >=로 바꾸지 않는다. random seed/clock를 명시적으로 주입하고 누락 evidence의 분모/실패 처리, numeric tolerance, 후보 ID/순서를 고정한다. LLM judge 선택이나 prompt 의미는 변경하지 않는다.
- **CHECK:** `go test ./cmd/issueops/selfworkflow/... ./internal/adapter/issueops/benchmark/... ./internal/domain/qualitycatalog -count=1`; 신규 domain suites.
- **EXPECT / QA:** 정상: 같은 seed/fixture의 점수·후보 순서·reliability byte/허용오차 동일. 실패: score가 target과 같은 경우 pass 아님, required label 누락은 실패, stale judge provenance는 수락하지 않음. 빈 결과도 100점으로 바뀌지 않음.
- **Evidence:** `.issueops/evidence/ddd-refactor/T17-success.txt`, `T17-failure.txt`, `T17-ownership.json`.

### Task 18: 검증 실행기·risk step·저장 orchestration 이전 (T18)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T17. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `cmd/issueops/selfworkflow/{steps,verifycmd,augmentcmd,stateio,progress}/`; `cmd/issueops/validationcli/`; `cmd/issueops/commandstep/`; `cmd/issueops/selfworkflow/self_augment_loop.go`
- **변경/신규 파일:** 확장 application/selfverify·selfaugment; 신규 `internal/adapter/verification/`에 실제 fixture/build/process/SDK smoke driver; contract에 StepResult/run DTO. progress 렌더는 inbound.
- **구현:** 단계 선택·retry/종료·evidence reuse 결정은 domain, 단계 실행/저장/취소 조율은 application으로 이동한다. validationcli의 각 자식 package를 기술 probe와 판단으로 분류해 probe 실행은 verification adapter로 옮긴다. probe가 자기 자신을 다시 실행하는 순환을 만들지 않는다.
- **경계·보존:** risk race 결과가 full go test를 대체하는 조건과 golden reuse, step label/timeout/order, finite attempt cap를 그대로 유지한다. 기존 출력을 baseline으로 고정한 뒤 wiring을 바꾼다.
- **CHECK:** `go test ./cmd/issueops/selfworkflow/... ./cmd/issueops/validationcli/... ./cmd/issueops/commandstep -count=1`; 신규 application/verification adapter suites.
- **EXPECT / QA:** 정상: 성공한 full-suite evidence만 해당 run에서 reuse. 실패: 한 step 실패/cancel/timeout 시 후속 동작과 최종 OK가 기존 계약과 동일; 다른 run의 partial 성공 합성 금지. fake runner가 모르는 argv를 성공 처리하지 않음.
- **Evidence:** `.issueops/evidence/ddd-refactor/T18-success.txt`, `T18-failure.txt`, `T18-ownership.json`.

### Task 19: CLI·MCP catalog·host protocol 위치 정합화 (T19)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T18,T12. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `internal/domain/{cli,mcp,nativehost,omolifecycle,hook}/`; `internal/contract/mcp/`; `cmd/issueops/{mcpcli,hookcli,rootcmd}/`; `configs/`; `skills/`
- **변경/신규 파일:** 공개 static schema/descriptor는 contract, catalog assembly와 render는 신규 `internal/adapter/inbound/catalog/`, host argv/extension code 생성은 신규 `internal/adapter/hostprotocol/`. host별 소비자는 root에서 주입한다.
- **구현:** 업무 정책과 무관한 schema 생성/host syntax가 domain으로 오인되지 않게 위치를 정리한다. 순수 domain helper(정렬·token parsing·보안 판정)는 그대로 둔다. CLI/MCP 공용 업무 규칙은 같은 application으로 모으고 contract/codegen의 출력 동일성을 검증한다.
- **경계·보존:** catalog/argv/extension bytes, tool 이름·schema, hook SessionStart 역할을 보존한다. adapter끼리 새 import를 열지 않고 root가 필요한 builder를 주입한다. 스킬의 실행 명령 wrapper만 새 내부 wiring과 대조하고 스킬의 사람 판단 workflow를 새 core 정책으로 복제하지 않는다.
- **CHECK:** `go test ./internal/architecture ./cmd/issueops/contractgolden ./cmd/issueops/mcpcli ./cmd/issueops/hookcli ./internal/adapter/omo ./internal/adapter/codex ./internal/adapter/claude -count=1`; 이동한 catalog/hostprotocol 전체 test.
- **EXPECT / QA:** 정상: MCP SDK/legacy handshake의 advertised catalog와 extension 생성 bytes 동일. 실패: invalid arguments는 application effect 전에 거부, SessionStart에 mutation 추가 없음. project-local opt-in 없는 install의 repo 쓰기 0.
- **Evidence:** `.issueops/evidence/ddd-refactor/T19-success.txt`, `T19-failure.txt`, `T19-ownership.json`.

### Task 20: 전체 production wiring 전환과 잔여 facade 제거 (T20)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T03–T19. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `cmd/issueops/issueopsapp/*_wiring.go`, `*_facade.go`; `cmd/issueops/{basiccli,statuscli,daemoncli,workercli,loopcli,statecli,webfetchcli,contractcli,projectcli,issueopscli}/`; 모든 `*_dependencies.go`
- **변경/신규 파일:** 기존 composition root, migrated capability의 instance dependencies. 신규 architecture public-entrypoint conformance tests.
- **구현:** T01 원장의 모든 entrypoint를 추적해 CLI/MCP/daemon/worker가 해당 application을 실제 호출하는지 확인한다. façade가 legacy 정책으로 되돌아가는 경로를 제거한다. migrated 기능의 package-global mutable function setters를 instance dependency로 교체한다. main/rootcmd는 parse/dispatch만 남긴다.
- **경계·보존:** daemon socket/IPC/lifecycle와 MCP server loop 같은 transport 실행은 cmd/adapter에 남겨도 된다. retain 판정은 원장에 근거를 기록한다. inspect/docs/webfetch/upstream/sqlstore처럼 이미 적절한 vertical은 공통 facade 요구로 재작성하지 않는다.
- **CHECK:** `go test ./internal/architecture ./cmd/issueops/... -count=1`; migrated application 전체와 MCP concurrency tests에 `-race`.
- **EXPECT / QA:** 정상: 실제 application+domain과 외부 port fixture를 production handler에 연결해 CLI/MCP가 같은 결정·저장 결과를 낸다. application recorder spy는 DTO/호출 1회 검사에만 보조 사용. 실패: domain 거부 fixture에서 공개 error와 effect 0회를 확인하고, 대표 domain 호출/결과를 우회한 실행 가능한 임시 mutation에서는 해당 테스트가 실패해야 한다. 서로 다른 두 service instance의 dependency/state가 누출되지 않음.
- **Evidence:** `.issueops/evidence/ddd-refactor/T20-success.txt`, `T20-failure.txt`, `T20-ownership.json`.

### Task 21: 문서·소유권 검사·회귀 방지 마감 (T21)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T20. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `.issueops/{ARCHITECTURE,CONVENTIONS,TESTING,ADR,OPERATIONS}.md`와 각 family module; `AGENTS.md`; `internal/architecture/{dependency,ownership_manifest}_test.go`; `.github/workflows/ci.yml`; T01 원장
- **변경/신규 파일:** 신규 `.issueops/adr/ddd-responsibility-ownership.md`, `.issueops/architecture/domain-responsibilities.md`; 기존 index와 테스트 갱신. 문서 수정 시 project-docs workflow 적용.
- **구현:** 목표 layer/업무 경계/transaction ownership과 technical retain 기준을 문서화한다. migration allowlist를 0으로 만들고 contract 정책·transport 업무 규칙 재등장과 application I/O 우회를 검사한다. docs/count projection 변화는 계약 unchanged 증거와 함께 최소 갱신한다.
- **경계·보존:** 기존 architecture 금지 edge를 넓히지 않는다. pure helper의 존재 자체를 위반으로 보는 과도한 AST lint를 만들지 않는다. generated OpenWiki는 수동 수정하지 않는다. 프로젝트 문서의 오래된 경로는 현재 구조와 일치시키되 과거 사고 기록은 역사로 유지한다.
- **CHECK:** `go test ./internal/architecture -count=1`; `uv run --directory skills/project-docs-optimize python -m scripts.check --root "$PWD" --mode check --json`; 변경된 skills의 repo validator.
- **EXPECT / QA:** 정상: 모든 원장 항목은 migrated 또는 근거 있는 retain, dangling link 0. 실패: contract에 이전한 state transition을 재도입한 synthetic fixture는 검사 실패. DTO shape helper는 허용. docs index projection 외 CLI/MCP golden 변화 0.
- **Evidence:** `.issueops/evidence/ddd-refactor/T21-success.txt`, `T21-failure.txt`, `T21-ownership.json`.

### Task 22: 전체 호환성·동시성·실제 진입점 검증 (T22)

- [ ] 완료
- **담당/등급:** 메인 / deep. **선행:** T21. **병렬:** 기본 NO. **Commit:** 별도 실행 승인 범위에서 task 단위 Conventional Commit + Lore body.
- **기존 근거:** `.github/workflows/ci.yml`; `.issueops/testing/{unit-and-contract,concurrency-and-race,cli-mcp-and-hosts,issueops-execution,self-verification}.md`; T01 baseline
- **변경/신규 파일:** `.issueops/evidence/ddd-refactor/final/`에 명령·출력·scenario 결과·원장·리뷰 증거.
- **구현:** §6의 battery와 scenario를 깨끗한 후보 상태에서 실행한다. baseline/candidate CLI·MCP·persisted state differential을 같은 fixture로 비교하고 public command에 대응되지 않는 새 application 경로가 없는지 확인한다.
- **경계·보존:** 전체 테스트 통과만으로 책임 분리를 완료 선언하지 않는다. 정책 소유권 원장 미완료 0, migrated 정책의 이전 경로 구현 0, CLI/MCP 실제 application wiring, 원자성·실패 효과 보존 증거가 모두 필요하다.
- **CHECK:** §6 명령을 사용한다. 실패한 항목만 숨기거나 baseline으로 승격하지 않는다.
- **EXPECT / QA:** 정상: 모든 task의 성공·거부 시나리오와 baseline comparison 일치. 실패: stale CAS/동시 writer/remote ambiguity/install failure/cancel/unknown schema에서 기존 안전한 종료·상태·오류를 보존. 두 fresh-context reviewer가 각각 correctness와 verification coverage를 확인.
- **Evidence:** `.issueops/evidence/ddd-refactor/T22-success.txt`, `T22-failure.txt`, `T22-ownership.json`.


## 6. 최종 검증과 종료 조건

### 공통 battery

구현 완료 후 실제 후보 tree에서 실행한다. 아래 명령을 계획 작성 중 통과했다고 주장하지 않는다.

```bash
gofmt -l $(git ls-files '*.go')
go vet ./...
go test ./internal/architecture -count=1
go test ./internal/domain/... ./internal/application/... -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/issueops ./cmd/issueops
python3 -m unittest discover -s scripts -p '*_test.py'
```

`gofmt` 출력은 0줄이어야 한다. 설치된 golangci-lint가 있으면 `golangci-lint run ./...`와 `GOOS=linux golangci-lint run ./...`를 실행하고, 최종 CI의 같은 lint gate 결과를 확인한다. dependency 추가가 없으므로 `go.mod/go.sum`의 의도하지 않은 변경은 실패다. 코드 이동으로 생긴 import 정리가 필요할 때만 `go mod tidy` 결과를 검토한다.

모든 `skills/*/SKILL.md`에 `python3 scripts/validate-skill.py <skill-dir>`를 적용한다. docs checker도 T21 명령으로 실행한다. public endpoint/DTO/schema 변경으로 static gate 대상이 생기면 `./bin/issueops api-doc static-check --json` 및 저장소의 agent review 계약을 수행한다. 단순 내부 DTO 이동이 공개 schema 변경인 것처럼 golden을 일괄 갱신하지 않는다.

### 격리된 실제 명령·MCP QA

T01/T20에서 Go integration test를 만들어 `t.TempDir`, `t.Setenv`, `exec.CommandContext`를 사용한다. 대상 binary는 방금 빌드한 후보다. 실행 timeout은 기존 검증 driver 값을 그대로 쓴다. stdout/stderr/exit/생성된 state를 수집한다. 실제 사용자 state·홈·인증·원격 저장소는 사용하지 않는다.

integration harness는 `cmd.Env`를 명시적 allowlist에서 조립한다. 부모 `os.Environ()` 전체에 override만 덧붙이지 않는다. 필요한 toolchain 경로·로케일·임시 home/state만 넣고 provider/host executable 경로는 fixture로 고정한다. 기본 PATH에서 실제 `gh/glab/orca/codex/claude/omo`로 fallback하지 못하게 모두 fail-closed shim으로 차단하며, fixture가 모르는 argv는 비영 exit로 끝낸다. 인증 token·실제 remote URL·부모 daemon/socket 경로가 전달되지 않는지와 예상 밖 실행 0회를 assertion으로 검사한다. 이 격리는 테스트 harness에만 적용하고 production command env 의미는 바꾸지 않는다.

| 시나리오 | 실제 진입점 | 통과 조건 |
|---|---|---|
| 공개 표면 | `issueops --help`, CLI contract golden, MCP stdio initialize→tools/list | 이름·schema·필수 필드·응답 shape가 baseline과 동일 |
| 읽기 | `inspect --json`, `docs --json`, `doctor --static-only --json`, fixture cycle의 `issueops next/status` | baseline과 같은 판정, read-only 명령의 state 변경 0 |
| 정책 | `policy check --workspace-root <repo> --cwd <repo> --json -- git status --short` 및 fixture outside-root/deny argv | baseline verdict/exit/warnings 동일, 거부 실행 효과 0 |
| loop/worker | 기존 `validationcli/contractauditworker`, worker/loop CLI fixture 시나리오를 후보 binary에 실행 | 실제 queued/terminal/attempt 상태와 결과 일치, timeout 유한 종료 |
| 게이트 | fixture repo에서 `gates init/check/report`의 argv·EXPECT 성공/실패 사례 | 비영 exit는 EXPECT가 맞아도 실패, ledger 원문 보존 |
| 설치 | `install --dry-run --json` 및 기존 native install integration matrix | dry-run은 write/spawn 0, host matrix·rollback 결과 동일 |
| 원격 흐름 | 기존 provider fixture runner를 연결한 CLI/MCP publication/reconcile integration | authority denial/응답 모호성 시 중복 create 0, receipt/state 일치 |
| 자기 검증 | `.github/workflows/ci.yml`과 같은 격리 설치 후 `self-verify --seed=100 --target-score=95 --json` | 실제 실행 단위 하나가 최종 `ok=true` 및 기존 목표 점수 계약 충족 |

CLI를 통한 원격 시나리오는 production composition과 같은 provider adapter에 fixture executable/HTTP 서버를 붙인다. application 직접 호출만으로 CLI 시나리오를 대체하지 않는다. 전체 프로그램에서 지원하지 않는 가짜 `--provider` 플래그를 계획에 추가하지 않는다.

self-verify 격리는 **worktree가 아닌, 자체 `.git`를 가진 임시 clone/copy 저장소**에서 수행한다. `scripts/install-native.sh`가 `--git-common-dir`를 통해 main checkout을 ROOT로 고르는 동작이 있으므로 home/state 격리만으로는 부족하다. 후보 source tree(추적된 미커밋 변경과 이번 구현에서 추가한 미추적 source/test 파일 포함)를 임시 저장소에 반영한 뒤 그곳의 `bin/issueops`를 빌드하고 SHA-256을 기록한다. T01 inventory와 복사한 source 파일 집합을 대조하며 사용자 secret/runtime state는 복사하지 않는다. parent checkout과 같은 git common-dir를 공유하면 QA 시작을 거부한다.

Go test의 child process 환경에서 home/Codex/state 디렉터리도 임시 경로로 지정하고, 해당 임시 저장소의 native installer에 `--skip-build --path-mode=skip`을 전달한다. 설치 receipt와 managed command symlink의 realpath/대상 binary SHA-256을 후보와 대조한 뒤 동일 binary로 self-verify를 실행한다. 실제 user-scope install이나 MCP 프로세스 종료를 QA에 사용하지 않는다. 외부 live host credentials가 필요한 probe는 결정적 fixture QA와 구분하고, 실행하지 않은 live 지원 증거를 만들어 내지 않는다.

### 책임 분리 완료 조건

- [ ] T01의 전체 파일/함수 원장에 미배정 0, migrate 상태 0, 모든 retain에 구체적인 기술 책임과 테스트 근거가 있다.
- [ ] contract에 상태 기반 정책이 없고, adapter/CLI에 이전한 업무 규칙의 중복 구현이나 legacy Effects 우회가 없다.
- [ ] domain이 I/O·host protocol·transport를 직접 다루지 않으며 정책의 deterministic test가 성공/거부 경로를 증명한다.
- [ ] application이 실제 호출 경로와 원자적 변경을 조율하고 store/host adapter는 technical effect만 실행한다.
- [ ] 업무 use case를 수행하는 모든 CLI/MCP 진입점이 실제 application+domain을 거치며 기존 snapshot/readback/오류/저장 계약을 보존한다. help/schema/catalog 같은 technical-only 진입점은 근거 있는 retain으로 구분한다.
- [ ] lock/CAS/역색인/related rows/receipt의 concurrency·crash·rollback 테스트가 통과한다.
- [ ] 전체 battery와 격리 QA가 통과하고 critic/verifier 지적을 실제 source·test로 해소했다.

### 중단·되돌림 기준

- 각 task 완료 시 테스트 가능한 상태로 남긴다. 이전 binary와 같은 schema를 유지하므로 해당 task의 코드/wiring을 되돌리는 것으로 복구한다. state 수동 수정이나 자동 schema migration은 하지 않는다.
- 결과·오류·효과 순서가 달라지면 그 task에서 멈추고 원래 순서를 복구한다. 동작 변경 없이는 분리할 수 없는 경우 구체적인 이유와 별도 변경안을 남긴다.
- 승인된 commit이 있다면 task 단위 revert, 없으면 해당 task에서 본인이 만든 diff만 되돌린다. 타인의 변경과 unrelated cleanup은 건드리지 않는다.
- 끝까지 남는 주제는 새 기능이 아니라 업무 소유권 결함이어야 한다. 검증이 어렵다는 이유로 retain 처리하지 않는다.

## 7. 계획 검토 기록

- 계획 전 조사: architecture 전체, domain 전체, 관련 application/contract/adapter 테스트가 통과했다. 이는 기준 HEAD의 일부 검증이며 전체 프로젝트의 최종 통과 증거는 아니다.
- 조사 위임: `high-volume-exploration`과 `parallel-independent-research`, 기대 이득 `context_isolation/parallel_speed`. 두 탐색자가 IssueOps와 나머지 adapter를 분담했다. 교차 경계의 맥락 손실을 메인의 source 대조로 보완한다.
- 구현 전 critic pass는 파일/소비자/원자성/누락 범위를, verifier pass는 QA의 실패 경로와 public wiring을 독립적으로 검토한다. 결과는 아래에 기록한다.
- Critic pass: **PASS**. raw store와 application-local transaction port 분리, publication/explicit complete 분리, unknown field reject와 정의된 sidecar 보존을 source 대조 후 수정하고 재검토했다.
- Verifier pass: **OKAY**. 독립 git root와 binary digest, allowlisted test environment, 실제 application+domain 통합 및 우회 mutation, multi-GOOS inventory와 technical-only retain을 보완하고 재검토했다. 후보 복사에 이번 구현의 미추적 source/test도 포함하도록 최종 보강했다.
- 두 판정은 이 계획의 검토 결과다. 리팩토링 구현이나 전체 최종 battery가 완료됐다는 뜻이 아니다.

Repo grounding: 기준 SHA와 §1의 소스, architecture/import 규칙, CI 및 testing 문서를 확인했다.
Decision-complete plan: 기존 구조를 유지하는 capability별 이전, 원자성 보존, production 경로 전환과 이전 규칙 제거를 한 task에 묶는다.
Assumptions/defaults: 전체 프로젝트 범위, 기존 외부 계약과 운영 동작 유지, 순차 구현, 새 framework와 state migration 없음.
Unresolved questions: 계획 작성을 막는 질문은 없다. 호환성 변경 요청이 추가되면 별도 설계 변경으로 처리한다.
Acceptance criteria: 정책 소유권 원장 미배정·이전 미완료 0, 기존 계약 보존, 전체 CI 및 시나리오 증거, 두 독립 리뷰의 지적 해소.
