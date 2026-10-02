# 에이전트 업데이트와 issueops 개선 조사

STATUS: complete — 연구 완료, self-verify 환경 실패 별도 기록

## 질문과 범위

2026-10-02에 접근 가능한 Claude Code, Codex, Omo와 주요 에이전트의 업데이트를
확인하고 issueops의 성능, 최적화, 가시성을 개선할 수 있는 지점을 찾는다.
제품 기능의 존재, 현재 설치 상태, issueops 적용 가능성, 측정 전 가설을 구별한다.
프로덕션 코드, 사용자 설정, 설치 상태, Git 이력은 변경하지 않는다.

## 조사 구성

1. Claude Code: 릴리스, 컨텍스트, 도구, 실행, 관측.
2. Codex: 릴리스, SDK, 프로토콜, 실행, 관측.
3. Omo: 설치된 구현, DAG, 모델 라우팅, 도구, 복구.
4. 다른 에이전트: Gemini CLI, Copilot, Cursor, OpenCode, Aider, Cline 등.
5. 공통 기술: MCP, 스킬, 캐시, 추적, 평가, 격리.
6. issueops: 현재 구현의 경계, 비용, 상태, 검증, 가시성.

첫 DAG는 독립 조사 60개와 근거 검증 집계 3개로 구성한다.
후속 DAG는 실제 수집 결과와 남은 질문으로 구성한다.
각 조사자는 자기 파일 하나에만 근거를 남긴다.
메인은 통합 근거, 주장 판정, 후속 질문, 최종 보고서를 관리한다.

## 모델 배정

요청: 단순 조사 GPT-6 Luna, 중간 분석 GPT-6.1 Sol, 복잡한 판단 GPT-6 Astra.
로컬 omo.jsonc의 category 기본값은 이 요청과 다르다.
사용자 지정 모델을 지키기 위해 workflow의 subagent_type + model override를 사용한다.
전역 설정은 수정하지 않는다. 실제 dispatch 결과로 모델 수락 여부를 확인한다.

## 산출물

- lane: no-format (answered_by: request)
- destination: 현재 대화와 저장소의 조사 Markdown (answered_by: request/default)
- audience: issueops 개발자 (answered_by: request)
- detail: 상세 기술 조사 (answered_by: request)
- format: Markdown 보고서와 근거 파일 (answered_by: default)
- 최종 파일: ../agent-updates-2026-10-02.md
- 이미지, PDF, 배포 산출물은 요청 범위에 포함하지 않는다.

## 근거 계약

외부 사실은 실제 읽은 URL, 조회일, 발표일 또는 버전을 기록한다.
로컬 사실은 파일:라인과 관측 버전을 기록한다.
같은 공급자의 여러 문서는 독립 출처로 세지 않는다.
공식 릴리스 자체는 단일 1차 출처 예외로 표시할 수 있다.
개선 효과는 직접 측정하지 않은 한 가설이며 숫자를 만들어내지 않는다.
최신성은 달력 날짜가 아니라 확인된 릴리스와 문서 날짜로 제한한다.
검색 결과 요약만으로 기능을 확정하지 않는다.

## 시작 관측

- 저장소의 초기 git status --short 출력은 비어 있었다.
- .codegraph 디렉터리는 없다.
- Go core와 얇은 호스트 어댑터가 프로젝트의 현재 구조적 제약이다.
- SessionStart는 context-only이며 hook enforcement를 되살리는 제안은 배제한다.
- 조사만 수행하므로 구현 테스트를 기능 검증인 것처럼 보고하지 않는다.

## 진행 기록

- 준비: mass-ulw planning 전체, 연구 계약, 헌법, 구조, 컨벤션, 검증 지침을 확인했다.
- 수집: `dag_337baed6-3a8d-4b9f-b88b-36efbb29b096`을 시작했다.
  Luna 41개, Sol 22개(검증 집계 3개 포함)를 요청했다.
- 실행 관측: 첫 작업 `st_01a0fa37`의 실제 모델은
  `chatgpt-subscription/gpt-6-luna`로 확인됐다. 나머지 모델의 실제 실행은
  각 작업의 영수증으로 확인하며, 요청값만으로 실행됐다고 간주하지 않는다.
- 초기 보고서: Claude 릴리스와 메모리 조사 파일이 생성됐다. 작업은 아직
  실행 중이므로 완료 근거로 인정하지 않는다.

## 최종 상태

위 진행 기록은 수집 당시의 기록이다. 기본 DAG 63/63, 추가 DAG 5/5가 완료됐고
메인은 실제 파일·원문·코드·변경 범위를 검증했다. Luna·Sol·Astra 모델 실행을
확인했으며 Fable 5.1의 최종 독립 검토 지적을 반영했다.
[근거 색인](sources-ledger.md), [검토 반영표](fable-5.1-review.md),
[self-verify 실패](self-verify-result.md)를 참조한다.
구현·설치·운영 설정·Git 이력 변경은 이 조사의 산출물이 아니다.
