# 문서 변경 검증의 책임과 운영 예시를 분리한다

- Lifecycle: `io-59a5d211f12e`
- Issue: https://github.com/m16khb-org/issueops/issues/544
- Source: `$SOURCE_ROOT`
- Branch: `544-doc-verification-ownership`; base `main` / `f8e7c37fd9ff09d68536aee244cf560560e4d919`.
- Canonical worktree는 direct execution prepare가 반환한 절대 경로를 사용한다.
- Owner: Codex `gpt-6.1-sol`, effort `high`. 이 저장소에는 다른 병렬 worker가 있다. 다른 worker 수정이나 다른 lifecycle을 변경하지 않는다.
- 사용자 승인: 개선 1·2·3의 IssueOps 병렬 구현 인계, PR·머지, issueops-cleanup, io-update. 이 준비 세션은 계획과 worktree·lease release까지만 수행한다. root가 같은 worktree의 supervised Orca worker를 띄우며 구현 owner는 PR 발행·execution complete까지 수행하고 머지·정리·업데이트는 root가 수행한다. 인계받은 owner는 자동 인계를 재실행하지 않는다.

## 문제와 성공 기준

`.issueops/TESTING.md:27-42`는 전체 Go 테스트·빌드·docs·inspect와 self-verify를 각각 최소 명령으로 나열한다. `.issueops/testing/self-verification.md:10-53`는 설치·state·daemon 쓰기와 여러 self-verify를 문서 변경에도 요구하지만 같은 파일 `:61-64`는 실제 self-verify 증거를 재사용하고 중복 실행하지 말라고 한다. #535와 PR #541은 CI 검사 소유권을 이미 정리했으므로 그 규칙을 보존한다.

성공 기준은 두 문서가 동일한 최소 완료 게이트를 안내하고 운영 명령 목록을 선택적 예시로 명시하는 것이다. self-verify에서 성공한 test/build/golden/docs/inspect 증거가 같은 revision·환경·입력의 최종 bundle에 있으면 별도 재실행을 요구하지 않는다. 문서-only 최소 완료 검증에는 설치나 daemon/state 변경을 추가하지 않는다. Go 변경의 vet/race 필요성은 변경 범위로 결정하고 해당 명령의 실제 step 증거가 없으면 별도 실행한다. CI의 HOME 격리·독립 race·실패 전파·single-run 실패 후 전체 재시작을 보존한다.

## 적용되는 결정과 주의사항

- `AGENTS.md §0,2,3,9`: 검증 가능한 사실을 확인하고 최소 범위로 수정한다. §9의 명령 목록이 독립적 필수 실행으로 읽히지 않게 TESTING owner 링크와 운영 예시 제목을 보강한다.
- `.issueops/CONSTITUTION.md §0,2`: 현재 코드와 지침을 대조한다. user-scope 설치·외부 상태는 문서 검증의 자동 side effect가 되지 않아야 한다.
- `.issueops/ARCHITECTURE.md Host integration`: 공용 skill과 Go core 책임을 유지한다. 이번 문서 변경은 core와 host 계약을 변경하지 않는다.
- `.issueops/CONVENTIONS.md Canonical single-owner pointers`: 테스트 최소 기준은 TESTING index, 실제 절차는 testing/self-verification owner module에 둔다.
- `.issueops/CAUTIONS.md`와 `cautions/lessons/2026-08-26-ci-gofmt-gate-local-battery-drift.md`: 서로 다른 revision·환경의 검사 증거를 섞지 않고 CI와 일치하는 gate를 유지한다. 오래된 golden 업데이트 안내로 문서-only golden을 임의 갱신하지 않는다.
- `.issueops/ADR.md`, `adr/2026-09-08-adversarial-review-throughput-executable-findings-change-tie.md`: docs-only 검토는 실제 side effect와 필수 계약 증거를 확인한다. 검증과 리뷰는 봉인된 변경 집합에 수행한다.
- `.issueops/TESTING.md`, `testing/unit-and-contract.md`, `testing/self-verification.md`: full test 성공의 golden 재사용, 명시적 `--llm-eval=false`, single-run 계약, Go vet/race 별도 구성원, CI 검사 소유권을 유지한다.

