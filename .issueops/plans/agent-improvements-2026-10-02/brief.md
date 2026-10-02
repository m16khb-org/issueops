# 10개 개선 구현 계약

상태: 설계 및 구현 진행 중.
시작 HEAD: `92eaa00143841f964c285dacdd41aedbeeecdf2e`.
시작 작업 트리: clean. `.codegraph/` 없음.

## 사용자 승인

사용자는 조사에서 도출한 10개 개선을 모두 구현하라고 승인했다.
공용 MCP를 조회 전용으로 제한하지 않는다. 서버 자신의 부모 프로세스 계보를
호출자 신원으로 삼는 결합을 대체하고, 공용 서버에서도 mutation을 지원한다.
세션마다 상주하는 stdio proxy를 새로 두어 서버 공유 효과를 없애지 않는다.

기존 조사:
`.issueops/research/agent-updates-2026-10-02.md`.
이 문서의 사실·프로토콜 버전·SDK 지원 교정을 재사용하며 대규모 웹 조사를
처음부터 반복하지 않는다.

## 수용 범위

| ID | 구현할 결과 | 최소 수용 기준 |
|---|---|---|
| I1 | JSONL 불완전 분석 표시 | 큰 행·손상 행 뒤의 누락을 완전 성공으로 숨기지 않음 |
| I2 | 네 probe 시간과 재사용 통계 | 실제 측정값 보존, 미실행·재사용·표본 수 구별, sleep 없는 테스트 |
| I3 | 호스트 진단 상태 분리 | 설치·링크·발견·연결·protocol 지원을 증거 수준별로 표시 |
| I4 | 구조화 MCP 결과 | 대표 read-only 도구에서 structuredContent·schema·정확한 annotation, text/CLI 호환 |
| I5 | 최신 SDK 및 protocol 호환 | 지원 범위를 확인한 SDK 갱신, 2025/2026·stdio/HTTP와 세 호스트 검증 |
| I6 | 요청 내 중복 조회 축소 | Git/history 관측 횟수 감소, fetch·다른 root/ref·동시 변경 뒤 stale 재사용 없음 |
| I7 | metadata discovery 개선 | bounded body 읽기, 결정적 cap 선택, 생략 수, 필수 정보·symlink/root 안전성 유지 |
| I8 | 단계별 지연 관측 | wait/callback/commit/전체 시간을 분리하고 계측 overhead 확인 |
| I9 | 읽기 전용 host 사용량·trace 연결 | 지원되는 입력·export 표면, unknown과 0 구분, 중복 usage 집계 방지, TRACEPARENT 전파 조건 |
| I10 | 로컬 공용 Streamable HTTP MCP | 세 호스트가 직접 연결, 신뢰된 요청별 actor, mutation, fencing·workspace·원자성·취소·restart 검증 |

## I10의 필수 계약

- 보존할 것은 실행 소유권, claim 권한, holder, generation, workspace, 원자적
  변경과 stale 요청 거부다. 서버 PID ancestry 자체를 보존할 필요는 없다.
- caller가 보내는 session_id/PID/clientInfo만으로 권한을 부여하지 않는다.
- 요청별 신원·권한의 발급, 전달, 검증, scope, 만료/철회, 재시작 후 의미를
  먼저 결정한다. 기존 state/claim/resume 구조를 재사용할 수 있는지 확인한다.
- stdio 호환은 유지하되, shared HTTP를 매 세션 상주 proxy 뒤에 숨기지 않는다.
- MCP transport session과 application actor/lease를 구분한다.
- shared process의 전역 current actor/current workspace 변수를 금지한다.
- install/update/bootstrap과 서버 start/stop/status의 연결을 설계한다.
  실제 사용자 설정을 바꾸는 대신 먼저 격리 HOME/state에서 설치·동작을 증명한다.
- 처음부터 외부 네트워크 공개·multi-tenant SaaS·별도 DB·scheduler를 만들지 않는다.

## 구현 원칙

Go core와 얇은 adapter, context-only SessionStart, 요청별 policy 재평가,
schema-version 거부 정책, secret redaction을 유지한다.
승인된 공유 서버 방향과 충돌하는 in-process-only ADR은 변경 근거와 함께 갱신한다.
source 변경 없이 상태 파일을 편집하거나 기존 실패 테스트를 약화하지 않는다.
API/DTO 변경은 static-check와 문서 리뷰 계약을 따른다.

기존 Python 테스트의 pydantic 누락은 baseline 환경 문제다.
저장소의 기존 dependency/test 환경을 먼저 확인하고 격리된 환경에서 검증한다.
테스트를 생략하거나 전역 Python에 임의 패키지를 설치하지 않는다.

## 모델·실행

요청 모델: GPT-6 Astra, GPT-6.1 Sol, GPT-6 Luna, Claude Fable 5.1,
Claude Opus 5.5, Claude Sonnet 5.5. high effort는 이 세션의 작업자 배정 참고 사항이다.
사용자 정정에 따라 issueops 기능·설정이나 별도 완료 게이트로 구현하지 않는다.
모델 설정 조사를 늘리지 않고 기능 구현·검증을 우선한다.

단계별 DAG:
1. 경계·테스트·요청별 신원 설계의 독립 조사 → 통합 계약 → 설계 검증.
2. 계약이 확정된 disjoint 구현과 각 회귀 검증 → 통합 검증.
3. 선행 DTO/authority/SDK가 필요한 후속 구현 → 실제 stdio/HTTP 검증.
4. 전체 진단·테스트·race·build·운영 문서·독립 최종 검토.

동시에 쓰는 파일은 겹치지 않게 한다. 후속 DAG는 직전 결과를 메인이 확인한 뒤
정의하며, failed node만 retry/amend한다. 결과 보고만으로 완료를 인정하지 않는다.

## 변경 범위

이번 구현의 별도 commit/push는 아직 요청되지 않았다.
다른 사용자 변경을 수정·되돌리지 않고 Git 이력·원격은 건드리지 않는다.
실제 성능 수치와 단순 가설, 자동 테스트와 실제 호스트 확인을 구분해 보고한다.
