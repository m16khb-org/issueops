# 최신 자기 검증 상태 선택 개선

- Lifecycle: `io-14e119b8fce9`
- Issue: https://github.com/m16khb-org/issueops/issues/523
- Source: `$SOURCE_ROOT`
- Branch: `523-latest-selfverify-status`, base `main` at `768546a219b082f5f7ad7f78f85664747eca9b74`
- 사용자 원문: “3 항목을 각각 $issueops 를 통해 병렬로 인계하여 진행해줘”
- 승인된 종료점: 독립 구현·검증·해당 브랜치 commit/push·Draft PR 발행·execution complete. merge/cleanup은 제외한다.
- 준비 후 direct canonical worktree를 유지하고 Orca의 새 Codex `gpt-6.1-sol/high` 세션으로 인계한다. 이미 인계받은 세션은 다시 인계하지 않는다.

## 문제와 수락 기준

`internal/domain/status/result.go:37`은 `self-verify` prefix에 처음 맞는 기록을 선택한다. `result_test.go:39`는 오래된 항목이 최신 항목보다 앞서면 오래된 항목을 고르는 현재 동작을 고정한다. `application/status/service.go:37`은 state 목록의 key/updated_at/bytes만 전달하므로 후보 목록과 실행 요약도 구별하지 못한다.

수정 후 최신 적격 실행 요약을 생성 시각 기준으로 선택한다. 최신 실행이 실패해도 이전 성공을 대신 고르지 않는다. 새 응답 필드를 만들지 않고 기존 latest_key/found/updated_at/bytes는 선택된 기록의 metadata를 그대로 제공한다. 전체 status OK는 기존 doctor/state/worker 조회 성공 의미를 유지하며 selfverify 결과의 OK로 바꾸지 않는다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제3장: JSON 읽기와 state I/O는 application/adapter, 적격성과 순서는 순수 domain에 둔다.
- `.issueops/ARCHITECTURE.md` 의존 방향: concrete adapter import는 root 조립에만 둔다. 기존 history capability를 재사용한다.
- `.issueops/CONVENTIONS.md` DDD 책임 분리: CLI는 flag/출력만 맡으며 domain에 filesystem·DB 접근을 추가하지 않는다.
- `.issueops/architecture/hexagonal-core.md` Status aggregation and inspect boundary: first-prefix 정책을 이번 변경의 실제 정책으로 갱신한다. doctor OK 및 조회 오류의 기존 판정은 유지한다.
- `.issueops/CAUTIONS.md` Universal summary: exact canonical cwd에서만 편집한다. 게이트 CHECK는 단일 argv 명령이다.
- `.issueops/ADR.md`와 `adr/2026-09-08-issueops-project-doc-gates-link-plan-checks-the-four-plan-se.md`: plan 네 절과 문서 반영 근거를 유지한다. 별도 구조 ADR은 필요 없다.
- `.issueops/TESTING.md`: focused RED→GREEN 뒤 전체 Go/race/vet 및 single-pass selfverify 계약을 적용한다. 부분 통과를 섞어 완료로 기록하지 않는다.

## 재사용하는 기존 구현

- `internal/application/selfaugment/history.go`의 `HistoryService.History`가 state JSON decode, schema/kind 판별, warning/skipped, 정렬을 이미 조합한다. status application에서 이 서비스를 재사용한다.
- `internal/domain/selfaugment/history_request.go`의 `HistorySnapshotDiagnostics`는 schema_version=1 및 self_verification_summary kind만 적격으로 본다. candidate export는 제외된다.
- `internal/domain/selfaugment/history_order.go`의 `SortHistoryEntries`가 generated_at 유효 항목 우선→generated_at 내림차순→updated_at 유효 항목 우선 및 내림차순→key 사전순을 구현한다. 이 순서를 그대로 따른다. 새 정렬 정책이나 별도 prefix 예외 목록을 만들지 않는다.
- `application/selfverify/save.go`는 summary를, `save_candidates.go`는 별도 kind의 후보 자료를 저장한다. baseline과 budget 키라도 실제 summary이면 적격이다. 오래된 summary의 복사로 updated_at만 최근이 되어도 더 최근 generated_at을 이기지 못한다.
- state read는 기존 `internal/application/state.Service.Read` 및 root 인스턴스를 주입한다. 임의 --state-key도 지원하려고 prefix를 빈 문자열로 하여 내용이 적격인 요약을 찾는다.

## 구현 순서

1. 현재 status의 순서 의존·후보 혼동을 재현하는 focused 실패 테스트를 작성한다. 새 정책은 기존 이력 테스트와 비교한다.
2. status application에 기존 state read capability를 주입한다. 기존 State()로 받은 목록을 HistoryService.List closure에서 재사용하여 목록 조회를 반복하지 않는다. HistoryService.History("", 1, zero retention options)를 호출한다. Delete callback은 연결하지 않으며 prune/promotion/write를 호출하지 않는다. State 조회가 실패하면 history 조회를 생략한다.
3. domain/status에서 raw key prefix 선택을 제거하고 검증된 최신 history 결과를 투영한다. history 생성/읽기 오류는 기존 status warning 경로에 명시하고 전체 OK를 false로 둔다. HistoryService의 skipped는 unrelated kind/schema/non-JSON이면 정상 제외한다. state_read 실패만 조용히 무시하지 않고 status warnings에 key와 이유를 남겨 오래된 fallback을 완전한 최신이라고 오해하지 않게 한다. invalid_generated_at warning도 전달하되 이 진단 경고 자체는 전체 OK를 false로 바꾸지 않는다. doctor/state/worker 조회 실패 및 추가 state_read 실패와 진단 경고를 domain 입력에서 구분한다. 기존 경고의 순서는 보존하고 history 경고를 뒤에 더한다. 시각 오류에는 기존 history fallback 순서를 그대로 따른다.
4. root wiring과 영향받는 status application/domain/CLI 테스트를 갱신한다. 기록 전체 content나 민감 값을 응답에 추가하지 않는다. history policy 자체의 의미는 바꾸지 않는다.
5. `.issueops/architecture/hexagonal-core.md` Status aggregation 절만 project-docs-update로 갱신한다. API 필드/record schema/외부 상태 쓰기 변화가 없음을 확인한다. ai-slop-clean, docs, verify, issue branch commit/push, Draft PR, complete를 라우터로 수행한다.

