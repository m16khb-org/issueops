# I2 재사용 표시와 duration 표본 수 구현 증거

## 결과와 실행 환경

할당된 재사용 표시·표본 통계와 deterministic 회귀 테스트를 구현했다.
지정된 다섯 패키지의 마지막 단일 실행은 exit 0이다. 아래 통합 우려 때문에
전체 저장소·호스트 검증 완료를 주장하지 않는다.

- 실제 환경: `PI_MODEL=gpt-6.1-sol`, `PI_REASONING_LEVEL=high`.
  요청한 high와 일치하며 환경·모델 설정을 변경하지 않았다.
- `contract.md`의 확정 결정을 따랐다. selfverify contract는 v7,
  summary snapshot envelope는 v1이다.
- parent가 인계한 네 probe의 duration 수정과 `validation_duration_test.go`를
  수정하지 않았다. 기존 command duration 137/149/163 ms와 worker의
  11+13+17+19=60 ms 회귀를 이번 package GREEN에 포함했다.
  probe 전체 wall-clock 측정이나 새 clock을 추가하지 않았다.
- commit/push, HOME 설치, config/golden 변경, sleep, 테스트 약화는 하지 않았다.

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/contract/selfverify/step.go` | `reused` bool을 true/false 모두 직렬화 |
| `internal/application/selfverify/steps.go` | race→full-suite, full-suite→golden 두 재사용 경로에 표시 |
| `internal/application/selfverify/self_verify_summary.go` | summary 변환에서 Reused 보존 |
| `internal/domain/selfaugment/summary.go` | pass/fail·labels 보존, reused를 slowest와 duration 표본에서 제외 |
| `internal/domain/selfaugment/step_stats.go` | 명시적으로 존재하는 빈 표본 label에 count=0 통계 생성 |
| `internal/contract/selfaugment/history_summary_types.go` | `reused_count` 추가 |
| `internal/domain/selfverify/contract.go` | contract v6→v7, hash 갱신 |
| `internal/domain/selfverify/contract_test.go` | v6/v7 hash·version 경계 회귀 |
| `internal/domain/selfaugment/summary_test.go` | mixed/reused-only, 실제 0 ms, 실패 표본, slowest 제외, v1/legacy 읽기 |
| `internal/application/selfverify/summary_test.go` | JSON StepResult→summary의 재사용 표시 전달 회귀 |
| `internal/application/selfverify/steps_test.go` | fresh full-suite 표시, 실패 golden child의 duration·오류 보존 |
| `internal/application/selfverify/python_history_test.go` | 새 v7에 맞춰 machine-consumed version assertion 갱신 |
| `cmd/issueops/selfworkflow/steps/self_verify_steps_test.go` | 기존 실행·재사용 테스트에 serialized marker assertion 추가 |
| `cmd/issueops/selfworkflow/steps/self_verify_reuse_fixture_test.go` | 임시 fixture에서 실제 loop/summary entry point와 snapshot 직렬화 실행 |

혼합 label은 실측 137 ms 하나와 reuse 하나일 때 count=1, reused_count=1,
min/max/average/p95=137이다. reused-only label은 count=0, reused_count>0,
duration 지표 0이며 그 0을 측정 표본으로 세지 않는다. 실제로 실행된 0 ms와
실패한 측정 결과는 각각 하나의 duration 표본이다. count=1의 p95는 단일 표본이지
장기 실행의 성능 추정치가 아니다.

## RED / GREEN / build

모든 Go 실행은 아래 PATH를 사용했다.

```sh
export PATH=/tmp/issueops-ten-improvements-01a0fa31/venv/bin:$PATH
```

RED (`mon_14JEBR4907F0H60K`, **exit 1**):

```sh
go test ./internal/application/selfverify ./internal/domain/selfaugment \
  ./internal/domain/selfverify ./cmd/issueops/selfworkflow/steps \
  -run 'Test(SummarySeparatesMeasuredDurationsFromReusedEvidence|SummaryCarriesReusedMarkersWithoutLosingCoverage|SummarySnapshotKeepsV1AndReadsLegacyDurationEvidence|ContractV7FencesLegacyTimingEvidence|PlannedSelfVerifyStepsUsesCachedContractGoldenAfterGoTest|PlannedSelfVerifyStepsUsesSuccessfulRaceAsFullTestEvidence|CachedContractGoldenStepFallsBackWhenGoTestDidNotPass|CachedContractGoldenStepUsesFullGoTestEvidence)$' \
  -count=1 -v
