# Hexagonal core, ports, and adapters

> Family index: [`../ARCHITECTURE.md`](../ARCHITECTURE.md). This module owns the
> dependency direction, component boundaries, and the core/port/adapter
> structure. Host-specific integration detail lives in
> [`host-integration.md`](host-integration.md); runtime, state, and process
> topology live in [`runtime.md`](runtime.md).

## Core decision: external harness core, not plugin-only

| 선택지 | 장점 | 단점 | 판단 |
|--------|------|------|------|
| Codex plugin/skill 중심 | Codex 경험에 깊게 통합 가능, 설치 UX가 좋음 | Claude Code/Omo와 공유가 어렵고, plugin API 변화에 core가 종속됨 | 단독 core로 부적절 |
| Claude Code command/hook 중심 | Claude 사용성이 좋고 MCP와 맞음 | Codex/Omo에서 같은 동작을 재사용하기 어렵고, hook에 정책이 흩어짐 | 단독 core로 부적절 |
| 외부 CLI/MCP/worker 중심 | 세 host에서 같은 binary와 schema를 호출, 테스트 가능, 상태 관리 일관 | 초기 설치/IPC/보안 설계 필요 | **채택** |
| Hybrid | 외부 core + host별 얇은 래퍼 | adapter 관리 비용이 있음 | **최종 구조** |

결론: **Go로 작성한 외부 하네스 코어를 만들고, Codex·Claude Code·Omo 설정은 core를 호출하는 얇은 adapter로 둔다.**

## Target architecture

```mermaid
flowchart LR
    Codex["Codex<br/>AGENTS.md · native skills · MCP config"] --> MCP["issueops mcp<br/>in-process stdio server"]
    Claude["Claude Code<br/>CLAUDE.md · skills · hooks · MCP config"] --> MCP
    Omo["Omo native<br/>AGENTS.md · skills · MCP · extension"] --> MCP
    Human["Human shell"] --> CLI["CLI: issueops"]
    Hook["SessionStart context hook"] --> CLI

    MCP --> Core["contract · domain · application<br/>policy · workspace · docs · state"]
    CLI --> Core
    Core --> Ports["ports/interfaces"]
    Ports --> FS["fs/git/wiki adapter"]
    Ports --> Proc["process runner adapter"]
    Ports --> State["state/log adapter"]
    Ports --> Config["config adapter"]

    Core -. future .-> Worker["local job worker<br/>queue · watch · long tasks"]
    Worker --> Core
```

Mermaid는 보조 자료다. 규칙·경계·검증 명령은 아래 텍스트를 우선한다.

### Application / domain / port / host adapter 구조

설치와 host 통합은 SOLID 경계로 나눈다.

- `internal/adapter/install.InstallNative`: 현재 host-neutral 설치 engine. 공통 입력과 skill 목록을 정규화하고 `port.HostInstaller`만 호출한다. 검증된 설치 계약을 유지하면서 신규 use case는 `internal/application/<capability>` vertical을 우선한다.
- `internal/port`: `NativeInstallRequest`, `NativeInstallResult`, `HostInstaller` interface를 정의한다. port는 contract DTO 외의 concrete 내부 구현을 모른다.
- `internal/adapter/codex`: Codex 구현체. user skill symlink, `~/.codex/config.toml` MCP 등록, `~/.codex/hooks.json` lifecycle hook을 기본 갱신한다.
- `internal/adapter/claude`: Claude Code 구현체. user skill symlink, user-scope MCP 등록 경로, `~/.claude/settings.json`의 `SessionStart` context hook만 기본 갱신한다.
- `internal/adapter/omo`: Omo native 구현체. user skill/MCP/extension 설정을 갱신하고 대상 repo에는 명시적 opt-in 없이 파일을 쓰지 않는다.
- `cmd/issueops/issueopsapp`: concrete adapter를 조립하는 유일한 composition root다.
- repo-local `.claude/skills`, `.claude/settings.json`, `.mcp.json`은 적용 대상 repo에 커밋될 수 있으므로 `--project-local` 같은 명시적 opt-in에서만 생성한다.

이 구조에서 새 host를 추가할 때는 domain/application 정책을 복제하지 않고 `port.HostInstaller` 구현체와 composition-root wiring만 추가하는 것이 원칙이다.

## Current package boundaries

