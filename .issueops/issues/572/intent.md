# 요청자 의도 계약

- lifecycle: io-7426b49dc042
- issue: https://github.com/m16khb-org/issueops/issues/572
- intent_class: refactoring

## 원문 요청
현재 프로젝트가 Effective Go의 관례에 맞춰 잘 작성되어있는지 전체 점검 및 리팩토링 workflowz
[점검 결과 공유 후 사용자 선택] 범위: A + B + 문서 주석·IssueOps* 이름 변경 / 진행 방식: IssueOps 사이클로 진행

## 해석
Effective Go(go.dev/doc/effective_go) 관례 점검 결과(임시 revive·staticcheck ST 측정 4,863건, 관점별 리뷰 7개)를 근거로 세 묶음을 리팩토링한다. (A) 동작을 바꾸지 않는 관례 위반: 디렉터리와 다른 패키지 이름 3개, 내장 식별자 가림, 에러 문자열 비교, 대소문자만 다른 메서드 쌍, 누락된 인터페이스 구현 확인(var _), 불필요한 if-return·else, wg.Go, stutter 이름(패키지 이름 접두어 뒤 대문자인 package-level exported 이름 전부: 28개 패키지 205개와 clause 변경 뒤 issueopsremote 8개). (B) port·시그니처·종료 경로: IssueProvider의 context 유무 이중 API 통일, 네트워크 외부 명령·HTTP에 context 전달, project lint-diagnose의 깊은 os.Exit 제거, stdio MCP 진입점의 SIGINT·SIGTERM 처리와 EOF drain 대기 취소, providerutil 에러 체인 보존. port.InstallPlan·PRGitObservations는 근거(#499 통합 고정, 단일 소비자가 전부 사용)에 따라 축소하지 않는다. (C) exported 식별자·패키지 문서 주석 보강. CLI/MCP 이름, JSON 필드, response golden, persisted schema는 바꾸지 않는다.

## 성공 기준
- 임시 Effective Go lint 설정(revive exported/package-comments/var-naming/confusing-naming/redefines-builtin-id/if-return/indent-error-flow 등, staticcheck ST1000~ST1023)으로 다시 측정했을 때 production 코드의 비주석 지적이 0건이고, exported·package 주석 지적이 0건이다(정당한 예외는 근거 주석이 있는 nolint로만 남긴다)
- go build ./..., go vet ./..., gofmt -l, ./scripts/lint-local.sh(GOOS=linux 포함), go test ./... -count=1, go test -race ./... 가 모두 통과한다
- response-contract golden(go test ./cmd/issueops/contractgolden -run Golden, issueopsapp TestResponseContractsGolden)이 갱신 없이 통과해 CLI/MCP 이름·JSON 필드가 그대로임을 보인다
- B군의 동작 변경(context 취소, lint-diagnose 종료 코드, stdio 신호 처리와 drain 취소, 에러 체인)마다 실패-후-통과 테스트 또는 실행 증거가 있다
- domain/issueopsremote, domain/guardpattern, domain/traceclassification의 package clause가 디렉터리 이름과 같고 importer 별칭이 한 가지로 정리된다

## 비목표
- CLI 명령·MCP tool 이름, JSON 필드 이름, persisted record schema, response golden 변경
- 동사 없는 fmt.Errorf 1,335건 일괄 변경(.golangci.yml이 perfsprint를 스타일 churn으로 끈 결정을 유지)
- 고유 명사로 시작하는 에러 문자열 4건(Claude/Codex/Herdr) 변경
- selfverify/steps.go의 불변식 panic, readiness_service.go의 의도적 copy-on-write 값 리시버 변경
- port.InstallPlan 축소(#499 통합 결과, install_plan_contract_test.go가 고정)
- PRGitObservations 분리(유일한 소비자가 11개 메서드를 모두 씀)
- dated 기록 문서(adr/20*, cautions/20*, plans/ 등)의 옛 이름 수정

## 제약
- 레이어 의존 규칙(internal/architecture/dependency_test.go)과 composition root 단일 예외를 유지한다
- 기존 re-export는 레이어 규칙상 필요한 것만 근거 주석과 함께 둔다(2026-10-06 ADR delete-re-export-shims)
- adapter/augmentation/signals.go가 소스 문자열(service.writeRecord( 등)을 grep하므로 이름 변경 시 함께 갱신한다

## 모호함
- resolved: 범위는 사용자가 A+B+문서 주석·IssueOps* 이름 변경으로 선택했다
- resolved: 진행 방식은 사용자가 IssueOps 사이클로 선택했다
- deferred: revive exported 규칙을 상시 lint 게이트로 켤지는 C3 완료 뒤 측정 결과로 결정한다(.golangci.yml의 스타일 linter 비활성 결정과 충돌하므로 ADR 필요)
- deferred: 문서 주석 언어(영어/한국어)는 패키지 안의 기존 주석 언어를 따른다
- resolved: 사용자가 단일 이슈로 진행을 선택했다(하위 #573~#575는 not planned로 닫음)
- resolved: 사용자가 stdio drain을 signal 처리까지 포함해 고치기로 선택했다

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
