# Research: ARTEX 오케스트레이션·시각화와 issueops 적용성

## TL;DR

**결론**: ARTEX(`jiwoochris/artex-global`)의 설계 중 issueops에 그대로 옮길 수 있는 것은 적습니다. ARTEX는 엔진이 LLM 루프를 직접 실행하고, issueops는 호스트가 실행하는 작업의 상태와 게이트만 소유하기 때문입니다. 적용할 가치가 있는 것은 세 묶음입니다.

1. **시각화**: 사이클 record를 읽기 전용으로 투영해 보드, 타임라인, 증거 그래프로 보여주는 것입니다. ARTEX의 서버·SSE·React 구조는 가져오지 않습니다.
2. **에이전트 운영 규칙**: 계획자만 작업을 만들고, 실행자는 과제 하나만 수행하고, 완료는 근거에 묶인 증명으로만 판정하는 규칙입니다. 팀 작업의 위임 계약에 적용할 수 있습니다.
3. **운영 교훈**: 정체 감지, 중단 원인 보존, 백그라운드 기능의 실행 여부 신호입니다. 앞의 둘은 기존 후보 O2와 겹칩니다.

**Confidence**: Medium. ARTEX 쪽 사실은 소스를 읽어 확인했고 실행하지 않았습니다. issueops 쪽 사실은 HEAD `7bec87a74`의 코드·문서·CLI 출력으로 확인했습니다. 적용 효과는 측정하지 않았습니다.
**Sources**: ARTEX 저장소와 그 에이전트 프레임워크 `Autumn-27/norma` v0.4.4, issueops 저장소. 보안 기법과 대상 시스템에 대한 동작은 조사 범위에서 뺐습니다.

## Method

- **기준 시점**: 2026-11-11. issueops HEAD `7bec87a74`. ARTEX는 `main`, norma는 태그 `v0.4.4`.
- **읽은 방식**: ARTEX와 norma를 얕게 클론해 읽기만 했습니다. 빌드·설치·실행은 하지 않았습니다.
- **병렬 조사 9개**(서로 다른 영역, 겹치지 않게 배정)
  1. 스케줄링 엔진: `server/engine.go`, `engine_timeout.go`
  2. 프런트엔드 시각화 개요: `web/`
  3. 에이전트 런타임: `agent/`, norma `agentcore`
  4. 상태 모델: `db/schema.sql`, `db/exploration.go`
  5. norma 내부: `harness`, `hook`, `coordinator`, `subagent`
  6. 계획자 제어부: `agent/planner.go`, `agent/tools.go`의 제어 도구
  7. 작업 상위 계층과 사람 개입: `server/orchestration.go`, `scheduler.go`, `task_control.go`
  8. 시각화 구현 상세: `sessions-tab.tsx`, `exploration-graph.tsx`, `server/broadcast.go`
  9. 변경 이력·테스트·설정값: `CHANGELOG.zh.md`, 테스트 파일, 설정 상수
- **issueops 대조**: `.issueops/ARCHITECTURE.md`, `AGENT_WORKFLOW.md`, `architecture/issueops.md`, `architecture/runtime.md`, `SUB_AGENT_PATTERNS.md`, `research/2026-11-03-optimization-quality-candidates.md`, `internal/contract/issueops`의 record 타입, `issueops list`·`status`·`model show` 출력.
- **한계**: ARTEX 동작은 모두 코드를 읽고 추론했습니다. 아래에서 [INFERENCE]는 재현하지 않은 추론입니다.

```text
Source fan-out: 9 non-overlapping source-reading passes over ARTEX and norma; one comparison pass over issueops
Source index: ARTEX and norma source (read-only clone), issueops repo docs/code/CLI output, all read 2026-11-11
Claim verification: issueops-side claims checked against HEAD files and CLI output; ARTEX runtime claims unverified by execution
Access boundary: no auth/paywall; one sub-agent pass stopped by host safeguards and was not retried
```

## Findings

### 1. ARTEX의 구조