| 경로 | 책임 | 금지/주의 |
|------|------|----------|
| `cmd/issueops` | composition root, CLI flag/출력, MCP stdio·JSON-RPC, daemon lifecycle, self-verify/self-augment orchestration | host별 정책과 domain 판정 복제 금지 |
| `internal/contract` | transport/state가 공유하는 versioned DTO와 error vocabulary | 판정 로직과 I/O 금지 |
| `internal/domain` | 순수 규칙, reducer, classifier | adapter/cmd, filesystem/process/DB I/O 금지. clock은 기본 주입하며 `auditid` timestamp ID 생성은 현재 명시적 예외 |
| `internal/application` | contract/domain/port를 조합하는 capability use case | concrete adapter와 transport 의존 금지 |
| `internal/port` | 외부 capability interface와 error contract | contract 외 concrete 내부 package 의존 금지 |
| `internal/adapter/inbound` | capability request를 application 호출로 변환 | outbound adapter 직접 의존 금지 |
| `internal/contract/cli` | 정적 command descriptor·usage 원문·actor flag 범례 | 목록 조합·도움말 렌더링·dispatch 금지 |
| `internal/adapter/inbound/catalog/cli` | 명령 목록 조합과 root·lifecycle·child 도움말 렌더링 | root가 도움말을 CLI invocation별 dependency로 전달 |
| `internal/contract/mcp` | 정적 tool/resource descriptor, schema와 dispatch group 타입 | 목록 조합·요청 처리·I/O 금지 |
| `internal/adapter/inbound/catalog/mcp` | 광고 목록·alias dispatch·resource map 조합 | root에서 완성한 catalog를 소비자에게 주입 |
| `internal/adapter/outbound` | state, SQL, webfetch 등 capability 외부 I/O 구현 | transport 정책과 domain 판정 복제 금지 |
| `internal/domain/toolconformance` | host-neutral schema projection·판정과 gate decision | host argv, credentials, production dispatch 의존 금지 |
| `internal/adapter/toolconformance` | fixture I/O와 behavioral replay 실행 | domain 판정 복제 금지 |
| `internal/adapter/failurecause` | typed causal evidence 수집과 adapter projection | stderr 문자열만으로 model blame 금지 |
| `internal/domain/operationalhealth` | normalized snapshot과 주입된 clock/preserve set을 판정하는 pure classifier | filesystem/process/SQLite I/O, cleanup mutation, host별 정책 금지 |
| `internal/adapter/hostprobe` | Codex/Claude/Omo의 격리된 live probe 실행과 증거 정규화 | 사용자 host 설정·credential DB 수정 금지 |
| `internal/adapter/orca` | 설치된 Orca CLI의 bounded argv/timeout/envelope projection | IssueOps 상태·복구 정책 복제, generic driver registry, 설치 대행 금지 |
| `internal/adapter/operationalhealth` | Git, 전체 IssueOps record/binding, 선택적 Orca inventory를 read-only snapshot으로 수집 | health 판정 복제, state 생성, cleanup mutation 금지 |
| `internal/adapter/codex` | Codex user skill symlink와 user MCP config 설치 | 대상 repo 파일 쓰기 금지 |
| `internal/adapter/claude` | Claude user skill symlink와 user-scope MCP 설정 | 기본 설치에서 `.claude/skills`, `.claude/settings.json`, `.mcp.json` 같은 repo-local 파일 쓰기 금지 |
| `internal/adapter/omo` | Omo user skill/MCP/extension 설정 설치 | 대상 repo 파일 쓰기와 host 공통 정책 복제 금지 |
| `internal/adapter/worker` | local IPC, job lifecycle, daemon state | shell policy 우회 금지 |
| `internal/domain/gates` | unlazy 호환 게이트 ledger의 순수 파서·판정·직렬화(원문 보존) | filesystem/process I/O, policy 실행 금지 |
| `internal/adapter/gates` | 게이트 파일 I/O와 policy 게이트 실행(argv 토큰화→policy→timeout/audit) | raw shell 실행, 크로스 케퍼빌리티 adapter 직접 import 금지(실행기는 composition root 주입) |
| `internal/adapter/issueops/gatesgate` | IssueOps PR readiness에 게이트 ledger 합성(`gates_incomplete:<file>`) | gates adapter 직접 import 금지(함수 변수 주입, loopgate와 동일 구조) |
| `internal/contract/channel` | 세션 간 메시지 채널 DTO(schema v1) | 판정 로직과 I/O 금지 |
| `internal/adapter/channel` | 채널 메시지 append/읽기/대기 원시(issueops state 위) | 크로스 케퍼빌리티 adapter import 금지, 인증 경계 아님 |
| `configs/codex` | Codex plugin/skill 템플릿 | core 로직 금지 |
| `skills` | Codex/Claude/Omo 공용 skill source of truth | host별 복사본을 만들어 drift 유발 금지 |
| `.mcp.json` | 이 하네스 repo의 dogfood/project-local MCP server 설정 | 기본 설치는 user-scope MCP를 사용하므로 대상 repo에 복사 금지 |
| `scripts/install-native.sh` | native skill/MCP 설치 및 갱신 | 사용자 홈 skill symlink만 기본 생성. repo-local 파일은 `--project-local` 명시 때만 생성 |

