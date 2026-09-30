# 자기 검증 Python 검사 정합성 구현 계획

Lifecycle: `io-0b57c36d0e6f`. Issue: https://github.com/m16khb-org/issueops/issues/528
Branch: `528-selfverify-python-parity`. Base: main. `$SOURCE_ROOT`는 source checkout, `$WORKTREE`는 준비 결과의 canonical checkout을 가리킨다.
사용자의 “진행해봐” 요청에 따라 제안한 검증 누락 수정을 수행한다. direct prepare 뒤 Orca가 ready이면 같은 worktree의 새 Codex 세션으로 인계한다. 승인된 종료점은 Draft PR, push/PR CI 성공, exact HEAD의 artifact verification과 execution complete/released다. merge와 cleanup은 root coordinator가 소유한다.

## 확인한 문제와 범위

`internal/application/selfverify/steps.go:51`은 26단계를 구성하고 `internal/domain/selfverify/step_plan.go:3`의 순서를 적용하지만 Python 검사가 없다. `.github/workflows/ci.yml:104`는 `python3 -m unittest discover -s scripts -p '*_test.py'`를 실행한다. `internal/domain/selfverify/contract.go:50`의 test_suite 목표와 coverage 역시 Go 검사만 포함한다.

범위는 selfverify application/domain, 필요한 composition/adapter wiring, 해당 테스트·golden 및 자기 검증 운영 문서다. 검사 약화, 개인정보 스캔 축소, 무관 hostprobe flaky 수정, 전역 설치 변경은 제외한다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제3장과 `.issueops/architecture/hexagonal-core.md`의 dependency direction: domain은 순수 단계·점수 정책만 소유하고 process 실행은 기존 경계로 처리한다.
- `.issueops/CONVENTIONS.md` 핵심 불변식: 기존 port와 composition 경계를 유지하고 host별로 Python 실행을 복제하지 않는다.
- `.issueops/ADR.md` Accepted baseline의 standalone verification: 외부 계정·별도 서비스·패키지 설치를 readiness 조건으로 추가하지 않는다. Python은 저장소가 이미 실행하는 검사 도구이며 자동 설치하지 않는다.
- `.issueops/CAUTIONS.md` 로컬 battery/CI 정합성과 `cautions/lessons/2026-08-26-ci-gofmt-gate-local-battery-drift.md`: CI 테스트를 로컬 완료 증거에서 빠뜨리지 않는다.
- `.issueops/testing/self-verification.md` 최종 battery: 같은 revision·환경의 단일 성공 run을 보존하고 실패 후 첫 게이트부터 재실행한다. Go risk 단계가 실제 수행하지 않은 vet/race는 별도 수행한다.
- 공개 plan·intent·검증 문서에는 개인 홈 경로를 쓰지 않고 변수 또는 저장소 상대경로만 쓴다.

## 재사용하는 기존 구현

`SelfVerifyStepDeps.RunCommandStep`의 timeout·출력 캡처·오류 전파를 재사용한다. 새 일반 process runner나 Python 도구 탐색 프레임워크를 만들지 않는다. domain의 `StepOrder`, `GoalDefinitions`, `CoverageDefinitions`를 확장하며 summary의 기존 실패 판정을 이용한다. 기존 step plan 테스트가 숫자 위치에 의존하므로 새 순서에 맞게 고친다.

## 구현 순서