- 역할이 세 가지입니다. 사람과 대화하는 MainAgent, 작업 단위(intent)를 만드는 유일한 주체인 Planner, intent 하나를 받아 실행하고 끝내는 Worker입니다.
- 상태는 PostgreSQL 그래프입니다. 노드 종류는 goal·intent·fact·finding·hint·digest이고, edge는 `derived_from`(근거 → intent), `yields`(intent → 결과), `proves`(근거 → goal), `covers`(요약 → 접힌 노드)입니다.
- Worker는 조건부 UPDATE 한 문장으로 intent를 선점합니다. lease와 heartbeat는 없습니다.
- Planner는 그래프가 바뀌면 깨어납니다. 깨울 때 무엇이 바뀌었는지를 이벤트로 함께 넘깁니다.

### 2. 에이전트 활용에서 확인한 장점

|#|규칙|ARTEX 근거|
|---|---|---|
|P1|작업은 계획자만 만든다. 실행자는 범위를 넓히지 않고, 범위 밖에서 발견한 단서는 결과 보고에 적어 계획자에게 넘긴다|`agent/worker.go`의 `workerDefaultTmpl`, Worker 도구 집합에 `add_intent` 없음|
|P2|실행자의 과제는 system 프롬프트에 둔다. 컨텍스트가 압축돼도 과제가 사라지지 않는다|`agent/worker.go`의 `renderIntentTask`|
|P3|스냅샷의 모든 목록은 항목 수 제한과 전체 개수를 짝으로 준다. 목록에 없는 것을 "시도한 적 없음"으로 읽지 않게 한다|`agent/tools.go`의 `graphOverviewData`|
|P4|새 작업은 기존 결과를 부모로 가져야 한다. 생성 경로에서 코드가 검사한다|`agent/tools.go`의 `add_intent` 처리|
|P5|완료는 목표마다 근거 노드를 지정한 증명으로만 판정한다. 한 번에 완료를 선언하는 도구는 기본으로 연결하지 않는다|`prove_goal`, `agent/toolcatalog.go`의 `defaultUnbound`|
|P6|작업이 어떻게 멈췄는지를 상태로 구분한다. 정상 종료, 한도 소진, 오류, 강제 종료마다 계획자의 다음 행동이 다르다|`server/engine.go`의 종료 분류, `plannerDefaultTmpl`|
|P7|한도에 걸리면 바로 끊지 않고 마무리 턴을 돌린다. 실행 도구를 숨기고 지금까지 찾은 것을 기록하게 한다|norma `harness`의 `Settlement`, `agent/wrapup.go`|
|P8|순서가 있는 작업은 한 번에 한 단계만 보낸다. 서로 독립인 작업만 병렬로 보내고, 한 번에 4개까지만 보낸다|`plannerDefaultTmpl`(프롬프트 문장)|
|P9|부정 결과에는 근거와 신뢰도(직접 관측, 추론)를 붙인다. 추론뿐인 부정 결과는 한 번 재확인한다|`record_fact` 스키마, `plannerDefaultTmpl`|
|P10|계획자를 깨울 때 이유를 함께 준다. 몰린 신호는 묶고, 신호가 없어도 주기적으로 깨운다|`server/manager.go`의 `TriggerEvent`, `server/engine.go`의 `plannerLoop`|
|P11|취소에는 이름 붙은 원인을 단다. 원인이 없는 취소는 진단 코드로 드러난다|`agent/cancelcause.go`|
|P12|역할마다 모델을 따로 정하고, 실행 중에도 바꿀 수 있다|`server/task_llm.go`|

### 3. ARTEX의 약점

- P1·P5·P8·P9 같은 규칙 대부분이 프롬프트 문장으로만 있습니다. 코드는 지켰는지 확인하지 않습니다. 예를 들어 한 번에 4개라는 제한을 검사하는 코드가 없습니다.
- 실행 중인 intent에 lease가 없어서 서버 프로세스가 하나라는 가정에 의존합니다.
- 노드와 edge를 한 트랜잭션에 쓰지 않고 쓰기 오류를 무시합니다.
- 계획자를 깨우는 신호, 계획자의 할 일 목록, 방향 전환 메시지가 메모리에만 있습니다.
- "탐색했지만 기록한 것이 없음"을 구분하는 값(`WriteCounts`)이 로그에만 쓰입니다.
- 이벤트 전송 경로에 유실 가능성이 두 가지 있습니다. 저장과 발행을 한 잠금으로 묶지 않아 순서가 뒤집힐 수 있고, 구독자 버퍼가 넘치면 연결을 유지한 채 이벤트를 버립니다. [INFERENCE]