### Cross-host tool contract boundary

`cmd/issueops/contractcli.Conformance`는 명령 호출마다 별도 인스턴스로 조립한다. root가 catalog, fixture 로더, replay, live runner와 capture probe를 전달하며 패키지 전역 setter는 사용하지 않는다. baseline과 replay는 해당 인스턴스의 catalog·root를 사용하고 live report도 그 root 아래에 저장한다.

`contract conformance`는 production MCP 의미를 바꾸기 전에 지원되는 live host가 실제로 생성한 raw arguments를 측정한다. 기본 live host는 계속 Codex/Claude이며, Omo native runner는 `--hosts omo`와 explicit `--model omo=provider/model`을 함께 지정한 opt-in에서만 episode를 시작한다. Omo lifecycle module의 canonical 생성은 `internal/adapter/hostprotocol`이 소유한다. composition root가 같은 builder를 installer, activation verifier와 native runner에 주입하며 adapter끼리 직접 의존하지 않는다. Production preflight는 exact harness path로 생성한 module과 전달된 source의 byte identity만 증명하고, 실제 async JavaScript 실행은 test-only goja proof가 소유한다. 어느 쪽도 live E2E를 뜻하지 않는다. live report는 H0의 `supported|unsupported|unavailable|not-run` 상태를 재사용하며 completed live episode가 없으면 `supported`를 기록하지 않는다. `internal/adapter/mcp`의 capture-only probe는 episode마다 한 tool만 광고하고 production catalog handler를 등록하거나 호출하지 않는다. 임시 config/plugin과 인증 격리는 `internal/adapter/hostprobe`가 소유하고 schema 의미와 판정은 `internal/contract/toolconformance`와 `internal/domain/toolconformance`가 소유한다.

Deterministic baseline과 live evidence는 advertised schema validity와 closed canonical-intent validity를 별도로 기록한다. 재현 gate가 동일 diagnostic signature를 두 번 이상 확인한 경우에만 production advertised schema와 SDK/legacy call entry를 같은 canonical validator로 원자적으로 강화한다. 이 gate가 열리지 않은 상태에서는 benchmark, failure-cause axis, self-verify coverage만 유지하고 production argument semantics는 변경하지 않는다.

### Public state and MCP resource boundary

공개 `state write|read|list|prune|doctor|maintain` 명령은 root가 호출별로 조립한 state application을 사용한다. MCP는 `MCPDependencies.State`로 같은 연산을 받으며 직접 호출과 SDK handler가 모두 해당 인스턴스를 사용한다. 저장소 경로와 worker override는 조립 시점에 고정한다. maintenance 대상 탐색과 SQL·파일 관측은 outbound adapter, 유지보수 실행 순서는 application, record 검증과 doctor 판정은 domain이 담당한다.

MCP resource는 `MCPDependencies.Resources`로 저장소 조회·harness 파일 root·문서 조회·정책 요약을 받는다. SDK resource callback과 서버 version도 해당 설정을 사용한다. `project-docs` resource는 기존처럼 현재 작업 디렉터리(`.`)를 대상으로 하며 harness 설치 root와 혼동하지 않는다. 없는 저장소의 read·doctor·maintain은 저장소를 만들지 않고, list는 기존처럼 저장소를 연다. state transport의 전역 setter와 resource 정책 요약 전역 변수는 제거했으며, 다른 capability의 전역 wiring 전환은 T20 후속 범위다.

### Project document boundary