## 검증 시나리오

- 오래된 summary→최신 summary와 반대 순서가 같은 최신 키를 고른다. 임의 키의 적격 summary도 동작한다.
- 최신 `ok:false` summary와 이전 성공 summary가 있으면 실패 실행의 key를 고른다. 전체 status OK는 기존 조회 성공 규칙을 유지한다.
- 새 candidates export, selfaugment summary, unsupported/missing schema, unrelated JSON이 최신 생성 시각이어도 제외된다.
- generated_at RFC3339Nano 및 timezone offset을 실제 시각으로 비교한다. invalid/missing generated_at은 valid 뒤에 오고, 전부 invalid면 valid updated_at 내림차순으로 결정한다. 시각까지 동률이면 key 사전순이며 입력 순열과 무관하다.
- 최근에 승격/복사한 오래된 baseline summary가 새 실행을 덮지 않는다. 같은 실행 복사끼리 동률은 기존 history tie-break와 같다.
- state list 한 번, 목록 내부 읽기 외에 각 record의 추가 StateRead 최대 한 번, read/delete/write 없는 빈 목록, State 실패 시 추가 read 없음, 조회 오류 및 순수 domain 입력 무변이.
- 정상 최신 summary와 시각 오류가 있는 오래된 summary가 함께 있어도 최신 key와 invalid_generated_at warning을 함께 반환하고, 모든 조회가 성공했다면 전체 OK=true다.
- state read 실패는 warning과 status 실패로 보이며 unrelated kind skip은 정상이다. 후보만 있는 경우 found=false다.
- CLI temp state에는 실제 production summary snapshot 형식 fixture를 사용한다. 전후 state 내용 비교로 조회가 데이터 변경을 일으키지 않음을 확인한다. 기존 저장소 초기화 side effect를 새로 제거하는 범위는 아니다.

## 성능 영향

기존 status는 metadata 목록만 O(n) 순회하지만 State.List 내부는 이미 각 record를 읽는다(application/state/service.go:123). 요약 종류와 생성 시각 확인을 위해 각 record의 추가 StateRead가 최대 한 번 발생하여 읽기 O(n)이 추가되고, 기존 이력 정렬 O(k log k), 공간 O(k)가 필요하다. 한 번 실행하는 진단 명령이므로 이력을 재사용하는 단순성을 택한다. 목록 중복 조회와 retention 실행은 금지한다. 작은/많은 fixture에서 read 횟수 및 elapsed를 기록하며 cache/index/새 SQL API는 만들지 않는다.

## 하위 호환성과 side effect

기존 JSON 필드와 envelope schema는 유지한다. key prefix만 맞는 임의 자료가 selfverify로 보이던 동작은 의도적으로 제거하며, schema/kind가 맞는 저장된 summary는 키와 성공 여부에 관계없이 인정한다. 기존 history의 시각 오류 fallback 및 tie-break를 재사용한다. 조회는 read-only이며 delete/write/promotion을 추가하지 않는다. 최신 실행 실패가 상태 전체 OK를 바꾸는 새 건강도 계약은 만들지 않는다. rollback은 해당 커밋 revert로 가능하며 데이터 마이그레이션은 없다.

## 게이트

- G1: 최신 적격 요약 선택과 오류·동률·실패 유지가 일관된다 | CHECK: go test ./internal/domain/status ./internal/application/status ./internal/application/selfaugment ./internal/domain/selfaugment ./cmd/issueops/statuscli -count=1 | EXPECT: ok
- G2: DDD 의존 방향을 지킨다 | CHECK: go test ./internal/architecture -count=1 | EXPECT: ok
- G3: CLI 응답 계약이 유지된다 | CHECK: go test ./cmd/issueops/contractgolden -run Golden -count=1 | EXPECT: ok

전체 race/vet/build/selfverify는 TESTING single-pass 규칙에 맞춰 최종 diff에서 수행하고 중복 등록하지 않는다. source main, 품질 scanner, reviewer effort 파일은 이번 담당 범위가 아니다.

Repo grounding: 위 status·history·producer·state 소스 및 docs를 읽었다.
Decision-complete plan: 기존 HistoryService를 read-only로 재사용하고 status projection만 바꾼다.
Assumptions/defaults: 최신은 검증 생성 시각, 실패도 적격, baseline 복사 시각은 우선 기준 아님.
Unresolved questions: none blocking.
Acceptance criteria: G1–G3 및 오류/무변이/CLI temp state 시나리오.
