# #534 self-verify 결과 판정 교정 계획

Lifecycle: io-c63caa7ea2ee. Issue: https://github.com/m16khb-org/issueops/issues/534
Source root: $HOME/workspace/issueops. Branch: 534-selfverify-evidence.
Base: b917fc960b72bca3e616df1162c6b89d40a517ba. Canonical worktree는 execution prepare 응답으로 확정한다.

## 목적과 승인 범위

실행하지 않은 golden 검사와 unhealthy binary drift가 성공으로 표시되지 않게 한다. 사용자 2026-10-01 지시는 다섯 이슈의 병렬 구현, main 병합, cleanup, io-update까지 승인했다. 과거 감사 전용 제한은 이 지시로 대체된다. 이 native owner는 GPT-6.1 Sol high로 구현부터 검증된 draft PR 게시, 최신 push/PR CI 성공 확인, execution complete/released까지 수행한다. 상위 조정 세션이 병합, cleanup, io-update를 맡는다. 현재 워크트리에서 인수 후 새 세션을 다시 띄우지 않는다.

## 적용되는 결정과 주의사항

- .issueops/CONSTITUTION.md 제2장 안전/정확성: exit code와 의미상 성공을 구분하고 실제 실패 fixture로 증명한다.
- .issueops/ARCHITECTURE.md 의존 방향, .issueops/CONVENTIONS.md layer: 결과 해석은 application/domain에 두고 generic process adapter는 exit-code 계약을 보존한다.
- .issueops/ADR.md accepted baseline: 공용 외부 Go core이며 host별 검증 분기는 추가하지 않는다.
- .issueops/CAUTIONS.md Removed CLI modes와 .issueops/testing/self-verification.md 단일-run 완료 battery: 현재 single-pass 계약과 full-suite 성공 증거 재사용을 유지한다. 최종 battery는 같은 revision/환경의 단일 bundle로 남긴다.
- .issueops/TESTING.md 최소 완료 기준: Go 변경은 정적 검사, race, self-verify 실제 결과를 확인한다. 다른 run의 부분 통과를 합치지 않는다.

## 재사용하는 기존 구현

- internal/application/selfverify/steps.go:139-149 CachedContractGoldenStep은 성공한 full-suite를 재사용하나 실패시 잘못된 ./cmd/issueops 패키지를 실행한다.
- steps.go:93-99 binary drift는 command runner exit만 읽는다. internal/adapter/verification/process.go:34-40은 일반 command 결과 OK=err==nil을 제공한다. 이 일반 계약을 바꾸지 않는다.
- 실제 golden owners는 cmd/issueops/contractgolden/contract_golden_test.go:18,22,26의 TestCLIUsageGolden/TestMCPToolsGolden/TestMCPResourcesGolden과 cmd/issueops/issueopsapp/response_contract_golden_test.go:19의 TestResponseContractsGolden이다.
- internal/adapter/doctor/observations.go:72-95는 root/bin/issueops를 관측한다. cmd/issueops/basiccli/inspect_doctor_cli.go:111-114의 JSON 조회는 unhealthy여도 exit 0이다. doctor의 조회 계약을 변경하지 않고 binary_drift 항목만 소비한다.
- 기존 StepResult와 doctor DTO, RunCommandStep 포트, full-suite reuse domain predicate, 기존 focused fixture를 재사용한다. 외부 프로세스 adapter를 새로 복제하지 않는다.

## 구현 순서

1. 현재 감사 fixture와 소스를 대조한다. 원본 근거는 source .issueops/evidence/audit-20260930/verification-* 및 verification-report.md에 있다. 안전한 임시 경로에 재현 로그를 보존하고 실제 테스트 실패를 먼저 추가한다.
2. Golden fallback은 두 실제 소유 패키지에서 위 네 named tests를 실행한다. go test -json의 test-level run/pass 이벤트와 package pass를 해석해 각 필수 테스트의 성공 실행을 요구한다. zero-test, skip, fail, malformed/truncated output은 성공 증거로 인정하지 않는다. 기존 runner output budget을 고려해 필요한 증거가 잘리지 않는 범위의 narrow run을 선택한다. success full-suite 경로는 추가 실행 없이 재사용한다.
3. Binary drift 단계는 성공적으로 종료한 doctor JSON의 checks 중 정확히 binary_drift 항목을 읽는다. 해당 healthy=false는 단계 실패이며 누락/중복/잘못된 JSON 또는 bool 누락은 명확한 진단으로 실패한다. 다른 체크의 healthy=false는 drift 실패로 삼지 않는다. 미빌드 바이너리에 대한 doctor의 healthy skip 계약을 그대로 인정한다. stale tempBin이라는 기존 주석은 실제 관측 대상 root/bin/issueops에 맞게 수정한다. 실행 자체 실패와 timeout은 원래 결과를 유지한다.
4. 실제 subprocess fixture로 golden no-match/failing test를 검증한다. 실제 doctor와 임시 root/bin mtime으로 stale/fresh/missing을 검증한다. 큰 전체 repository 복제를 피하고 작은 임시 모듈/디렉터리와 existing test helper를 사용한다. argv만 검증하는 mock은 보조 증거로만 둔다.
5. 변경한 판정 계약을 정규 testing 문서에 짧게 반영한다. #535는 CI 반복 검사 비용만, #536은 loop_contract metadata와 폐기 안내를 담당한다. steps.go의 Python/risk/full-test orchestration 및 loop.go metadata를 바꾸지 않는다. .github/workflows/ci.yml은 소유하지 않는다.
6. IssueOps implement → ai-slop-clean → docs → verify → create-pr → complete 절차로 진행한다. 독립 구현 리뷰와 최신 remote checks를 확인한 뒤 owner endpoint에서 멈춘다.