문서 route·read·revise·append는 root가 조립한 `internal/application/projectdocs.Service`를 호출한다. application은 domain의 경로 허용 목록·수정 SHA·추가 문서 규칙을 적용하고 파일 효과를 조율하며, adapter는 읽기·쓰기·문서 렌더링만 담당한다. CLI와 `MCPDependencies.ProjectDocs`는 각각 이 인스턴스를 받고 MCP 직접 호출과 SDK가 같은 service를 사용한다. 기본 대상과 상대 경로 기준 디렉터리는 조립 시 고정하되 명시한 `repo`는 그대로 존중한다. MCP의 빈 `repo`는 기존 `CLAUDE_PROJECT_DIR → PWD → 현재 디렉터리` 우선순위로 조립 시 선택한다. CLI의 기본 `.`과 project-docs resource는 조립 당시 작업 디렉터리를 뜻한다. 기존 문서 실행 facade와 전역 경로·문서 콜백은 제거했으며, inspect 등 다른 capability의 전역 wiring은 후속 범위다.

bootstrap은 별도 `application/projectbootstrap.Service`가 경로 정규화와 문서 생성 순서를 담당한다. root가 문서 렌더링·파일 처리와 `application/lifecycle.Service`를 조립해 CLI와 MCP 인스턴스에 전달한다. lifecycle profile adapter는 고정된 state 경로와 명시적인 경로·Git 관측 함수를 받으며, 기존 전역 setter와 bootstrap/profile 실행 facade는 제거했다. MCP bootstrap은 계속 dry-run 전용이고 CLI는 쓰기·sync·기존 문서 보존 규칙을 domain에 위임한다. doctor도 root에서 같은 lifecycle application을 조립하며, 진단 인스턴스가 사용할 state 경로를 조립 시 고정한다.

### Doctor diagnosis boundary

doctor의 파일·프로세스·HTTP 관측은 adapter가 수행하고, 문서 누락·저장소 내부 runtime 상태·loop 미완료·pipe 용량·MCP 연결 및 FD 압박·native hook 누락·binary 변경 여부는 `internal/domain/doctor`가 판정한다. lifecycle 진단과 전체 health 집계, operational 진단과 중복되는 state artifact의 분류도 같은 domain에 둔다. 관측값에는 읽기 실패와 미관측 상태를 유지하며 판정 함수는 filesystem이나 process를 호출하지 않는다.

`application/doctor.Service`는 기존 순서로 관측값을 모아 domain 결과를 합치고 출력 순서를 정한다. `--static-only`는 pipe·MCP live 관측을 호출하지 않는다. 실제 CLI의 진단 결과, 오류·종료 코드와 저장 파일 변화를 이전 binary와 비교한다. root의 doctor factory는 state·lifecycle·loop 조회를 인스턴스마다 조립한다. CLI의 `basiccli.Doctor`는 application과 경로 정규화, home·harness 경로·version, live 관측 함수를 명시적으로 받는다. adapter의 실행 facade·DTO 재노출·전역 setter와 CLI의 doctor setter는 제거했으며 gateway 관측도 별도 probe 인스턴스를 사용한다. status는 같은 doctor application을 명시적으로 받지만 status의 집계 로직·worker 조회 등 나머지 경계는 후속 범위다.

### Loop runtime boundary

loop의 생성·시도 기록·종료·status는 root가 조립한 `application/looprun.Service`를 호출한다. CLI와 MCP 직접 호출·SDK는 각 인스턴스에 고정한 저장소 경로와 작업 디렉터리를 사용한다. adapter의 `Store`는 SQL 읽기·쓰기와 기존 span 잠금만 수행하며 전역 저장소 setter와 lifecycle 실행 facade는 제거했다.

PR readiness와 doctor의 loop 조회는 `application/looprun.Reader`가 기존 record만 읽고, `domain/looprun.EvaluateRepoGate`가 같은 repo의 미완료 여부·집계와 읽기 실패 시 차단을 판정한다. 빈 저장소를 조회해도 생성하거나 권한을 고치지 않는다. doctor는 조립 시 이 reader를 고정하며, IssueOps readiness의 기존 조립 진입점은 호출 시 reader를 만든다. IssueOps readiness 소비자 전체의 인스턴스 전환은 후속 범위다.

### Self-verification history boundary

`self-verify history|compare`와 MCP의 같은 도구는 `internal/application/selfaugment.HistoryService`를 호출한다. root는 저장소 경로를 인스턴스에 고정하고 state application과 SQL adapter를 조립한다. CLI에는 History·Compare 함수를, MCP에는 `MCPDependencies.SelfHistory`를 전달한다. MCP 직접 호출과 SDK 서버 모두 해당 인스턴스를 사용하며 history adapter의 전역 저장소 setter나 parent façade를 거치지 않는다. 정렬·보존·비교 판정은 domain, 조회·삭제 순서는 application, flag·출력·프로토콜 오류 변환은 transport가 소유한다.

