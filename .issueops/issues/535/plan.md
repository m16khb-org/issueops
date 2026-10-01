# #535 CI와 self-verify 검사 책임 정리

## Context
Cycle: io-1fa62d908157. Issue: https://github.com/m16khb-org/issueops/issues/535.
Base: main b917fc960b72bca3e616df1162c6b89d40a517ba. Branch: 535-ci-verification-cost.
최신 사용자 요청은 5개 이슈 병렬 인계부터 검증, main 병합, cleanup, io-update까지다. 구현 owner는 gpt-6.1-sol/high의 새 Orca Codex 세션이며, 자신의 Draft PR·최신 push/PR CI 성공·execution complete/released까지 담당한다. main 병합·정리·전역 update는 root coordinator가 담당한다. 이전 audit-only 제한은 최신 승인으로 대체됐다.

### Gap Analysis
현재 CI의 일반 환경과 임시 HOME은 다르다. 환경이 다른 결과를 재사용하지 않는다. Python/일반 Go 검사의 유일한 실행 소유자를 이미 필요한 임시 HOME self-verify로 정한다. race는 독립 CI 단계로 유지한다. 새 skip 옵션·캐시·증거 수입 경로는 만들지 않는다. #534는 steps.go의 golden fallback/binary drift, #536은 loop_contract 메타데이터를 소유하므로 이 파일들을 수정하지 않는다.

## 적용되는 결정과 주의사항
- `.issueops/CONSTITUTION.md` 제2장: 측정한 병목만 최적화하고 정확성을 성능보다 우선한다.
- `.issueops/ARCHITECTURE.md` 의존 방향 불변식: 이번 변경은 CI 실행 연결에 한정되며 core 정책을 호스트 adapter에 복제하지 않는다.
- `.issueops/CONVENTIONS.md` 생성물/dependency 규칙: 새 dependency나 범용 실행 프레임워크를 추가하지 않는다.
- `.issueops/CAUTIONS.md` 로컬/CI 검사 집합: 검증을 삭제하거나 clean checkout과 다른 환경 증거를 혼합하지 않는다.
- `.issueops/TESTING.md` 및 `testing/self-verification.md` 최종 검증 battery: 동일 revision·환경·입력의 전체 성공 run 하나만 완료 증거로 인정한다. self-verify가 소유한 test/build/golden/docs/inspect를 별도 최종 책임으로 중복 수행하지 않는다.
- `.issueops/ADR.md` External integrations baseline: 검증은 외부 companion 도구 없이 독립 실행 가능해야 한다.

## 재사용하는 기존 구현
- `.github/workflows/ci.yml:96-120`: 독립 Python, build, 일반 Go, race, 임시 HOME install+self-verify 순서를 확인했다. Build는 설치 전 binary 준비를 위해 유지한다.
- `internal/application/selfverify/steps.go:67-89`: self-verify 자체가 Python discovery, 일반 Go, full-suite golden 포함 관계를 이미 소유한다. 이 구현을 그대로 호출한다.
- `.issueops/testing/self-verification.md`의 battery 단일 소유 규칙과 실패 전파를 사용한다.
- 감사 근거 `.issueops/evidence/audit-20260930/ci-issue.md`, `main-ci.log`: 성공 run 36694854078에서 앞선 Python 약92초/일반Go 약72초, self-verify 내부 Python92.810초/Go122.495초. 두 환경이 다르므로 215.305초를 절감 예측값으로 쓰지 않는다.

## 성능 영향
런타임 hot path에는 변화가 없다. CI의 Python discovery와 일반 Go full suite의 호출 수를 각각 2회에서 1회로 줄인다. self-verify의 dirty-state risk tier가 race를 재사용할 수 있지만 clean CI의 독립 race는 유지한다. 같은 runner OS/toolchain/workflow 조건의 baseline run과 변경 run에서 호출 수, 단계별 시간, self-verify JSON DurationMS를 기록한다. 실행 시각·부하 차이를 설명하고 고정 절감률을 약속하지 않는다.

## 하위 호환성과 side effect
CLI/MCP JSON, step labels, record schema, golden 계약, standalone local self-verify 동작은 바꾸지 않는다. 검증 결과를 외부에서 수입하거나 skip하는 경로가 없으므로 SHA/dirty/환경 불일치 결과가 성공 근거로 들어갈 새 경로도 없다. 과거 로그는 비교 통계로만 쓰고 현재 gate 증거로 쓰지 않는다. race의 HOME은 기존대로이며 native integration은 임시 HOME에서 실제 설치 후 검증한다. 워크플로우 실패는 기존 shell exit 및 self-verify exit로 전파한다. 롤백은 이 PR revert다.