1. Python runtime과 전체 discovery를 현재 환경에서 실측한다. 기존 publish helper는 `list[str] | None` 런타임 annotation을 사용하므로 Python 3.9에서 실패한다. 지원 최소 버전은 전체 테스트와 import 경로를 확인해 3.10 이상으로 정하되 실제 추가 요구가 있으면 증거로 상향한다. CI의 버전도 확인하고 문서에 근거를 적는다.
2. 실패 테스트부터 작성한다. 신규 `Python script tests` 단계가 CI와 동일한 discovery argv를 정확한 repo root에서 호출하고 실패·성공이 summary/termination에 전파되는 RED를 만든다.
3. 긴 risk QA/Go 테스트 앞에 신규 단계를 둔다. Python executable 누락은 기존 runner 오류를 명확한 단계 실패로 보여주고, 비호환 버전은 짧은 버전 검사로 빨리 실패시킨다. 같은 python3 실행 경로를 버전 검사와 discovery에 사용한다. 실행은 timeout과 기존 출력 budget을 따르며 버전 실패 후 discovery는 실행하지 않는다. 성공한 discovery 한 번만 완료 근거로 인정한다.
4. 신규 단계 label을 test_suite 목표와 coverage에 포함한다. 기존 JSON 필드와 record 형식은 보존하고 contract hash/version·후보 export·baseline 비교의 의미를 확인한다. 검사 범위가 달라진 구버전 요약을 현재 완전 검사로 인정하지 않는다. 필요한 golden만 실제 결과에 맞춰 갱신하며 후보 상태를 근거 없이 satisfied로 바꾸지 않는다.
5. `.issueops/testing/self-verification.md` 등 정규 문서에 실행 명령, 지원 runtime, 실패 진단, 단계 시간 비용을 반영한다. source 자동생성 경로 정규화는 다른 cycle 책임이다.
6. 정리→문서 검토→현재 next.review profile 독립 구현 리뷰→단일 최종 battery→commit/push→Draft PR과 CI 확인→완료를 수행한다.

## 성능 영향

self-verify에만 저장소 Python 테스트 한 번과 짧은 버전 확인 비용이 추가된다. 일반 status/CLI hot path는 바꾸지 않는다. discovery 전후 elapsed를 동일 revision에서 측정하고 신규 StepResult DurationMS와 함께 기록한다. 테스트는 추적 파일을 검사하므로 비용은 파일량에 비례한다. CI와 동일한 전체 범위를 유지하고 캐시/샘플링/skip으로 비용을 줄이지 않는다. 초기 timeout은 5분으로 두고 실측이 이를 넘으면 원인을 확인한 뒤 근거 있게 조정한다. 긴 Go 검사는 process-local GOFLAGS=-p=2, GOMAXPROCS=4로 동시 부하를 제한한다.

## 하위 호환성과 side effect

명령 플래그와 JSON 필드, persistent record schema를 유지한다. step 수와 labels, score/coverage contract 변화는 의도한 additive 변경이며 역사 요약·후보·비교 테스트로 구분한다. Python 없는 환경은 성공이 아니라 명시적 실패가 되어야 한다. 새 global config나 dependency 설치는 없다. Python bytecode는 ignored runtime 부산물로만 남으며 tracked 변경에 포함하지 않는다. 롤백은 이 PR revert이고 저장 데이터 마이그레이션은 없다. LLM 프롬프트·DB schema를 변경하지 않는다.

## 수락 기준과 게이트

- G1: Python fail/pass·runtime 오류·순서·score 계약 | CHECK: go test ./internal/application/selfverify ./internal/domain/selfverify -count=1 | EXPECT: ok
- G2: CI와 같은 저장소 Python 검사 | CHECK: python3 -m unittest discover -s scripts -p '*_test.py' | EXPECT: OK
- G3: 실제 신규 selfverify 단계와 실패 전파를 격리된 임시 Git fixture에서 실행한다. 실패하는 *_test.py를 실제 Python으로 실행해 step OK=false, summary OK=false, termination=false를 확인하고 통과 fixture에서 true를 확인한다. missing executable과 설치된 구버전 또는 통제된 interpreter fixture로 조기 실패와 discovery 미실행도 확인한다. wiring mock만으로 대체하지 않는다.
- G4: 최종 gofmt/vet/race와 self-verify JSON battery를 한 revision·환경에서 통과시킨다. self-verify가 수행한 전체 Go test/build/docs/inspect는 중복 책임으로 실행하지 않는다. 최종 모든 단계와 minimum score를 실제 JSON에서 읽는다.
- G5: 최신 push·PR CI 성공, exact HEAD artifact verification, complete/released를 확인한다. PR 게시 후에도 CI 확인 전 완료라고 보고하지 않는다.

Repo grounding: 위 source·CI·project docs를 직접 읽었다.
Decision-complete plan: 기존 실행 경계를 재사용한 초기 Python 단계와 domain 검사 범위를 추가한다.
Assumptions/defaults: 지원 runtime은 테스트의 실제 요구사항에 근거하며 외부 도구 자동 설치는 없다.
Unresolved questions: blocking 없음.
Acceptance criteria: G1~G5 및 no weakening 조건.
