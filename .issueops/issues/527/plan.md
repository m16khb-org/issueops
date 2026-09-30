# 공개 추적 문서의 로컬 경로 재유입 수정

Lifecycle: `io-a7461598e205` · 이슈: https://github.com/m16khb-org/issueops/issues/527
Branch: `527-public-material-paths` · 기준 브랜치: `main`
작업 공간: `$SOURCE_ROOT.worktrees/527-public-material-paths` (`$WORKTREE`)

## 요청과 완료 범위

사용자 요청 `진행해봐`에 따라 공개 사본의 경로 재유입 결함을 고친다. 준비 뒤 direct canonical worktree를 만들고 Orca ready 환경이면 같은 작업 공간의 새 native Codex 세션으로 인계한다. 구현·검증·커밋·push·Draft PR·최신 HEAD의 원격 CI 성공·execution complete/released까지 수행한다. 머지와 cleanup, 전역 설치는 상위 조정 세션이 담당한다. 설치된 Go CLI와 Python을 사용하고 새 의존성은 추가하지 않는다.

## 확인한 현재 동작

- `internal/application/issueopsremote/tracked_materials.go:77`의 contents는 봉인 plan/intent/spec bytes를 공개 사본에 전달한다. intent가 없으면 record 기반 렌더를 쓴다. plan-review는 기존 domain 렌더러를 사용한다.
- 같은 파일의 Write(42행)는 bytes가 다르면 덮어쓰고 같으면 건너뛴다. 오류는 warning이며 phase를 되돌리지 않는다.
- `internal/application/issueopscycle/phase_service.go:105`는 implement와 ai-slop-clean 진입 때 Write를 호출한다.
- `internal/adapter/issueops/issueops_materials_test.go:79`는 이 두 전이 및 사본 반복 생성 fixture를 제공한다. 130행 이후는 봉인 밖 계획을 덮어쓰지 않는 계약을 검증한다.
- `internal/domain/issueops/tracked_plan_review.go:13`은 기존 계획 검토 renderer이며 해시·로컬 경로 가림을 유지해야 한다.
- `scripts/meeting_notes_skill_contract_test.py:137` 이후 검사는 Git tracked UTF-8 파일 전체를 순회한다. 새 계획 문서도 대상이므로 공개 파일을 stage한 상태의 검증이 필요하다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제2장 안전/정확성: 봉인 bytes와 실행 재개 입력은 수정하지 않는다.
- `.issueops/ARCHITECTURE.md` 의존 방향, `architecture/hexagonal-core.md` Current package boundaries: 순수 경로 표현 규칙은 domain, 사본 읽기·쓰기는 application에 둔다.
- `.issueops/CONVENTIONS.md` 이슈 산출물 레이아웃: 공개 사본은 파생물이며 같으면 쓰지 않는다. 기존 봉인 밖 계획은 계속 건드리지 않는다.
- `.issueops/CAUTIONS.md` 로컬 검증/CI 일치: 실제 공개 문서도 stage 후 Python 검사한다.
- `.issueops/cautions/issueops-stages.md` execution mode와 세션 런처: direct worktree를 다시 만들거나 mode를 바꾸지 않는다.
- `.issueops/ADR.md` 및 `adr/decisions/2026-08-28-issueops-devils-advocate-plan-binding.md`: 계획 검토는 현재 digest에 묶는다.
- `.issueops/TESTING.md`, `testing/unit-and-contract.md`, `testing/self-verification.md`: focused RED/GREEN과 동일 revision 최종 battery를 남긴다. self-verify가 실제 수행한 full test/build/docs/inspect는 중복하지 않고 누락된 vet/race는 별도로 실행한다.

## 재사용하는 기존 구현

`TrackedMaterials.contents/Write`, `MaterialFiles`, `planSource`, 기존 intent/review renderer와 `materialsCycleForTest`를 재사용한다. domain에 작은 순수 공개 문서 경로 정규화 함수를 추가하고 contents가 완성한 모든 material의 공개 bytes에만 한 번 적용한다. 다른 공개 본문 renderer, secret 필터, 임의 파일을 순회하는 scrubber는 만들지 않는다. 기존 plan-review 가림 정책을 바꾸지 않고 마지막 공개 파생 경계에 통합한다.

## 선택한 처리 규칙