## 구현 작업
1. RED: 임시 fixture에서 현재 workflow의 중복 실행과 기대 소유 관계를 관측한다. 작은 repo-owned Python workflow 검증 테스트를 추가해 Python와 일반 Go 독립 단계 부재, race/build 유지, 임시 HOME 설치+self-verify의 동일 환경, 실패 전파를 검증한다. 테스트는 단순 줄수 대신 실제 CI run blocks를 추출해 disposable fixture/fake command boundary로 실행하여 Python·Go·race 실패 각각 nonzero임을 관측한다. source checkout/전역 HOME은 쓰지 않는다.
2. GREEN: `.github/workflows/ci.yml`에서 독립 Python discovery와 일반 Go test 단계를 제거한다. 남는 self-verify 단계가 해당 검사를 소유함을 짧은 주석으로 명시한다. 독립 race, build, format/lint/skill validation, 임시 HOME 설치 및 native integration은 그대로 보존한다. self-verify command에는 표준 deterministic 계약대로 `--llm-eval=false`를 명시한다. 별도 증거 캐시·skip flag·selfverify production 변경은 하지 않는다.
3. `.issueops/testing/self-verification.md`에 CI 검사 소유 표를 추가한다: Python/self-verify 임시 HOME 1회, 일반Go/self-verify 임시 HOME 1회, golden/성공 full Go 포함, race/독립 CI 1회, nativeintegration/self-verify 임시 HOME 1회. 환경 차이와 standalone 동작을 명시하고 주변 역사·다른 스킬은 수정하지 않는다.
4. 단일 final battery, 독립 diff 리뷰, PR 발행, 최신 push와 PR CI를 완료한다. 실제 CI 로그에서 테스트 횟수와 시간, self-verify coverage gaps 비어있음·termination/OK 성공을 확인한다. Python/Go 실패는 기존 selfverify 실패 전파 테스트와 새 workflow 연결 fixture가 함께 증명하고 race 실패는 workflow fixture exit로 증명한다. 해당 실측과 CI URL을 PR·완료 증거에 기록한다.

## 검증과 게이트
각 gate CHECK는 compound shell이 아닌 한 argv 명령으로 등록하고 EXPECT는 실제 stdout token으로 정한다. owner가 fixture 테스트명과 출력이 확정된 뒤 단일 gates init으로 만든다.
- G1: 신규 workflow 테스트를 직접 실행해 정상 흐름·Python 실패·Go 실패·race 실패 네 시나리오를 검증한다. 성공 exit0, 세 실패가 각각 CI block nonzero인 assertion을 요구한다.
- G2: 기존 selfverify Python/Go failure tests를 focused 실행하고 standalone PlannedSteps가 Python/Go/golden/native 단계를 유지함을 확인한다.
- G3: 최종 scoped full battery에 self-verify 전체 JSON을 한 번 실행해 Python/Go/golden/native 증거를 기록한다. Go production 변경이 없으므로 추가 full Go/race 최종 중복 실행은 하지 않되 CI 독립 race 결과를 보존한다. 변경 필요로 Go가 추가되면 TESTING의 vet/race 기준을 같은 battery에 적용한다.
- G4: PR의 실제 성공 CI를 readback하고 baseline과 전후 호출 수·단계별 시간을 표로 남긴다. 최종 push SHA와 CI head SHA 일치, self-verify coverage gaps 없음, race 성공이 EXPECT다. 이전/다른 SHA 증거는 통과 근거로 인정하지 않는다.

## 인계와 실행 경계
이 계획을 artifact stage한 뒤 독립 plan 리뷰를 통과하고 `execution prepare --mode direct`가 canonical worktree를 만든다. 기존 holder가 writer 0을 관측한 뒤 release한다. Orca의 같은 worktree 새 native Codex 세션을 gpt-6.1-sol/high로 띄우며 receipt와 실제 process argv를 확인한다. 새 owner는 실제 whoami, next의 released direct replace/claim 명령 체인을 따른다. 자동으로 또 인계하지 않는다.
구현은 한 owner가 순차 수행한다. 다른 네 worktree 작업은 병렬이므로 전역 설치·소스 checkout 변경·타 owner 파일 덮어쓰기를 금지한다. GOFLAGS=-p=2, GOMAXPROCS=4를 각 검사 프로세스에 적용한다.

Repo grounding: workflow·PlannedSteps·기존 CI 로그 및 project docs를 대조했다.
Decision-complete plan: 중복 두 CI 단계를 제거하고 임시 HOME self-verify를 단일 소유자로 쓴다.
Assumptions/defaults: base는 사용자 지정 main, TDD와 실제 CI readback, owner는 gpt-6.1-sol/high다.
Rejected alternatives: cross-run cache와 self-verify skip 옵션은 환경 증거 검증 복잡도를 늘려 제외한다.