### 4. ARTEX의 시각화

- 작업 화면의 기본 탭은 그래프가 아니라 세션 목록입니다. 왼쪽에 역할별 세션, 오른쪽에 고른 세션의 단계 기록이 나옵니다. 실행자 세션은 작업 단위 하나에 하나씩 생깁니다.
- 실시간 기록은 순번이 증가하는 이벤트 로그 하나에서 나옵니다. 기록 조회가 돌려준 순번부터 SSE를 열고, 클라이언트는 순번 기준으로 합칩니다.
- 그래프는 React Flow에 직접 만든 계층형 레이아웃을 씁니다. 노드 종류는 아이콘과 색, 상태는 배지로 표시합니다. 접힌 영역은 클라이언트에서 한 노드로 합칩니다.
- 설계 규칙으로 가져올 만한 것은 네 가지입니다.
  - 상태와 색의 대응표를 한 곳에 둔다.
  - 목록에는 요약만 싣고 상세는 펼칠 때 불러온다.
  - 화면 요소마다 안정된 ID를 붙이고, 그 ID로 다른 화면과 연결한다.
  - 잘린 목록 옆에 전체 개수를 붙인다.
- ARTEX에 없는 것: 그래프 노드에서 실행 기록으로 가는 연결, 시간 축 화면(실행자별 swimlane), URL로 지정할 수 있는 화면 상태.

### 5. ARTEX 변경 이력의 교훈

오케스트레이션 관련 이력은 `CHANGELOG.zh.md`의 0.3.2~0.3.14에 있습니다. 수정 대부분이 다섯 가지 문제로 모입니다.

|반복된 문제|고친 방법|
|---|---|
|결과 없이 끝나는 실행|외부 타임아웃 대신 실행 안에서 마무리 턴을 돌림(0.3.8). 출력 없는 턴에는 "계속하라"는 입력을 넣음(0.3.9)|
|계획자 정체와 조기 종료|"진행 중인 것이 없음"을 코드로 감지해 알림(0.3.8). 완료 선언 전에 측정값을 확인하게 함(0.3.5)|
|라운드마다 커지는 컨텍스트|조회 도구에 페이지 나누기와 목록 상한을 넣음(0.3.6~0.3.14)|
|일시정지·취소·삭제·완료 사이의 경쟁|쓰기가 끝난 것을 확인한 뒤 정리하고, 삭제 중인 작업에 새 작업이 들어오지 못하게 막음(0.3.2, 0.3.3)|
|모델·게이트웨이의 특이 동작|도구 이름 별칭, 비스트리밍 경로, 파싱 전 정리|

눈여겨볼 사례가 하나 있습니다. 그래프 압축 기능은 0.3.8에 들어갔지만 연결이 빠져 0.3.12까지 한 번도 실행되지 않았습니다. 실행 횟수를 드러내는 신호가 없었습니다.

### 6. issueops 대조

**issueops가 이미 더 강한 것**

|ARTEX의 약점|issueops의 현재 상태|
|---|---|
|lease 없음|generation, claim token, 프로세스 영수증으로 실행을 점유합니다([issueops.md](../architecture/issueops.md))|
|쓰기 오류 무시|원격 쓰기 전에 intent를 저장하고, 결과가 불분명하면 `reconcile`로 확정합니다|
|완료 판정이 문장에 의존|승인을 문서 sha256과 변경 집합 fingerprint에 묶고, 바뀌면 stale로 거부합니다|
|상태 조합 누락을 사후에 발견|`next` 분류기에 상태 matrix 테스트가 있습니다(커밋 `a8cf22294`)|
|반복 위반의 승격 절차 없음|재발한 caution을 테스트·lint·게이트로 승격합니다([AGENT_WORKFLOW.md](../AGENT_WORKFLOW.md))|

**core에 넣을 자리가 없는 것**

- 계획자 루프, 실행자 풀, 신호 묶기, 마무리 턴: issueops는 LLM 루프를 소유하지 않습니다. hook은 context-only입니다(ADR 2026-11-03).
- 실시간 세션 기록 화면: 세션 기록은 호스트와 Herdr가 소유합니다. `AGENTS.md` §10은 외부 도구의 기능을 core에 복제하지 않는다고 정합니다.
- 상주 서버와 SSE: `AGENTS.md` §7은 장기 실행 worker를 필요성이 확인된 뒤 도입한다고 정합니다.