1. 입력은 문서 bytes와 record에 이미 있는 source root(`record.Repo`) 및 worktree root다. 파일·환경·현재 작업 디렉터리 조회를 domain에서 하지 않는다.
2. source root는 `$SOURCE_ROOT`, worktree root는 `$WORKTREE`로 바꾼다. 둘의 접두 관계가 있으면 더 구체적인 긴 root를 먼저 판정한다. 뒤의 경로·파일·줄 번호는 그대로 남긴다. 빈 값·상대 root·파일시스템 루트 `/`는 치환 후보로 쓰지 않는다.
3. 해당 두 root에서 확인되는 POSIX 사용자 홈 접두(`/Users/<account>` 또는 `/home/<account>`)만 `$HOME`으로 바꾼다. 그 외 계정·임의 absolute path를 추정하지 않는다. 다른 플랫폼 지원이나 범용 개인정보 탐지는 이번 범위가 아니다.
4. path token의 경계를 확인해 `prefix + 더 긴 이름`, URL의 path/query, 문장 중 일반 문자열의 부분 일치는 건드리지 않는다. 새 정규화 함수에 들어온 URL bytes는 그대로 보존한다. plan-review는 기존 RenderTrackedPlanReview가 먼저 수행한 가림 결과를 입력으로 받으므로, 기존 renderer의 /home/·/Users/ URL 부분 가림은 이번 변경에서 복구하거나 변경하지 않는다. URL 보존 기준은 신규 정규화가 추가 손상을 만들지 않는다는 뜻이다. Markdown inline code·code block·인용된 shell argv·Markdown 로컬 링크 등 실제 로컬 경로 표현은 정규화한다. 공백을 포함하는 root도 입력으로 검증한다.
5. 이미 쓰인 `$SOURCE_ROOT`·`$WORKTREE`·`$HOME`과 상대 경로, 일반 본문은 그대로 둔다. 같은 bytes에 반복 적용해도 결과가 같다. planSource의 공개 파일 건너뛰기와 warning 계약은 유지한다.
6. 원본 file·mode·hash, record·artifact digest·owner prompt·resume token에는 쓰지 않는다. phase가 다시 쓰는 사본에만 정규화된 bytes가 들어간다.

## 구현 순서와 수락 기준

1. 기존 전이 fixture에 source/worktree 및 같은 사용자 홈 경로를 포함한 plan/intent/spec을 준비한다. 생성 때 공개 경로와 원본 bytes/hash를 함께 검사하는 실패 테스트를 먼저 실행한다. 식별정보는 합성 계정만 사용하고 실제 로컬 계정 문자열은 추적 파일에 쓰지 않는다.
2. 순수 domain 함수에 boundary·겹친 root·공백·URL·상대경로·반복 적용 테스트를 작성하고 public contents 경계에서 호출한다.
3. implement→ai-slop-clean 전이에서 공개 문서를 수동으로 바꾸거나 삭제한 뒤 다시 생성해도 정규화가 유지되는지 확인한다. plan/intent/spec/plan-review 및 record fallback을 포함하고, 원본 hash는 전후 같다. 신규 함수의 URL 입력은 byte 보존을 확인하며, plan-review의 `/home/` 포함 URL 입력은 기존 RenderTrackedPlanReview 결과가 그대로 유지되는지 별도로 확인한다. unchanged 두 번째 Write가 Written를 늘리지 않는지 검증한다.
4. `.issueops/CONVENTIONS.md`의 공개 파생물 설명을 실제 정책과 맞추고 필요한 caution만 추가한다. 원본을 고치거나 검사를 완화해 통과시키지 않는다.
5. 실제 공개 파일을 stage한 뒤 Python CI 검사를 실행한다. 최종 자기 검증·정적/race·독립 구현 리뷰·Draft PR 발행·최신 HEAD push/PR CI 성공·원격 readback·execution complete까지 진행한다.

## 게이트 원장 입력

- G1: 공개 경로와 원본 불변성 | CHECK: go test ./internal/domain/issueops ./internal/application/issueopsremote ./internal/adapter/issueops -run 'Test.*(Tracked|Materials|PublicMaterial)' -count=1 | EXPECT: ok
- G2: 실제 공개 추적 파일 검사 | CHECK: python3 -m unittest discover -s scripts -p '*_test.py' | EXPECT: OK

G2는 unittest의 실제 종료코드 0과 출력 OK를 함께 요구한다. 최종 self-verify는 별도 최종 battery로 한 번 수행하며 같은 책임의 full test/build/docs/inspect를 원장에 중복 등록하지 않는다. `GOFLAGS=-p=2 GOMAXPROCS=4`를 긴 테스트 프로세스 환경에만 적용한다. Python은 `/opt/homebrew/bin/python3`이며 PATH에서 `/opt/homebrew/bin`을 앞에 둔다. self-verify가 수행하지 않은 vet/race는 별도 최종 evidence에 남긴다.

## 성능 영향

문서 생성 phase에서만 문서 길이에 비례한 텍스트 변환을 추가한다. 후보 root 수는 최대 네 개의 상수 크기다. 추가 파일 읽기·원격 조회·cache·새 상태는 없다. 표본 문서와 큰 합성 문서의 정규화 시간을 측정해 비정상 증가가 없는지 확인하고 개선 효과를 추측하지 않는다.

## 하위 호환성과 side effect

CLI/MCP field와 record schema를 바꾸지 않는다. 공개 파생 문서의 로컬 경로 표현만 바뀌며, 실행 계약은 봉인 원본을 계속 읽는다. 기존 plan-review 마스킹과 사본 warning·파일 mode·외부 계획 건너뛰기는 유지한다. 이미 공개된 과거 문서를 일괄 다시 쓰지 않는다. 현재 사이클의 다음 정상 전이가 사본을 갱신한다. 롤백은 이 변경을 revert하면 다음 재생성부터 기존 표현으로 돌아가며 원본 복구는 필요 없다.

Repo grounding: 위 source 경로와 기존 fixture를 확인했다.
Decision-complete plan: 공개 파생 경계에만 순수 경로 정책을 적용한다.
Assumptions/defaults: POSIX root에서 확인되는 사용자 홈만 다루며 외부 URL은 보존한다.
Unresolved questions: 구현을 막는 미결정은 없다.
Acceptance criteria: 최초 생성·전이 재생성·원본 불변·URL 보존·CI 및 최종 HEAD 검증.
