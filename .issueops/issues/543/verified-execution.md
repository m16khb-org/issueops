# #543 검증 기록

Lifecycle: io-ca6dfd9d657f. 기준은 봉인된 branch_prepare.base_sha이며 구현은 canonical worktree에서 수행했다.

## 의도 대조

- 커밋된 Go 변경: TestCommittedOnlyGoScopeSelectsRaceAndVet가 수정 전 tier=standard로 FAIL했고 scope 연결 뒤 race/vet 선택으로 PASS했다.
- dirty union과 범위: TestScopedPlanUnionPreservesNULPathsAndRenameSources, TestScopedPlanEmptyAndInvalidRefs, TestScopedPlanUsesTreeRangeAndPreservesObservationFailures가 commit·staged·unstaged·untracked 합집합, 빈 범위, invalid ref·관측 실패, 분기된 tree 비교를 확인했다.
- 경로: 양끝 whitespace, quote, newline, Go→non-Go rename의 source와 destination을 실제 임시 Git 저장소에서 확인했다.
- 출력: 성공·실패 20 KiB 로그와 oversized scope failure plan에서 기준·HEAD·오류 근거와 8 KiB aggregate budget 및 실패 진단을 보존했다.
- 표면: CLI request capture, MCP dispatcher, 실제 SDK tools/call, production facade→loop→collector→runner 회귀를 통과했다. 스킬의 exact status 조회·branch_prepare.base_sha·base-ref 전달을 정적 대조했다.

## 실패와 교정

기존 테스트의 TrimSpace 기대는 실제 Git 경로 구별 계약으로 바꾸고 별도 whitespace 회귀를 추가했다. MCP schema의 minLength는 기존 canonical validator가 지원하지 않아 SDK 요청이 실패했다. 실패를 별도 재현한 뒤 지원되는 pattern과 입력 검증을 사용했으며 같은 SDK 회귀가 통과했다. 실패 로그는 review용 evidence bundle에 보존했다. 독립 diff 리뷰는 변경된 문서의 이전 working-tree 전용 문구를 고정한 단위 테스트도 발견했다. named RED를 재현한 뒤 assertion을 optional base·standalone·실패 계약으로 바꾸고 두 스킬의 exact cycle 조회와 실제 base-ref 호출도 확인했다. 기존 vet/race 명령 증거 assertion은 유지했다. full 시작 전 coordinator가 요청한 DDD inventory 검사도 named FAIL로 재현했다. 정규 update-ddd-inventory로 이번 변경의 6개 production 파일에 해당하는 심볼 7개 추가·1개 제거만 동기화했고 owner·task·policy·artifact 목록은 그대로임을 JSON 대조했다. inventory와 contract function role의 named 검사도 통과했다.

최종 single-pass의 첫 실행은 설치 계약 두 검사에서 실패했다. 모든 격리 환경 변수를 제거한 named 재현에서도 같은 실패가 났고, 새 base_ref 스키마가 만든 MCP catalog digest가 기존 tracked 설정과 달랐다. 이전·현재 catalog digest를 각각 golden과 대조하고 현재 digest가 실제 실패의 project config hash를 그대로 만드는 것을 확인했다. coordinator 승인 범위에서 Omo·agy tracked template의 catalog digest 두 값과 install golden의 content hash 10개만 갱신했으며 다른 snapshot 값은 바뀌지 않았다. 두 named 검사와 agy 관련 fixture 검사는 통과했고 원본 실패 로그는 저장소 밖 evidence bundle에 보존했다.

## Side effect

optional CLI/MCP 기준 입력, risk plan scope JSON, 기준 전달 스킬 및 검증 규칙을 추가했다. Git은 ref 해소·diff·status만 읽으며 fetch·ref 변경·원격 쓰기를 하지 않는다. Git 관측은 명시 cwd와 30초 timeout을 사용하고 external diff/textconv를 비활성화한다. runtime state는 저장소 밖 임시 디렉터리에 격리한다. 실제 사용자 설치 설정은 테스트로 갱신하지 않는다.

## 성능 측정

named scope battery에서 collector 4개 테스트는 4.259초, production facade 및 SDK 회귀를 포함한 issueopsapp focused 검사는 4.945초였다. scope 지정 시 Git 4회(base/HEAD/diff/status), 무기준 실행은 status 1회이며 plan은 risk 실행과 coverage 재사용에서 공유한다. hot path 최적화나 전체 성능 개선은 주장하지 않는다.

## 정리와 품질

production Go 추가 줄에서 설명을 반복하던 신규 주석 한 줄을 제거했다. 같은 줄 기반 heuristic으로 SNR은 0.9276→0.9338, declaration boilerplate 비율은 0.0066→0.0066, 길이 24 초과 substantive 줄의 반복 비율은 0→0이었다. 이는 AST 기반 품질 지표가 아니라 변경분의 줄 측정이다. 기존 human-readable ParseGitStatusPath API는 보존했고 신규 collector는 NUL 형식만 사용한다. 기존 core classifier·runner·StepResult schema·CI 독립 vet/race를 유지했다.

## 검증 범위

API static 및 host-agent review는 explicit MCP schema 파일에 대해 PASS했다. full deterministic single-pass self-verify와 독립 구현 리뷰 결과는 execution completion receipt 및 PR의 확인 기록에 남긴다. full-suite가 race에 포함되면 같은 run의 go test 및 golden step은 기존 규칙대로 재사용하며 동일한 배터리를 중복 실행하지 않는다.

## 운영 문서 대조

MCP project_docs_route/read로 최종 문서와 SHA를 확인했다. 문서 한 문장의 수정은 구현 단계에서 직접 편집했으므로 project_docs_revise는 거치지 않았다. CONSTITUTION의 실패 정확성·cwd·timeout, ARCHITECTURE의 adapter I/O와 application/domain 경계, CONVENTIONS의 공용 CLI/MCP DTO·선택 필드·snake_case, CAUTIONS의 named RED와 출력 보존, ADR의 thin adapter 및 standalone 계약을 대조했다. 검증 범위는 testing/unit-and-contract.md와 두 스킬에 반영했으며 동료가 맡은 TESTING.md 및 testing/self-verification.md는 수정하지 않았다.

## 완료 경계

이 워커의 종료점은 Draft PR 발행·CI 확인·execution complete와 lease release다. merge·cleanup·io-update는 root coordinator가 수행한다. Hook은 context만 제공하며 실제 구현·검증·원격 발행·completion은 CLI와 active holder가 수행한다.