계획·검증 요약 저장과 기준선 승격도 root가 저장소별로 조립한 `SavePlan`, `SaveSummary`, `PromoteBaseline` application을 호출한다. CLI는 해당 함수를 직접 받고 MCP는 `MCPDependencies.SelfState`로 받는다. 저장 형식과 승격 가능 여부는 domain, 저장·읽기 순서는 application, SQL 접근은 adapter가 담당한다. 기존 `selfworkflow/stateio`의 production 래퍼와 전역 저장소 setter는 제거했다. 계획 생성·후보 내보내기·lesson 저장은 `MCPDependencies.SelfPlanning`과 같은 root 조립 함수를 CLI에서도 사용한다. 계획 root와 상태 저장소는 인스턴스마다 고정하며, 이전 `augmentplan`·`candidateexport`의 production 래퍼와 lesson 저장 전역 setter를 제거했다.

`quality inspect`는 호출별 `qualitycli.Deps`로 collection과 기준선 저장소를 받는다. 수집·기준선 조회/저장 순서는 `internal/application/quality`, schema·ratio 검증과 상태 반영은 domain, 경로 정규화·소스 스캔·coverage 실행은 outbound adapter가 담당한다. 계획 후보를 quality 후보로 변환하는 책임도 application에 두며 서로 다른 domain의 DTO를 직접 참조하지 않는다. CLI는 flag·출력·종료 오류 변환을 담당하고 패키지 전역 root나 collector를 교체하지 않는다.

self-verify 실행은 root가 저장소 경로와 step adapter를 고정해 `application/selfverify.ExecuteLoop`에 연결한다. CLI와 MCP는 같은 application request를 사용하며 MCP는 `MCPDependencies.SelfVerify`로 서버별 실행기를 받는다. 전역 실행 콜백과 production `verifyloop` 중계 패키지는 제거했다. gate 오류는 application의 동일한 error identity로 판별하고, 실패한 검증 결과를 요청에 따라 저장하는 기존 계약을 유지한다.

`cmd/issueops/selfworkflow` 부모 패키지와 `model`, `augmentcatalog`, `summary`, `steps`, `rerun`, `loopresult`는 production 의존 그래프에서 제거했다. 계약 필드 목록은 `domain/selfverify.ContractValue`, 후보 선택은 `domain/selfaugment.SelectedCandidateID`를 직접 사용한다. 기존 테스트는 각 패키지의 테스트 전용 구성과 정식 contract/domain/application 참조로 유지하며, runtime 호환 별칭이나 전역 catalog setter는 남기지 않는다. CLI의 flag·환경변수 우선순위와 progress·LLM prompt 출력은 해당 inbound adapter에 둔다.

### Operational-health boundary

기존 top-level `doctor`가 cross-system operational health의 유일한 공개 표면이다. `internal/adapter/operationalhealth`가 read-only inventory를 정규화하고, `internal/domain/operationalhealth`가 deterministic finding을 만든다. IssueOps stale scan은 같은 cycle-authority 판정만 재사용하되 기존 strong-signal release policy와 locked re-probe를 유지한다. Stability audit는 ownership/residue 규칙을 다시 구현하지 않고 방금 빌드한 binary의 `doctor` 결과를 gate로 소비한다.

### Dependency fitness ratchet

`internal/architecture`는 production import graph의 test-only fitness boundary다. `go list -json ./...`의 direct `Imports`만 정렬된 `importer -> imported` edge로 수집하며, test import와 transitive dependency는 graph에 포함하지 않는다.