## 재사용하는 기존 구현

새 검증 runner나 cross-run cache를 만들지 않는다. `internal/application/selfverify/steps_test.go:17`은 full Go step argv 계약을, `python_test.go:19`는 Python discovery argv 계약을 고정한다. `internal/adapter/verification/probe/smoke/validation_smoke.go:43,73`은 inspect/docs smoke의 기존 소유자다. 검증 문서의 최종 battery와 CI 검사 소유권 절을 그대로 중심으로 삼고 앞부분을 그 계약에 연결한다. 기존 project-doc checker와 self-verify를 그대로 실행하며 스타일만 검사하는 새 테스트를 만들지 않는다.

## 성능 영향

production hot path와 알고리즘은 바뀌지 않는다. 이 문서가 안내하는 최종 검증에서는 같은 evidence를 가진 전체 테스트·golden·빌드·docs·inspect 반복이 사라진다. 고정 절감률은 약속하지 않는다. 변경 전후 필수 명령 수와 실행 소유 관계를 리뷰 근거로 남긴다. 단일 self-verify 결과를 이전 실행·다른 환경에서 재사용하는 최적화는 추가하지 않는다.

## 하위 호환성과 side effect

CLI flag·JSON·MCP·record schema·provider 계약은 바뀌지 않는다. 읽는 절차 산문만 변경하므로 prompt template 출력 계약 변경이 없다. docs-only 최소 검증에 user-home install·bootstrap apply·daemon start/stop·state save/promote를 넣지 않는다. 기존 명령 목록은 선택적 운영 예시로 보존하고 각 실행은 해당 변경 범위 및 별도 승인/격리 요구를 따른다고 설명한다. self-verify의 기존 내부 native smoke와 임시 build 계약을 삭제하지 않는다. 현재 설치가 없거나 binary가 stale하면 하네스 설치/업데이트가 별도 전제이며 이 문서 변경의 implicit install 권한으로 해결하지 않는다. root가 별도로 승인받은 io-update는 merge/cleanup 후 실행한다.

## 구현 작업과 소유 범위

1. canonical cwd·branch·HEAD·source clean status와 exact lifecycle/generation/actor를 대조한 뒤 `next --id` 체인으로 인수한다. active self 이후 기존 구현 skill대로 plan을 링크하고 issue별 gate 원장을 만든다.
2. `.issueops/testing/self-verification.md`의 `문서 단계 검증`은 문서 링크·구조 대조와 `최종 검증 battery`를 필수 기준으로 연결한다. 거대한 현재 명령 fence는 별도 `선택적 운영 명령 예시`로 옮겨 전체 실행 목록이 아니라 관련 기능 변경 시 골라 쓰는 예시임을 명시한다. 설치·bootstrap apply·daemon/state·self-augment와 저장/promote는 docs-only 최소 검증에서 요구하지 않는다. 명령 예시를 한 번씩 전부 실행하지 않는다.
3. `최종 검증 battery`의 test/build/golden/docs/inspect 소유 관계를 간결하게 정리하고 같은 설명의 중복 문장을 없앤다. vet/race 포함 여부는 실제 step 증거 기준으로 유지한다. 현재 working-tree risk 동작을 단언하는 표현은 일반화하여 future behavior를 가정하지 않는다. CI 검사 소유권 표와 HOME별 재사용 금지·독립 race는 보존한다.
4. `.issueops/TESTING.md` 최소 기준은 정규 owner 링크와 동일한 단일 self-verify 게이트로 정리한다. 검증용 binary 준비가 필요하면 build를 실행용 전제와 self-verify가 수행한 검증용 temp build로 구분한다. 별도 full test/golden/docs/inspect를 필수 최종 책임으로 다시 요구하지 않는다.
5. `AGENTS.md §9`의 기존 명령은 개별 운영 예시임을 한두 문장으로 명확히 하고 필수 완료 기준을 TESTING으로 연결한다. 필수 gofmt/vet/race가 코드 변경에서 빠지지 않도록 현재 범위의 조건을 유지한다. 사용자 홈 설치 명령을 최소 검증으로 다시 요구하지 않는다. 이것 외 루트 규칙은 수정하지 않는다.
6. ai-slop-clean·project-doc 양방향 대조·문서 gate와 self-verify를 수행하고 독립 diff 리뷰 후 issue branch에 exact-path commit/push·draft PR·execution complete 한다. worker는 머지·cleanup·io-update를 실행하지 않는다.

