# #535 검증 보고서

## 결과와 판정 조건

CI의 독립 Python discovery와 일반 Go 전체 테스트 단계를 제거했다. 임시 HOME에서
실행하는 기존 self-verify가 두 검사를 소유한다. build와 독립 race, native 설치 및
integration, shell 실패 전파는 유지한다. self-verify production 코드는 바꾸지 않았다.

완료 판정은 이 문서의 설명만으로 내리지 않는다. 동일 구현 revision의 단일 전체
self-verify 성공 결과, 독립 diff 리뷰, 최신 push·PR CI 성공 결과와 execution completion
receipt를 함께 확인한다. 최종 HEAD와 PR URL은 해당 cycle의 durable completion과
`/tmp/issueops-five-20261001/535/owner-result.json`에 기록한다. 발행 후 CI 실측 표는
PR 본문에 남긴다. 이전 baseline과 실패 run은 완료 근거로 사용하지 않는다.

## 의도 대조

| 성공 기준 | 이 변경의 확인 방법과 증거 |
| --- | --- |
| 실행 책임과 환경 차이를 명시한다 | `.issueops/testing/self-verification.md`의 CI 검사 소유권 표를 실제 workflow와 대조한다. |
| 동일 검사의 중복을 줄이고 다른 SHA·dirty·환경의 결과를 가져오지 않는다 | 독립 두 단계를 제거한다. 새 skip/cache/import 경로는 없다. workflow fixture의 성공 이벤트는 build/race/install/self-verify/Python/Go/golden/native 각 1회다. |
| race·임시 HOME native·실패 전파를 유지한다 | 실제 workflow 블록을 disposable fixture에서 실행해 성공 및 Python/Go/race/install 실패를 관측한다. 설치와 self-verify는 같은 HOME/CODEX_HOME을 사용한다. |
| 독립 local self-verify와 실제 실패 처리를 유지한다 | 기존 selfverify 패키지 회귀 테스트 및 단일 전체 self-verify의 Python/Go/golden/native step 결과를 확인한다. |
| 같은 CI 조건의 호출 수와 단계별 시간을 비교한다 | main baseline과 최신 feature push/PR CI의 Ubuntu runner/toolchain, 실행 횟수, 단계 시간 및 self-verify DurationMS를 실제 job 로그에서 읽는다. 날짜·부하·cache 차이 때문에 고정 절감률을 주장하지 않는다. |

G1과 G2는 gates.md의 실행 결과가 증거다. G3는 전체 self-verify JSON의 `ok`,
`termination_eligible`, 빈 `coverage_gaps`를 검사한다. 원격 CI 항목은 발행 후 확인하며,
최신 두 event의 head SHA가 최종 HEAD와 같고 race와 self-verify가 성공해야 완료한다.
UI·DB·API schema 변경은 없다.

## 변경 범위

- `.github/workflows/ci.yml`: 독립 두 단계를 제거하고 `--llm-eval=false`를 명시한다.
- `scripts/ci_workflow_test.py`: 실제 CI shell 연결과 임시 HOME 정리/실패 전파를 검증한다.
- `.issueops/testing/self-verification.md`: 검사별 소유자·환경·횟수를 문서화한다.
- `internal/architecture/ddd_responsibility_test.go` 및 생성 inventory: 새 script를 기존 T20 실행 script 책임에 등록한다. 첫 전체 검사가 등록 누락을 발견했으므로 필요한 test-only 등록을 추가했다.
- `.issueops/issues/535/`: intent/plan/review/gates와 이 보고서를 커밋한다.

#534 소유의 steps.go와 #536 소유의 loop.go는 수정하지 않는다. source checkout과
다른 worktree를 수정하지 않는다. 새 dependency, CLI flag, JSON schema 변경은 없다.

## 검증 방식과 실행 근거

Success criteria: 위 표의 다섯 기준을 이진 결과와 실제 CI 관측으로 확인한다.
Evidence artifact: `/tmp/issueops-five-20261001/535/`의 RED/GREEN, 전체 battery, 독립 리뷰와 CI job log 및 해석 JSON.
Cleanup receipt: fixture의 ambient HOME sentinel 보존과 EXIT trap 정리를 assertion으로 확인한다. native QA의 임시 HOME 삭제 및 battery 자손 종료를 별도 관측한다.
Verification mode: 전체 loop. 실행 연결과 실제 native integration 및 원격 CI를 함께 확인한다.
Skipped checks: 요구된 검사를 줄이지 않는다. API/UI/DB 검사는 해당 변경이 없어 적용되지 않는다.