## 성능 영향

정상 full-suite 성공에는 golden subprocess를 추가하지 않는다. 실패 후 collect-all 진단에만 좁은 golden 실행과 JSON 파싱 비용이 든다. binary drift는 기존 doctor 출력 크기에 선형인 파싱만 추가하며 추가 doctor 실행은 없다. GOFLAGS=-p=2, GOMAXPROCS=4를 해당 세션 프로세스 환경에만 적용해 다섯 워크트리의 검사 경합을 줄인다. 전역 config/install은 변경하지 않는다.

## 하위 호환성과 side effect

CLI flag, StepResult shape, MCP schema, record schema 및 doctor exit code를 유지한다. 변경은 잘못된 OK 판정을 실패로 교정하는 것이다. 미빌드 바이너리 skip과 full-suite reuse를 유지한다. 생성물은 정규 명령으로만 갱신한다. 실패 JSON은 로그 예산 안의 진단을 제공하며 secret을 추가 기록하지 않는다. 새 durable state나 migration은 없다. 롤백은 해당 구현 커밋의 revert다. 원격 변경은 이 이슈의 draft PR 및 IssueOps 증거뿐이며 main merge/cleanup/update는 조정 세션이 맡는다.

## 수용 기준과 게이트

G1: 실제 fallback 및 drift 회귀 | CHECK: go test ./internal/application/selfverify ./cmd/issueops/selfworkflow/steps -count=1 | EXPECT: ok
G2: 실제 네 golden 테스트 실행 | CHECK: python3 /tmp/issueops-five-20261001/534/verify-golden.py | EXPECT: all four golden tests executed and passed

G2 verifier는 인계 자료에 포함된 실행 가능한 임시 스크립트다. 실제 go test -json exit 0, 네 named test의 run/pass 및 두 package pass를 모두 요구하고 fail/skip을 거부한다. 누락/skip fixture self-test도 스크립트에 있다. 이 스크립트는 제품 소스에 추가하지 않는다.

### PR 진입 전 최종 검증 절차

G1–G2만 게이트 원장에 등록한다. 최종 battery는 동일 revision/환경에서 gofmt 결과가 비었음을 확인하고, go vet ./..., go test -race ./... -count=1, ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json을 모두 실행한 단일 증거 bundle로 남긴다. 실행 자체의 exit, self-verify .ok 및 termination_eligible를 확인한다. 이미 같은 self-verify가 소유한 full-suite/build/docs/inspect를 별도 중복 실행하지 않는다. 필요한 로컬 bin/issueops 빌드는 battery 전에 수행한다. 필수 항목 하나라도 실패하면 완료 표시를 하지 않는다. 이 절차는 IssueOps verify가 소유하며 원장의 CHECK 텍스트로 변환하지 않는다.

### 게시 후 완료 확인 절차

PR 게시 후 latest pushed HEAD의 required CI 성공과 execution complete/released를 확인한다. 이는 최종 완료 기준이며 PR 진입 전에 채우는 원장에 넣지 않는다. 검증 증거와 PR/HEAD를 조정 세션에 전달한다.

Repo grounding: 위 경로와 실제 이슈 #534 본문 및 감사 증거를 대조했다.
Decision-complete plan: application 판정 교정과 focused regression fixtures로 범위를 제한하고 프로세스/doctor generic 계약을 보존한다.
Assumptions/defaults: main 봉인 base와 GPT-6.1 Sol high 구현자를 사용한다. native launch는 동일 worktree와 Codex bypass flag를 사용한다.
Unresolved questions: none blocking.
Acceptance criteria: G1–G2, PR 진입 전 최종 검증 및 게시 후 완료 확인 절차와 malformed/no-match/skip/stale/fresh/missing fixture 결과가 모두 충족된다.
