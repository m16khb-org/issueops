# 병렬 실행과 화면 QA 세부

SKILL.md의 `## 0` 절에서 넘어온 세부 규칙이다.

봉인된 fingerprint와 1절의 증거 재사용 조건을 먼저 확인한다. 아래 작업 중 유효한
증거가 없는 필수 작업만 고르고, 입력과 실행 환경을 공유해 서로 영향을 주지 않는
작업을 **같은 fingerprint에 대해 동시에** 시작한다. 같은 DB나 브라우저 상태를 쓰는
검증은 순서대로 실행한다. 리뷰어에게는 기존 검증 증거와 아직 실행 중인 항목을 구분해
전달하고, 추가 결과가 판정에 영향을 주면 기록 전에 그 결과로 delta 리뷰를 받는다.

1. 게이트 원장과 저장소 검증 배터리(1절). `gates check`는 `--write` 없이 실행한다.
   `--write`는 4·5단계가 소유한다.
2. 스키마 실측(2절). 변경 집합에 스키마 파일이 있을 때만 활성화된다.
3. 구현 리뷰 서브에이전트(3절).
4. 화면 QA(`next`의 `review.frontend`가 true일 때만).
   [`aside-web-qa`](../../aside-web-qa/SKILL.md)에 네 입력을 이렇게 매핑한다.
   TARGET=계획이나 이슈 본문의 로컬 실행 절차가 준 URL, SCOPE=변경 집합의 frontend 경로,
   REQUIREMENTS=intent 문서와 이슈의 성공 기준, `gates.md`, INTENT=4단계 report의 `## UI 판단` 절.
   넷 중 하나라도 없으면 지어내지 말고 QA를 `Not Run`으로 report에 사유와 함께 적는다.
   보고서 출력 경로는 ignored 영역 `.issueops/issues/<n>/review/`나 워크트리 밖으로
   고정한다 — 워크트리 안 미추적 파일은 fingerprint에 들어가 봉인을 깬다. 제품을 바꾸는
   시나리오는 그 스킬의 allowed mutations·cleanup 계약 안으로 한정하고 정리 영수증을
   report에 적는다. `aside-functional-qa`·`aside-visual-qa`를 직접 부르지 않는다.

- **결과를 모두 모은 뒤 판정을 기록한다.** 배터리가 실패하거나 필수 스키마·QA 증거가
  빠졌으면 먼저 도착한 리뷰 `pass`를 기록하지 않는다. 실패 출력과 리뷰 지적은 수정에
  쓸 근거로 보존하고 4단계로 돌아간다. 수정으로 fingerprint가 바뀌면 이전 판정을
  재사용하지 않는다. `Not Run`이 성공 기준 검증을 누락했다면 readiness의 blocker다.
- 이 동시 실행의 전제는 정리 단계가 관련 검증을 이미 통과시켰다는 것이다. 배터리
  실패가 드물지 않으면 병렬화가 아니라 5단계를 먼저 고친다.
  검증 명령의 실패 기록으로 판단한다. `issueops review-metrics --repo "$WORKTREE"
  --json`의 revise 비율은 계획 리뷰 판정 비율이며 배터리 실패율이 아니다.
- 이 fan-out은 `SUB_AGENT_PATTERNS.md`의 기대 이득 `parallel_speed`에 해당한다.
  그 slug와 실제로 절약한 벽시계 시간을 verified-execution report에 적는다.
- 정리(5단계)·문서 반영(6단계)과는 동시에 실행하지 않는다. 그 둘은 파일을 쓰므로
  봉인을 바꾼다.
