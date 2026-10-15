# 요청자 의도 계약

- lifecycle: io-9e1b5fa96747
- issue: https://github.com/m16khb-org/issueops/issues/563
- intent_class: standard

## 원문 요청
1

(직전 선택지 원문) 1. main의 lint 부채를 별도 사이클에서 먼저 고친다 (권장): 고친 뒤 이 사이클은 `sync-base`로 base를 맞추고 reseed해서 새 head로 CI를 다시 받은 다음 머지합니다. 범위가 섞이지 않습니다. 참고로 #560(orca 연동 제거, 현재 plan 단계)이 머지되면 orca 관련 지적이 함께 사라집니다. 그래도 trace, issueopsapp, sqlstore 등에 남은 지적은 따로 고쳐야 합니다.

## 해석
main(bac123aa)의 CI verify job이 2026-10-12 dd271b8b 이후 golangci-lint v2.12.2 단계에서 90건(errcheck 62, govet inline 25, unused 3)으로 실패해 Build·race test·self-verify 단계가 skipped된다. 이 때문에 PR #562(#561)를 포함한 모든 PR이 checks_failing으로 머지되지 못한다. 별도 사이클에서 지적 90건을 코드와 테스트에서 직접 고쳐(설정 제외·nolint 억제 없이) main CI를 다시 green으로 만든다. 이후 #561 사이클은 sync-base와 reseed로 새 head에서 CI를 다시 받아 머지한다(이번 사이클 범위 밖).

## 성공 기준
- CI와 같은 GOOS=linux, go.mod toolchain의 golangci-lint v2.12.2 run ./... 이 0 issues로 끝난다; go vet ./..., go test ./... -count=1, go test -race ./... -count=1, go build ./cmd/issueops 가 통과한다; 이 사이클 PR의 GitHub Actions CI verify job이 lint 이후 단계까지 모두 success다; .golangci.yml의 linter·제외 설정과 nolint 주석은 늘리지 않는다.

## 비목표
- (없음)

## 제약
- (없음)

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
