# 최신 자기 검증 상태 선택 검증 보고

- Lifecycle: `io-14e119b8fce9`, generation 2, branch `523-latest-selfverify-status`.
- Issue: https://github.com/m16khb-org/issueops/issues/523
- 범위: 상태 집계의 최신 적격 summary 선택, wiring, 회귀 테스트, 해당 architecture 문서.
- 승인된 종료점: 해당 브랜치 commit/push, Draft PR, execution complete. merge와 cycle cleanup은 제외한다.

## 의도 대조

| 수락 기준 | 관측 증거 |
|---|---|
| 생성 시각으로 최신 적격 실행을 선택하고 실패도 유지한다 | `TestRunSelectsLatestEligibleSummaryRegardlessOfOrder`의 8개 입력 순서와 임의 키의 최신 `ok:false` summary가 같은 key를 선택한다. |
| 후보 자료, 다른 kind, unsupported/missing schema, 일반 JSON을 제외한다 | 위 테스트 및 후보만 있는 목록에서 `found=false`를 확인했다. 후보 kind는 production의 `self_verification_candidate_export`다. |
| 기존 history 시각 정렬과 fallback을 유지한다 | Nano/offset, valid/invalid, 전부 invalid, updated_at fallback, key tie 회귀 테스트를 통과했다. |
| 시각 경고와 조회 오류를 구별한다 | 정상 최신 + invalid 시각이면 warning과 `OK=true`, 추가 state read 실패면 warning과 `OK=false`이며 기존 warning 순서를 유지한다. |
| 목록·추가 읽기 비용과 조회 무변이를 검증한다 | 목록 1회, record별 추가 read 1회, 빈 목록/목록 실패 시 추가 read 0회. `SaveSummary`를 사용한 실제 저장소·CLI 테스트가 전후 envelope를 비교한다. |

## 실행 증거

- 준비 세션 baseline: `TestEvaluateSelectsFirstSelfVerifyPrefixInSourceOrder` PASS.
- RED: `go test ./internal/application/status -run 'TestRunSelectsLatest|TestRunReusesHistory|TestRunHistoryReadFailure|TestRunSkipsHistory' -count=1`가 후보 선택·임의 키 누락·조회 오류 누락으로 실패했다. 전체 출력은 ignored `artifact/red.txt`에 보존했다.
- GREEN: application/domain/status, statuscli, root wiring focused tests PASS. 관련 history/domain tests, architecture fitness, response golden은 G1–G3 원장에 CLI 실행 결과를 기록했다.
- SURFACE: 빌드한 `bin/issueops system-status --json`이 임시 state에서 `custom-failed-run`을 선택하고 invalid 시각 warning을 반환했다. 조회 전후 모든 state envelope가 같았다. CLI fixture 초기 검사는 JSON 키를 `selfverify`로 잘못 적어 실패했고 실제 계약 `self_verify`로 정정하여 재실행했다. 제품 코드 변경 사유가 아니다.
- CLEAN: 테스트의 중복 정렬 계산을 제거하고 같은 G1–G3을 다시 통과했다. 최종 battery와 독립 리뷰의 정확한 명령·결과는 durable `implementation_review` 및 `execution.completion.verification`에 기록한다. 기록 이전에는 최종 PASS로 간주하지 않는다.

## 성능 영향

`BenchmarkRunHistoryReads`는 10개/1,000개 메모리 fixture에서 callback·decode·정렬 비용을 측정했다. 수정 전 0.57µs/29.9µs, 추가 read 0회였고 수정 후 63.3µs/6.33ms, 추가 read 10회/1,000회였다. 실제 disk/SQLite 지연을 뜻하는 수치는 아니다. 기존 `State.List`도 각 record를 읽으므로 그 내부 읽기에 추가로 O(n) read가 생긴다. 목록 중복 조회·retention·cache는 추가하지 않았다.

## 정리와 side effect

- 변경: status application은 state read와 기존 HistoryService를 조합하고 domain은 검증된 metadata를 투영한다. domain에 JSON·파일 I/O를 넣지 않았다.
- 유지: JSON envelope/응답 필드, latest_key/updated_at/bytes metadata, doctor/state/worker 조회 성공 의미. 최신 실행의 `ok:false`는 status 전체 OK를 바꾸지 않는다.
- state side effect: 새 write/delete/promotion 없음. 기존 state 목록 조회의 저장소 초기화 동작은 유지한다.
- 파일·원격 side effect: 구현/테스트/문서와 IssueOps 추적 자료를 해당 브랜치에 남긴다. Draft PR publication과 durable cycle 기록만 승인 범위 안에서 실행한다.
- QA cleanup: CLI smoke의 TemporaryDirectory가 제거됐고, Go fixture는 `t.TempDir`로 정리된다. 서버·background writer를 생성하지 않았다.
- Slop 측정: 변경된 tracked/untracked Go의 비어 있지 않은 추가 줄을 대상으로 한 shell 근사 SNR 0.9969 전후, boilerplate 비율 0.0685→0.0660. 주석 1개는 baseline 재저장의 이유를 설명하므로 유지했다. 품질 향상을 수치 이상의 의미로 주장하지 않는다.
- 범위 밖: scanner/reviewer effort/host defaults/source main은 수정하지 않았다. 구조 ADR·새 포트·migration은 없다.
- rollback: 해당 브랜치의 변경 커밋 revert. 데이터 migration이 필요하지 않다.

## 검증 방식

메인 에이전트가 구현·테스트·CLI QA를 직접 수행한다. 최종 독립 리뷰는 `devils-advocate-review` 패턴의 빈 context reviewer가 담당한다. 전체 battery와 read-only 리뷰를 같은 입력에서 병렬 실행하는 경우 `parallel_speed`이며, 최종 결과를 모두 관측한 뒤만 durable verdict를 기록한다.

UI/DB 실측/외부 LLM 평가는 해당 없음: 이번 범위에는 frontend, 스키마, 모델 호출 변경이 없다. deterministic selfverify는 명시적으로 `--llm-eval=false`를 사용한다.

## CI 문서 보정 (generation 3)

- 기존 generation 2 구현·검증 기록은 그대로 보존한다. 이번 실행은 추적 계획의 개인 홈 경로를 `$SOURCE_ROOT` 표기로 바꾸는 문서 보정만 수행한다.
- CI와 같은 meeting-notes 계약 unittest를 실행해 `plan.md`의 식별정보 탐지 실패를 재현했다. 실패 전체 출력과 실제 로컬 인계 자료는 ignored `artifact/`에 보존한다.
- 검사 코드는 수정하지 않는다. 이전 HEAD `c1abd4c2c6c84cb4abc5ec0a13f3854418cc344d`의 Go 구현·테스트 파일과 현재 파일의 내용 해시를 대조해 같음을 확인한다. 기존 구현 검증은 그 동일 파일에 대한 증거로 유지하며 새 실행 결과로 바꿔 적지 않는다.
- 보정 후 전체 Python unittest와 원격 CI 결과, 독립 문서 변경 리뷰, 새 HEAD는 최신 durable review·completion 기록에서 확인한다.