- `internal/domain|application/... -> internal/adapter/...|cmd/...`, `internal/adapter/... -> cmd/...`, `internal/port -> contract 외 internal/...`는 baseline 없이 즉시 실패한다. 과거 `core` 규칙도 재도입 방지용으로 유지한다.
- legacy adapter edge는 0이다. `internal/adapter/*`는 composition root(`cmd/issueops/issueopsapp`)에서 조립한다. 같은 capability의 하위 package 사이 edge는 구현 정리로 허용한다. capability는 `internal/adapter/` 다음 경로 요소이며, `outbound`/`inbound`이면 그 다음 요소까지 포함한다. `isSharedStorageEngineEdge`는 공유 저장 엔진 `outbound/sqlstore`를 outbound 어댑터와 `internal/adapter/issueops`에만, `outbound/issueopsrecord`를 `outbound/issueops*`에만 허용한다.
- `isProcessLifetimeEdge`는 `outbound/processlease`를 세 명령 실행 패키지(`internal/adapter/issueops`, `internal/adapter/orca`, `internal/adapter/provider/providerutil`)에만 허용한다. 이 도구는 OS 잠금과 자식 프로세스의 파일 디스크립터 상속만 담당하며 프로젝트 내부 패키지를 참조하지 않는다. 잠금 파일은 실행 권한이나 작업 상태를 저장하지 않는다. domain/application/port/contract/inbound의 직접 참조는 금지한다. `TestProcessLifetimePrimitiveHasNarrowConsumers`와 `TestProductionGraphHasNoLegacyAdapterEdges`가 허용 범위를 검사한다.
- baseline을 줄이는 변경은 의도된 architecture 개선으로 같은 review에서만 허용한다. production package 이동이나 runtime wiring은 이 ratchet의 범위가 아니다.
- Issue #499의 maintenance baseline은 기존 package/import graph를 바꾸지 않고 책임별 sibling file로 분해한다. 당시 지목된 7개 비테스트 entry file과 새 sibling은 모두 900줄 미만이며, 이후 예외는 같은 review에서 근거와 검증을 남겨야 한다.

## 현재 hardening 추가 사항

- `internal/port`는 Orca probe/run/worktree/terminal/task/dispatch 역할 interface와 공용 `InstallPlan`을 소유한다. 기존 `OrcaClient` aggregate와 `omo.InstallPlan` alias는 내부 소비자의 type/method-set 호환을 위한 명시적 예외다.
- gates legacy ledger 이름은 persisted schema v1 migration 전까지 유지한다. Orca task payload에는 version 필드가 없으므로 legacy UTC timestamp는 지원 대상 Orca CLI 전부의 `completed_at` readback이 RFC3339Nano임을 확인하고 release contract에서 legacy layout이 제거된 때에만 소스 상수를 올려 닫는다. 시간 경과만으로 호환 경로를 제거하지 않는다.
- Orca/operational-health fan-out은 bounded `errgroup`을 쓰되 indexed error와 partial finding을 보존한다. `quality inspect`의 5-collector fan-out은 모든 read-only 결과를 오류와 함께 끝까지 회수해야 하므로 조기 취소하지 않는 명시적 예외다. 각 collector는 공유 쓰기 없이 버퍼 1 채널에 정확히 한 번 전송해 수신 순서와 무관하게 종료하며, 한 collector 오류도 나머지 진단을 버리지 않는다. channel wait는 append-only immutable record ID를 한 호출 안에서만 기억하며, cross-process writer 때문에 process-global cache나 in-process notification을 authority로 삼지 않는다.
- `internal/contract/cli`가 command descriptor와 canonical usage 원문을 소유한다. `internal/adapter/inbound/catalog/cli`는 명령 목록과 도움말을 조합하고 root가 `issueopscli.Dependencies`에 lifecycle·child 도움말을 전달한다. `internal/domain/cli`에는 명령 허용 여부와 usage key를 해석하는 순수 규칙만 둔다. `cmd/issueops/*cli`는 flag/출력/dispatch를 담당하며 `contractcli`는 root가 넘긴 CLI/MCP 목록으로 호환성 계약을 만든다.
- `internal/contract/mcp`가 정적 schema와 descriptor를, `internal/adapter/inbound/catalog/mcp`가 목록 조합을 소유한다. root는 `MCPDependencies.Catalog`로 서버마다 완성한 목록을 전달한다. `cmd/issueops/mcpcli`는 주입된 목록으로 광고·입력 검증·dispatch를 수행하며 catalog builder를 직접 import하지 않는다. `internal/adapter/mcp`는 capture-only conformance probe로 제한한다.
- `issueops contract schema|check`는 CLI/MCP command list, MCP tool name, required response field를 검증하는 DTO compatibility 표면이다.
- `issueops policy audit`는 redacted command-policy decision을 append-only JSONL로 기록하며 command를 실행하지 않는다.
- `issueops worker`는 lifecycle job record(`enqueue/status/list/cancel/cleanup-stuck`)와 policy-gated `run --read-only`(MCP `worker_run_read_only`)를 제공한다. 장기 상주 job daemon은 없다.