**적용할 수 있는 것**

- P1·P5·P6·P8·P9는 위임 계약과 팀 작업 규칙에 적용할 수 있습니다. issueops에는 child cycle과 parent의 accept/reject가 이미 있습니다.
- P6·P11과 정체 감지는 후보 O2(정체 일수, 원격 오류 원인 보존)와 같은 방향입니다. HEAD에 `idle_days` 계열 필드는 없습니다.
- 실행 횟수 신호는 issueops의 미해결 항목과 같은 모양입니다. 사이클 결과 보존(ADR 2026-10-23)은 코드가 있지만 2026-11-03 조사에서 보존 행이 0건이었고 원인을 구분하지 못했습니다.

### 7. 시각화 적용안

`issueops next`처럼 record를 읽기만 하는 투영을 두고, 그 결과로 자체 완결 HTML을 만드는 형태가 issueops 원칙에 맞습니다.

|화면|보여줄 것|record의 근거(`internal/contract/issueops`)|
|---|---|---|
|사이클 보드|사이클마다 현재 단계, 점유 상태, 미충족 게이트, 멈춘 기간|`issueops list`, `next`의 `missing`, `Record.UpdatedAt`|
|사이클 타임라인|단계 진입·완료, 회귀, 결정, 리뷰, 완료 이력|`PhaseLedgerEntry`, `RegressEvent`, `Decision`, `ImplementationReview`, `SkillRoutingEntry`, `ExecutionCompletionHistory`, `ExecutionSyncBaseEvent`|
|증거 그래프|의도 → 계획 → 리뷰 → 구현 → PR. 해시가 어긋난 연결은 stale로 표시. 부모·자식 사이클|`ReviewedFingerprint`, `reviewed_plan_digest`, `ChildCycleRef`, `DelegationContract`|

증거 그래프는 ARTEX의 `derived_from`·`yields`·`proves` 그래프에 대응합니다. issueops는 승인을 해시에 묶으므로 stale 연결까지 표시할 수 있습니다.

데이터 공백은 두 가지입니다.

- 통합 이벤트 기록이 없습니다. 시각이 필드마다 흩어져 있어 투영으로 조립해야 합니다. `PhaseLedgerEntry`의 주석대로 원장은 기존 필드의 색인입니다.
- record는 `cleanup finish`에서 삭제됩니다. 끝난 사이클의 타임라인을 남기려면 삭제 전에 `.issueops/issues/<번호>/`에 보고서를 저장해야 합니다.

## 후보 목록

우선순위 기준은 [2026-11-03 후보 보고서](2026-11-03-optimization-quality-candidates.md)와 같습니다. 모든 후보는 HEAD에서 다시 확인한 뒤 계획에 넣어야 합니다.

