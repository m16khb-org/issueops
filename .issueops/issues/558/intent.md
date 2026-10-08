# 요청자 의도 계약

- lifecycle: io-7dff0c24f1e9
- issue: https://github.com/m16khb-org/issueops/issues/558
- intent_class: standard

## 원문 요청
로 하나의 이슈로 깔끔하게 정리하여 진행

## 해석
직전 턴에서 도출한 issueops 품질·성능·최적화 개선점(1~9, 11번)을 이슈 하나로 묶어 IssueOps 사이클 전체(이슈→계획→구현→검증→draft PR)를 진행한다. 10번 복잡도 hotspot 목록은 구체적 수정안이 없는 관측이므로 비목표로 둔다. 대상: (1) main CI errcheck 실패 수정 (2) 로컬 최종 battery에 toolchain 고정 golangci-lint 추가 (3) sqlstore read-only 조회의 row별 DB open과 list-then-get 루프 제거 (4) channel recv wait의 context 취소 반영 (5) channel_v1 retention (6) internal/adapter/issueops 테스트 병렬화 (7) MCP HTTP server.log 크기 제한 (8) 은퇴한 subsystem의 user state 잔재 정리 (9) omo/omp adapter 중복 통합 (11) shellQuote 의미 통일과 bucket 상수 단일화. 계획 검토에서 채널 retention 경로는 state prune 대신 channel send 시 정리로, 은퇴 경로 삭제는 state maintain 대신 install/update로 바뀌었다.

## 성공 기준
- go.mod toolchain으로 실행한 golangci-lint가 darwin과 linux 모두 0건이고 PR CI가 통과한다
- 로컬 최종 검증 battery 문서에 toolchain을 고정한 golangci-lint 단계가 들어간다
- 다른 채널 메시지 500개가 있는 state에서 channel recv가 50ms 이하로 끝난다(기준 520ms)
- channel recv --wait가 요청 context 취소 시 즉시 반환한다(application 테스트와 2026-07-28 프로토콜 MCP HTTP 테스트로 고정)
- channel send가 보존 기간(7일)이 지난 채널 메시지를 지운다
- go test ./internal/adapter/issueops -count=1 단독 wall time이 44.5초 이하가 된다(전체 suite 기준 89초, 단독 기준 74초)
- MCP HTTP server.log가 크기 상한을 넘으면 rotation되어 무한히 커지지 않는다
- 은퇴한 daemon/, hook-failures.jsonl, hook-metrics.jsonl, .last-store-maintain, issueops-migration-receipt.json 잔재를 state doctor가 retired_path로 보고하고 install/update가 commit 뒤 제거한다
- omo와 omp host adapter가 하나의 구현을 공유하고, install 결과 문구와 생성 설정 바이트가 변경 전과 같다
- gofmt, go vet, go test -race ./..., self-verify가 통과한다

## 비목표
- gocyclo/gocognit 상위 함수 리팩터링
- hook 지연·바이너리 크기 최적화
- 새 기능 추가

## 제약
- 4개 host(Codex/Claude/Omo/omp)의 관찰 결과와 CLI/MCP JSON 계약, advertised MCP catalog를 바꾸지 않는다
- sqlstore의 read-only 조회 계열은 state 파일을 만들거나 고치지 않는다
- state maintain은 비파괴로 남는다

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
