# 요청자 의도 계약

- lifecycle: io-7426b49dc042
- issue: https://github.com/m16khb-org/issueops/issues/560
- intent_class: refactoring

## 원문 요청
orca에 종속되는건 별로인것 같아 orca를 걷어내려면 오래걸릴까?? / herdr 혹은 일반 터미널 두개만 가정하는게 어떨까 / 1,2,3까지만 일단 진행 (1: 기본 경로에서 Orca 빼기, 2: Orca 모드 코드 삭제, 3: 계약 정리. 4번 cmux 제거는 제외)

## 해석
IssueOps에서 Orca 연동을 완전히 걷어낸다. 실행 방식은 direct(git sibling worktree + lease) 하나만 남고, 세션 인계는 Herdr 또는 사용자가 직접 연 두 번째 일반 터미널에서 release → replace --preview → --reseed → claim 체인으로 한다. (1) 스킬·문서의 자동 세션 선택에서 Orca probe와 Orca 인계를 빼고 Herdr → 현재 세션/수동 터미널 순서로 바꾼다. (2) orca·auto 실행 모드, execution resume, execution switch-mode, Orca intent 체인과 reconcile 분기, Orca claim context 봉인, operational health·doctor의 Orca 판정, cleanup의 orca_remove 단계, handoff-delivery의 orca launcher, internal/adapter/orca·internal/port/orca*를 삭제한다. (3) CLI 플래그·MCP 스키마·기록 필드(execution.orca, Orca pending kind)·golden 계약에서 Orca 표면을 정리한다. cmux는 이번 범위에서 유지한다.

## 성공 기준
- git ls-files '*.go' | xargs grep -il orca 결과가 0개다
- issueops --help와 usage golden에 execution resume, execution switch-mode, --mode auto|orca가 없다
- 기존 direct 기록이 계속 읽힌다: 로컬 state에서 issueops list --json의 read_errors가 0이다
- 임시 저장소에서 direct prepare → release → replace --preview → --reseed → claim 인계 체인이 실제로 동작한다
- skills/ 아래 Orca 언급이 0개이고 자동 세션 선택이 Herdr → 현재 세션 또는 수동 터미널 순서다
- gofmt, go vet, go test ./..., go test -race ./..., go build, self-verify 95점 게이트가 통과한다

## 비목표
- cmux 연동 제거(handoff-cmux)
- 사용자 머신의 Orca 설치·리소스 정리
- 저장소 밖 사용자 전역 스킬(orca-cli, orchestration) 수정
- Herdr 연동을 Go 코드로 옮기는 일
- 과거 기록 문서(.issueops/issues, plans, verified-execution, research, archive) 수정

## 제약
- Codex·Claude Code·Omo·omp 네 호스트의 관찰 결과가 같아야 한다
- 기록 리더는 현재 모양만 받는다(2026-10-07 ADR). 마이그레이션 명령을 만들지 않는다
- lease generation·holder·canonical cwd fencing은 약화하지 않는다

## 모호함
- resolved: 현재 로컬 기록 24개 중 Orca mode 기록은 0개라 마이그레이션이 필요 없다
- deferred: --mode·--direct-reason·--expected-readiness-fingerprint 플래그를 없앨지 direct 단일 값으로 남길지는 계획 단계에서 코드 결합을 확인해 정한다

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