|ID|우선순위|제목|변경 대상|근거|위험|검증|
|---|---|---|---|---|---|---|
|D1|P1|사이클 타임라인 투영(JSON)|읽기 전용 capability vertical, `status`·`next`와 같은 계열|Findings 7|새 CLI/MCP 표면은 response-contract golden과 다섯 호스트에 퍼짐|fixture record로 투영 결과 표 테스트, 쓰기 없음 확인|
|D2|P1|사이클 보드 투영|`list` 확장 또는 별도 투영|Findings 7, O2|`list` 계약 변경|golden, 정체 사이클 fixture|
|D3|P2|증거 그래프 투영|노드·edge·stale 표시를 담은 JSON|Findings 7|stale 판정이 `next`의 게이트 판정과 어긋나면 안 됨|`next`와 같은 fixture에서 stale 일치 테스트|
|D4|P2|HTML 렌더링|투영 JSON을 읽는 자체 완결 HTML. core에 둘지 스킬에 둘지 계획에서 결정|Findings 4, 7|프런트엔드 빌드 체인과 상주 서버를 도입하지 않아야 함|생성 HTML을 브라우저로 열어 확인, 외부 요청 0건|
|D5|P2|끝난 사이클의 타임라인 보존|`cleanup finish` 전 보고서 저장|Findings 7|정리 순서 계약([issueops-cleanup.md](../architecture/issueops-cleanup.md))을 바꾸면 안 됨|정리 fixture에서 보고서 존재 확인|
|A1|P1|정체 감지|`list`의 정체 일수, `next` 경고|P10, 후보 O2|golden 갱신|정체 fixture|
|A2|P1|중단 원인 보존|`invoked_unknown` record에 원격 명령과 HTTP 상태|P6, P11, 후보 O2|redaction 필요|provider fake로 404 주입|
|A3|P2|백그라운드·보존 기능의 실행 횟수 신호|`doctor` 또는 `system-status`|Findings 5의 사례, 2026-11-03 Disputed 절|새 필드의 계약 변경|보존 경로를 실행한 뒤 횟수 증가 확인|
|A4|P2|스킬 규칙 목록화|스킬 본문의 규칙을 실패 유형별로 묶고 문장·코드 강제 여부를 표시|Findings 3|효과를 측정한 근거 없음. 목록만 만들고 승격은 기존 절차를 따름|목록 문서와 승격 후보 수|
|A5|P2|위임 계약에 팀 운영 규칙 반영|child cycle 위임 계약과 관련 스킬|P1, P5, P6, P8, P9|스킬 토큰 예산(5000) 근접|`validate-skill.py`, 위임 fixture|
|X1|—|2026-11-03 후보 중 남은 것|O2, O3, V3, V9, V10, I1 등|해당 보고서|HEAD에서 상태 재확인 필요. 커밋 `fc49729de..HEAD`에서 Q1~Q4, V1, V2, V4, V5, V7, R1~R3, O1, M1~M5, S1이 반영된 것으로 보임|후보마다 해당 보고서의 검증 방법|

**채택하지 않는 것**

- ARTEX 코드 복사: ARTEX는 AGPL-3.0이고 issueops는 GPL-3.0-or-later입니다. 설계만 참고해 새로 작성합니다.
- React·SSE 서버와 실시간 세션 기록 화면: Findings 6의 이유입니다.
- 계획자 루프와 실행자 풀을 core에 추가: issueops는 LLM 루프를 소유하지 않습니다.
- norma 도입: 1.0 이전이고 공개 커밋이 하나뿐입니다. issueops는 에이전트 루프가 필요하지 않습니다.

## Open Questions

- lease의 claim·release·replace 시각이 record에 이력으로 남는지 확인하지 못했습니다. 타임라인에 인계 구간을 그리려면 필요합니다.
- 실제 업무 사이클의 record 모양을 보지 못했습니다. 이 조사 머신의 record는 테스트 fixture 2건뿐이었습니다.
- ADR은 색인만 키워드로 검색했습니다. 시각화를 기각한 결정은 색인에서 찾지 못했습니다.
- HTML 렌더링을 core와 스킬 중 어디에 둘지는 결정하지 않았습니다.
- 사이클 결과 보존 행이 0건인 원인은 여전히 모릅니다.

## Source Index

|#|출처|설명|유형|권위|
|---|---|---|---|---|
|1|https://github.com/jiwoochris/artex-global (`main`)|ARTEX 소스: `agent/`, `server/`, `db/`, `web/`, `CHANGELOG.zh.md`|소스|High|
|2|https://github.com/Autumn-27/norma (`v0.4.4`)|에이전트 프레임워크: `harness`, `hook`, `agentcore`, `compaction`|소스|High|
|3|`.issueops/ARCHITECTURE.md`, `AGENT_WORKFLOW.md`, `architecture/issueops.md`, `architecture/runtime.md`|issueops 구조와 작업 흐름|저장소 문서|High|
|4|`.issueops/SUB_AGENT_PATTERNS.md`|위임 패턴과 금지 패턴|저장소 문서|High|
|5|`.issueops/research/2026-11-03-optimization-quality-candidates.md`|최적화 후보와 운영 신호|저장소 연구|Medium|
|6|`internal/contract/issueops`의 `Record`, `PhaseLedgerEntry`, `Execution`|record 필드|소스|High|
|7|`issueops list --json`, `issueops status --json`, `issueops model show --json`|CLI 출력(2026-11-11)|실행 출력|High|
|8|`git log fc49729de..HEAD`|2026-11-03 이후 반영된 후보|git 이력|High|
