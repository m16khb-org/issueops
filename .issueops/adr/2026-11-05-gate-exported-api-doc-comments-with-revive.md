---
name: 2026-11-05-gate-exported-api-doc-comments-with-revive
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Gate exported API doc comments with revive

- Date: 2026-11-05
- Kind: `adr`
- Source: cli
- Summary: revive의 exported·package-comments 두 규칙을 .golangci.yml에 켜서 exported 식별자와 패키지의 문서 주석 누락을 CI와 lint-local.sh가 실패로 막는다. 테스트 파일은 제외하고, 이 규칙을 숨기는 comments 제외 preset을 뺀다. revive의 나머지 규칙과 다른 문체 linter는 계속 끈다. 2026-11-04까지의 '문체 linter(revive 포함)는 켜지 않는다' 결정 가운데 revive 부분을 대체한다.
- Context: #572(PR #576)가 exported 식별자의 문서 주석 누락 3,053건과 package comment 누락을 0건으로 만들었다. 그러나 측정은 이슈 폴더의 임시 설정(`.issueops/issues/572/effective-go-lint.yml`)으로만 했고 저장소 게이트(`.golangci.yml`)는 revive를 끄고 `comments` 제외 preset도 켜 두었다. 게이트가 없으면 새 코드가 주석 없는 exported 이름을 다시 쌓는다. 지금은 기존 지적이 0건이라 켜는 비용이 새 코드에만 든다.
- Decision: 1) `.golangci.yml`에서 revive를 켜고 규칙을 `exported`와 `package-comments` 두 개로 한정한다. `exported`는 doc comment 누락, 이름으로 시작하지 않는 주석, 패키지 이름을 반복하는 이름(stutter)을 잡는다. 2) `_test.go`는 revive에서 제외한다. 3) `comments` 제외 preset은 이 두 규칙의 지적을 숨기므로 뺀다. staticcheck는 SA·S1만 켜 두어 이 preset이 숨기던 ST1000·ST1020~ST1022는 여전히 꺼져 있다. 4) revive의 나머지 규칙과 gocritic·perfsprint·predeclared·stylecheck·quickfix는 계속 끈다. 의도한 예외는 기존 규칙대로 사유를 붙인 `//nolint:revive // <사유>`로 남긴다.
- Consequences: 새 exported 함수·타입·상수·변수·메서드와 새 패키지는 doc comment 없이 CI lint와 `scripts/lint-local.sh`를 통과하지 못한다. 패키지 밖에서 쓰지 않는 이름은 주석 대신 비공개로 바꾸는 선택이 생긴다. `testing/unit-and-contract.md`의 linter 정책이 이 결정을 따른다. 2026-11-04 Effective Go ADR의 "revive exported 규칙을 상시 게이트로 켤지는 별도 결정" 항목이 이 기록으로 닫히고, 같은 기록의 "revive/ST stay off the repo gate" 중 revive 두 규칙 부분을 대체한다. windows 전용 파일은 lint-local이 보지 않으므로 GOOS=windows 측정은 필요할 때 따로 돌린다.
- Alternatives / rejected options:
  - 게이트 없이 두기 — 기각: 주석 누락이 다시 쌓여 #572 같은 일괄 작업이 반복된다.
  - `internal/contract`·`internal/port`에만 켜기 — 기각: 경로별 예외 규칙이 늘고, 0건인 지금은 전체에 켜도 추가 비용이 없다.
  - revive 기본 규칙 전체 켜기 — 기각: 이름·흐름 문체 규칙은 결함을 잡지 않는다는 기존 판단(2026-10-03 lint 범위)이 그대로 유효하다.
- Evidence:
  - `./scripts/lint-local.sh`: darwin `0 issues.`, linux `0 issues.`(설정 변경 뒤, main `59b6cf1a1`).
  - 임시 패키지에 주석 없는 exported 함수를 두면 `exported function ... should have comment or be unexported (revive)`로 실패하는 것을 확인하고 지웠다.
  - golangci-lint v2.12.2 `pkg/result/processors/exclusion_presets.go`의 `comments` preset이 revive `exported`·`package-comments` 지적(EXC0012~EXC0015)을 제외한다.