- RED: 기존 workflow에서 독립 Python 호출 수 1과 추가 Python/Go 이벤트 때문에 새 검사가 실패했다(`red.log`).
- GREEN: 두 단계를 제거한 workflow에서 두 test와 성공/네 실패 subcase가 통과했다(`green.log`).
- Focused: `go test ./internal/application/selfverify -count=1`, `go test ./internal/architecture -count=1`이 통과했다. 후자는 script 등록 및 생성 inventory와 source 일치를 확인했다.
- Final battery: root의 `serialized-verification.py --issue 535`가 단일 전체 실행을 소유한다. `final-battery-metadata.json`에 명령·종료 코드·시간·환경을 남기고 `final-selfverify.json`에 전체 step과 coverage를 남긴다. gates G3는 이 run의 전체 성공만 받는다.
- 실패 이력: 첫 run의 architecture artifact 등록 누락을 수정했다. 두 번째 run은 병렬 부하 중 기존 risk QA 600초 한도에 걸렸다. 각각 `failed-battery-1`, `failed-battery-2`에 보존했다. timeout과 테스트 집합을 바꾸지 않고 root 승인에 따라 무거운 battery만 직렬화한다.
- 독립 리뷰: clean/docs 봉인 뒤 fresh-context 리뷰가 계획 검토의 주장과 전체 diff를 확인한다. 결과는 `implementation-review` durable record와 외부 리뷰 evidence에 기록한다.

## 성능 측정

baseline은 비교 전용 run 36694854078이다. Python discovery는 독립 92초와
self-verify 내부 92.810초로 2회, 일반 Go는 독립 72초와 내부 122.495초로 2회였다.
독립 race는 239초로 1회였고 self-verify CI 블록은 224초였다. runner는 Ubuntu 24,
Go 1.26.3이었다. feature run에서 Python/일반 Go 각각 1회, race 1회인지 job log와
self-verify 결과를 함께 세고 PR 본문에 실제 시간을 비교한다. 다른 HOME의 결과를
재사용해서 얻는 절감으로 설명하지 않는다. 런타임 hot path에는 변화가 없다.

## Side effect와 정리

CI에서 Python/Go 실패가 발견되는 위치가 self-verify 내부로 이동한다. 검사 자체와
실패 종료 코드는 유지된다. race 실패는 독립 CI 단계에서 발견된다. 임시 HOME의
native 설치는 CI와 local QA의 기존 환경에만 남고 EXIT trap/TemporaryDirectory가
성공·실패 후 제거한다. 전역 install/update는 수행하지 않는다.

같은 self-verify run에서 성공한 risk QA race를 일반 Go/golden 증거로 재사용하는
기존 dirty-Go 계약은 유지한다. 기본 HOME의 CI race와 임시 HOME self-verify 사이에
새 재사용 경로를 만들지 않는다. build는 설치용 binary와 self-verify temp binary의
서로 다른 목적 때문에 각각 유지한다. rollback은 이 PR revert다.

## 정리 측정과 범위

CI 호출 중복을 제거했다. 테스트 fixture는 실제 shell 경계 확인을 위해 남겼다.
새 Python script의 비어 있지 않은 줄은 102, 여섯 줄 중복 블록은 0, comment/import
기반 SNR 추정은 0.980392, import 비율은 0.088235다. AST branch는 parser 3,
소유권 test 0, 실행 test 8이다. 정리 전후 script는 같으므로 품질 수치 개선을
주장하지 않는다(`quality-before.json`, `quality-after.json`). 범위 밖 정리는 하지 않았다.

독립 review 패턴은 `devils-advocate-review`다. 봉인 뒤 리뷰와 focused evidence 읽기만
함께 진행하며, 무거운 battery는 root의 직렬 lock을 따른다. 병렬 검증으로 절약한
시간은 측정하지 않았으므로 수치를 만들지 않는다.