허용 편집 경로: `.issueops/TESTING.md`, `.issueops/testing/self-verification.md`, `AGENTS.md`의 §9 최소 문구, lifecycle이 생성하는 `.issueops/issues/544/{plan,intent,plan-review,gates}.md`. 다른 Go 코드·CI·testing/unit-and-contract.md는 수정하지 않는다. unit-and-contract의 risk 의미 업데이트는 riskqa worker 소유다.

## 검증 게이트

문서-only 변경이라 행동 회귀 테스트를 새로 만들지 않는다. 변경 전 모순을 위 위치와 인용으로 기록하고 변경 후 문서 대조를 GREEN/SURFACE 근거로 사용한다.

- G1: 문서 diff 공백·충돌 확인 | CHECK: `git diff --check` | EXPECT: exit 0, 출력 없음.
- G2: project-doc family 링크·구조·라인 예산 | CHECK: `uv run --directory skills/project-docs-optimize python -m scripts.check --root "$PWD" --mode check --json` | EXPECT: exit 0 and JSON ok=true.
- G3: 최종 문서 검증 | CHECK: `issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json` | EXPECT: exit 0 and summary termination_eligible=true, 전체 필수 step 성공. 실행 binary의 source root가 canonical worktree인지 실제 inspect와 process 경로로 먼저 확인하고 canonical source의 최신 binary를 사용한다.
- G4: side-effect·검사 소유권 독립 리뷰 | CHECK: `git diff f8e7c37fd9ff09d68536aee244cf560560e4d919 -- AGENTS.md .issueops/TESTING.md .issueops/testing/self-verification.md` | EXPECT: 필수 검증 목록이 운영 예시와 분리되고 같은 self-verify의 test/build/golden/docs/inspect 중복 없음, 설치·daemon·state 쓰기 필수 없음, 실제 증거 기반 vet/race 및 CI HOME 소유권 보존. 사람이 읽는 diff에 의한 판정과 근거를 gate evidence로 기록한다.

최종 증거는 같은 revision·환경·입력 상태의 한 bundle에 모은다. 실패하거나 파일을 고치면 전체 필수 gate를 처음부터 실행한다. 동일 성공 self-verify 내부 증거를 별도 명령으로 중복 검증하지 않는다. bounded output은 worktree 밖 임시 결과 파일에 보존한다.

## 중단 규칙

actor/generation/branch/worktree mismatch, active other holder, source checkout dirty, plan digest 불일치, remote 결과 모호함은 stop이다. 기록·publication을 우회하지 않고 동일 ID의 next/reconcile chain을 따른다. 다른 worker의 변경을 되돌리지 않는다. 새 scope·판독 불가 외부 권한이 필요한 경우 root에 전달한다. 실패 증거 없이 선택적 문체 선호나 추가 테스트를 범위에 넣지 않는다.

Repo grounding: TESTING/self-verification/unit-and-contract, AGENTS §9, 실제 selfverify tests와 smoke 구현, 관련 #535 완료 상태를 확인했다.
Decision-complete plan: 필수 기준은 owner module에 두고 같은 성공 step 재사용, 운영 예시 선택 실행, 독립 docs owner와 exact path를 정했다.
Assumptions/defaults: 기본 branch main은 provider default와 현재 remote HEAD로 확인했다. runtime code와 CI는 수정하지 않는다.
Unresolved questions: blocking 없음.
Acceptance criteria: G1-G4와 독립 diff 리뷰·완전한 self-verify 증거로 판정한다.