```

표시가 없는 JSON, reused-only count=2, 혼합 count=2·p95=9000,
재사용 결과를 포함한 slowest 목록, v6와 같은 contract hash가 실제로 실패했다.
v1 legacy snapshot control은 이 RED에서도 통과했다.

첫 package GREEN 시도 (`mon_YW1ACFZKKGMWPGXV`, **exit 1**)는
`TestPythonContractDistinguishesHistoricalEvidenceWithoutChangingSnapshotSchema`의
기존 `Version != 6` assertion에서 실패했다. 재사용·통계 회귀와 나머지 패키지는
통과했다. 변경된 계약에 맞춰 이 assertion만 7로 갱신했다.
두 실패 실행의 전체 stdout/stderr는 monitor/session 도구 전사에 보존했다.

최종 package GREEN (`mon_B2N8J49R5R3FGRXM`, **exit 0**):

```sh
go test ./internal/adapter/verification/probe/contractauditworker \
  ./internal/application/selfverify ./internal/domain/selfaugment \
  ./internal/domain/selfverify ./cmd/issueops/selfworkflow/steps -count=1
```

```text
ok issueops/internal/adapter/verification/probe/contractauditworker 0.290s
ok issueops/internal/application/selfverify 10.726s
ok issueops/internal/domain/selfaugment 0.117s
ok issueops/internal/domain/selfverify 0.154s
ok issueops/cmd/issueops/selfworkflow/steps 0.121s
```

CLI build (`mon_DVHTD2XQ1RHJN28P`, **exit 0**):

```sh
go build -o /tmp/issueops-i2-build.VfLlMD/issueops ./cmd/issueops
```

변경 Go 파일 14개 각각의 LSP 검사에서 error/warning은 0건이었다.
`step_stats.go`의 기존 sort/minmax modernization hint 2건은 그대로 남겼다.
인계된 probe 패키지의 error 진단도 0건이다.
변경 파일 전부를 명시한 `gofmt -l`은 출력 없이 exit 0,
`git diff --check`도 exit 0이다. tracked diff와 새 테스트 두 파일을 모두
검토했으며 source 변경은 위 파일에만 있다.

## 임시 fixture 사용 증거

`mon_ENJS1DK7VDX8B47X`, **exit 0**:

```sh
go test ./cmd/issueops/selfworkflow/steps \
  -run '^TestSelfVerifyReuseFixtureEntryPoint$' -count=1 -v
```

`t.TempDir()`에서 `ExecuteLoop`→`PlannedSteps`→`SummarizeSelfVerification`을
실행했다. 외부 probe 실행만 fixture로 주입했다. coverage 재사용, scoring,
통계, JSON 직렬화는 실제 구현을 사용했다. covered command가 다시 실행되면
즉시 실패한다. 임시 verification 디렉터리 제거도 확인했다.

관측 JSON의 핵심 값:

```json
{
  "contract_version": 7,
  "snapshot_schema": 1,
  "passed_steps": 28,
  "step_duration_stats": [
    {"label": "risk QA tier", "count": 1, "reused_count": 0, "p95_duration_ms": 137},
    {"label": "go test", "count": 0, "reused_count": 1, "p95_duration_ms": 0},
    {"label": "contract golden tests", "count": 0, "reused_count": 1, "p95_duration_ms": 0}
  ]
}
```

28개 pass는 fixture의 orchestration 결과이며 실제 전체 저장소 테스트나
네이티브 호스트 통과 수가 아니다. 기존 baseline 재사용 경로인
`internal/application/selfaugment/planner.go:50-54`는 contract name/version/hash를
대조하므로 v6 증거를 v7의 현재 성공 증거로 인정하지 않는다.

## 남은 통합 우려

1. **범위 밖 v6 assertion.** 아래 검사는 `mon_ETH0V41V2HP2RYH2`에서
   **exit 1**이다. `cmd/issueops/selfworkflow/summary/self_augment_summary_test.go:118`
   은 여전히 version 6을 기대하고 실제 출력은 version 7이다. 쓰기 허용 범위가
   아니므로 수정하지 않았다.

   ```sh
   go test ./cmd/issueops/selfworkflow/summary \
     -run '^TestSelfVerificationContractIncludesSummaryExtensions$' -count=1 -v
   ```

2. **cross-version duration 비교 경계.**
   `internal/domain/selfaugment/history_compare.go:18-19,50-54`는 snapshot의
   contract version/hash 호환성을 확인하지 않고 slowest/p95를 비교한다.
   현재 summary 생성은 reused를 제외하지만, legacy v6와 v7 snapshot을 직접
   비교하는 경로까지 차단하지는 않는다. 계약의 "v6/v7 duration series를
   합치지 않는다" 요구를 완료하려면 통합 소유자가 이 범위 밖 comparator의
   version 경계를 구현해야 한다. legacy duration의 reuse를 추정하는 fallback은
   추가하지 않았다.

3. **shared response golden 및 문서 게이트.**
   `cmd/issueops/testdata/response_contracts.golden.json`의 새 serialized 필드와
   v7 hash projection 갱신은 통합 소유자 작업이다. golden을 변경하지 않았다.
   전체 API 문서 게이트, whole-repo vet/race/test와 실제 CLI self-verify 및
   Codex/Claude/Omo 검증은 이번 할당 범위에서 실행하지 않았다.
