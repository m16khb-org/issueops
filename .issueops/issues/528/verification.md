# #528 구현 검증

## 의도 대조

CI의 저장소 Python discovery 누락을 로컬 self-verify에서 해소했다. 실제 실패·성공,
missing/unsupported runtime의 조기 실패, 순서·점수·coverage·후보·역사 호환성을 확인했다.
검사와 개인정보 스캔 범위는 유지했다. source checkout, 설치본, bin 업데이트,
문서 경로 정규화와 무관 hostprobe 수정은 범위에 포함하지 않았다.

## 변경과 계약

기존 RunCommandStep을 재사용해 gofmt 다음에 Python script tests를 실행한다.
Python 3.10 미만은 discovery 전에 실패하며 같은 sys.executable로 CI argv를 실행한다.
버전 확인과 discovery의 timeout은 합쳐 5분이다. JSON 필드와 snapshot schema v1은 유지하고
검사 범위를 구분하는 summary contract만 v4에서 v5로 올렸다. 후보 계획은 현재 contract와
termination이 일치하는 성공 요약을 요구하며 후보 목록의 상태는 변경하지 않았다.
역사 요약은 계속 읽을 수 있고 Python label 추가·누락을 비교 결과에 표시한다.

## 검증 결과

- RED: 신규 Python 순서 테스트가 기존 risk QA 위치에서 실패했다. v4/hash mismatch와
  ineligible 요약도 기존 후보 계획 판정에서 잘못 통과하는 것을 재현했다.
- G1: selfverify application/domain, selfaugment application과 selfworkflow 전체 focused tests가 통과했다.
- G2: 실제 self-verify의 Python 3.14.6 discovery는 53 tests, 78.778초,
  OK (skipped=1)로 끝났다. 기존 platform 조건의 skip이며 이번 변경은 skip을 추가하지 않았다.
  이 run에서는 새 공개 자료가 untracked였으므로 그 자료의 개인정보 스캔 증거는 아니다.
  모든 변경을 exact-path stage한 뒤 CI와 같은 전체 Python suite를 별도로 실행한다.
  최종 tracked 집합과 결과는 ignored artifact/session/python-staged.log와 completion 영수증에 보존한다.
- G3: 임시 Git fixture와 실제 Python runner에서 pass/fail을 실행했다. missing과 통제된
  Python 3.9 runtime은 discovery marker를 만들지 않았고 긴 risk QA도 호출하지 않았다.
  실패 시 step OK, 전체 OK, summary termination이 모두 false였다. fixture는 테스트 종료 때 제거됐다.
- G4: 같은 소스·환경의 단일 최종 battery가 757.583초에 성공했다. gofmt, build,
  darwin 및 GOOS=linux golangci-lint와 self-verify를 실행했다. self-verify는 27단계 모두 성공,
  모든 14개 목표 100점, coverage gaps 0이었다. risk QA에서 전체 go test -race와 go vet을
  실제 실행했고 그 성공한 전체 suite를 go test와 golden 증거로 재사용했다.
- 역사 계약: v4와 v5의 hash가 달랐고 JSON 필드 목록은 같았다. v4→v5 비교는 Python label을
  added로, 역방향은 missing regression으로 보고했다. v4 요약은 현재 완료 근거로 거부됐다.
- API 문서 preflight: 이번 변경에는 API candidate files가 없었다.
- G5: Draft PR과 최신 push/PR CI, exact HEAD verify-artifact 및 generation 반납은
  publication 이후 lifecycle의 remote_artifact와 execution completion 영수증에 기록한다.

## 재실행 명령

$WORKTREE에서 Python 3.10+를 PATH에 두고 process-local GOFLAGS=-p=2,
GOMAXPROCS=4를 사용한다. 기존 인계 baseline은 다시 실행하지 않았다.

```bash
go test ./internal/application/selfverify ./internal/domain/selfverify ./internal/application/selfaugment ./cmd/issueops/selfworkflow/... -count=1
go test ./internal/application/selfverify -run TestPythonDiscoveryRuntimeAndEarlyFailure -v -count=1
gofmt -l $(git ls-files '*.go')
golangci-lint run ./...
GOOS=linux golangci-lint run ./...
go build -o "$WORKTREE/.issueops/issues/528/artifact/session/issueops" ./cmd/issueops
ISSUEOPS_ROOT="$WORKTREE" "$WORKTREE/.issueops/issues/528/artifact/session/issueops" self-verify --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
```

self-verify의 risk QA가 vet/race를 실행하지 않은 환경에서는 누락된 명령을 같은 battery에
추가한다. 실패하면 첫 항목부터 다시 실행하며 부분 성공을 합치지 않는다.
실제 runtime 경로와 원문 JSON/log는 ignored artifact/session에만 보관했다.
게이트 G2/G4는 이 최종 bundle을 읽으며 새 discovery나 긴 Go suite를 중복 실행하지 않는다.

## 정리와 측정

최종 diff에서 dead code·중복·불필요한 abstraction·경계 위반·약한 자료·근거 없는 주장을
검토했다. 추가로 제거할 slop은 없었다. 긴 인수 절차와 동작 fixture는 해당 계약의 증거라 유지했다.
변경된 Go 파일 전체와 untracked 테스트를 포함한 shell 근사치: nonblank 1398줄,
comment 11줄, SNR 0.9921, 중복 nonblank 440줄(31.47%), comment/boilerplate 근사 0.79%다.
정리 전후 값은 같으며 AST 분석이나 실제 dead-code 비율을 뜻하지 않는다.
빈 입력이면 비율을 계산하지 않는다. 문서 파일은 코드 지표에서 제외했다.

## 성능 영향과 side effect

Python step은 78,927ms로 인계 73.498초 baseline에 비해 약 5.4초 늘었다. 서로 다른 revision의
단일 관측이므로 성능 회귀로 단정하지 않는다. 일반 CLI hot path에는 Python 실행이 없다.
후보 계획은 기존 한 번의 snapshot read를 유지하며 현재 contract hash 비교만 추가한다.

변경은 $WORKTREE의 코드·테스트·문서·이슈 자료, ignored binary/log/bytecode,
IssueOps durable 기록과 승인된 issue branch/Draft PR에 남는다. 별도 dependency 설치,
데이터 마이그레이션은 없다. 롤백은 PR revert이며 merge/cleanup은 root coordinator가 맡는다.

Durable state record: io-0b57c36d0e6f, direct generation 2의 native owner.
Phase routing: plan → compatibility-review → implement → ai-slop-clean → feedback → pr → done.
Flow evidence: 봉인 plan/intent/계획 리뷰, RED→GREEN, 실제 Python fixture, 단일 battery와 gates.md.
Hook boundary: hook은 project-doc context만 제공하며 구현·검증·publication은 owner/CLI가 수행한다.
Cleanup/readiness evidence: fixture는 제거됐고 runtime 부산물은 ignored 경로에만 남는다.
