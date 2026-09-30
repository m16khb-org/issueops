# #521 품질 지표의 제품 코드 선정 경계

- Lifecycle: io-dd786760d2a9
- Issue: https://github.com/m16khb-org/issueops/issues/521
- Branch: 521-quality-production-source-scope
- Base: main, 768546a219b082f5f7ad7f78f85664747eca9b74
- 사용자 원문: “3 항목을 각각 $issueops 를 통해 병렬로 인계하여 진행해줘”
- 승인된 종료점: 독립 canonical worktree에서 구현·검증·커밋·push·draft PR 발행·execution complete. merge와 cleanup은 하지 않는다.
- direct execution prepare 후 같은 worktree에 Orca 새 Codex gpt-6.1-sol/high 세션으로 인계한다. 새 세션은 자동 재인계를 반복하지 않는다.

## 문제와 범위

`internal/adapter/outbound/quality/source.go:14-32`의 CollectBranchFunctions는 일부 디렉터리만 제외하고 모든 non-test Go 파일을 파싱한다. 실제 source checkout의 `.issueops/evidence/ddd-refactor/reproduction/` Go 파일이 분기 상위에 나온다. `snr.go:14-36`의 ComputeCodeSNR는 별도 WalkDir로 파일을 고른다. 실제 composition root `cmd/issueops/issueopsapp/quality_wiring.go:32-33`은 두 collector를 연결한다.

두 지표가 제품 Go 코드 대상으로 같은 파일 선정 함수를 사용하도록 고친다. scope는 outbound quality scanner와 focused tests, 실제 wiring 검증, 품질 기준 문서다. coverage 실행·cache fingerprint 알고리즘, reviewer effort, self-verify latest 선택, API DTO/schema, 설치/전역 설정은 바꾸지 않는다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제2장 안전/정확성: 읽기 전용 분석에서 경계 밖 파일을 읽지 않고 실패를 정상 빈 결과로 숨기지 않는다.
- `.issueops/ARCHITECTURE.md` 의존 방향, `.issueops/architecture/domain-responsibilities.md` quality collector: 파일/Git 관측은 outbound adapter, branch 계산과 결과 분류는 기존 domain 책임을 유지한다. collector 실패가 다른 collector 진단을 버리지 않아야 한다.
- `.issueops/CONVENTIONS.md` 생성물/dependency: 새 외부 dependency와 전역 cache를 만들지 않는다. 호스트별 로직 복제 금지.
- `.issueops/ADR.md` Accepted baseline 및 `adr/decisions/2026-05-25-plugin-vs-external-worker.md`: 같은 CLI/MCP core 결과를 유지한다.
- `.issueops/adr/decisions/2026-08-08-legacy-baseline-invariant.md`: import fitness baseline을 늘려 우회하지 않는다.
- `.issueops/CAUTIONS.md`, `cautions/audit-and-process.md` RED 선행 및 JSON 검증: named regression test 실패를 먼저 보존하고 실제 DTO tag로 성공을 확인한다.
- `.issueops/TESTING.md`, `testing/self-verification.md`: focused RED/GREEN 뒤 full single-pass verification을 수행하며 `--llm-eval=false`를 명시한다. 기존 설치/source checkout을 바꾸지 않는다.

## 재사용하는 기존 구현

- CollectBranchFunctions의 AST branch counting, 정렬, warning 합성 및 ComputeCodeSNR의 줄 분류를 유지한다.
- 같은 outbound quality package 안에 공통 파일 선정 helper를 둔다. 서로 다른 파일 선별 정책을 복제하지 않는다.
- `coverage.go:125`의 `git ls-files --others --exclude-standard -z` 선례를 따라 Git에서는 tracked와 nonignored untracked를 함께 수집한다. 기존 coverage helper 자체는 stdout/stderr 혼합과 timeout 부재로 그대로 확장하지 않고, 기존 bounded buffer를 재사용한 좁은 read-only Git 관측으로 구현한다.
- 기존 `cmd/issueops/qualitycli/quality_snr_test.go`, outbound quality tests와 composition wiring을 재사용한다. 새 프레임워크나 별도 스캐너 CLI를 만들지 않는다.

## 선정 정책

1. Git 작업 공간이면 `git ls-files --cached --others --exclude-standard -z -- .`에 해당하는 현재 작업 공간 파일을 후보로 삼는다. tracked 파일이 ignore 패턴과 겹쳐도 제품 파일이면 포함한다. staged/unstaged 수정과 nonignored 신규 파일을 보존한다. tracked 삭제 파일은 현재 존재하지 않으므로 집계하지 않는다.
2. 모든 모드에서 `_test.go`, `testdata`, `vendor`, `node_modules`, `.git`, `.codegraph`, `.issueops-runtime`, `bin`, `.issueops/evidence`, `.issueops/tmp`, `.issueops/issues` 아래 파일을 제외한다. `.issueops` 전체를 제품 코드로 간주하지 않으며, 다른 정상 디렉터리를 임의로 배제하지 않는다. 제외는 경로 구성요소 기준으로 판정한다.
3. 표준 `// Code generated ... DO NOT EDIT.` 주석이 있는 파일은 Go의 `ast.IsGenerated`에 맞춰 제외한다. 일반 주석에서 generated 단어가 나온다는 이유로 제외하지 않는다. 공통 파일 선정에서 판정해 두 지표를 일치시킨다.
4. symlink 파일·디렉터리는 따라가지 않는다. Git 후보 경로는 root-relative 여부를 검증한다. Git worktree의 `.git` 파일과 하위 디렉터리를 root로 넘기는 경우도 동작해야 한다.
5. Git이 없는 일반 디렉터리는 같은 명시적 제외 규칙의 WalkDir로 분석한다. Git 미설치 때도 일반 디렉터리 기능을 유지한다. Git 관측 실패·timeout·출력 제한·파일 읽기 오류는 warning/error로 표면화하고 성공한 빈 분석으로 바꾸지 않는다. non-Git은 정상 경로로 구별하고 실제 Git 오류를 non-Git으로 오인해 ignored 파일을 다시 포함하지 않는다.
6. 두 collector는 동일 helper를 사용하되 현재 함수 signature를 유지한다. branch collector의 warning/partial 결과와 SNR error를 현행 application 정책에 맞춰 전달한다. CLI JSON field, collection_status, gate_status의 의미와 domain 분류를 바꾸지 않는다.

## 성능 영향

quality inspect는 진단 명령이며 요청 hot path가 아니다. 기존 전체 WalkDir+parse 대비 Git 모드에서 후보 파일 열거 O(N), 제품 파일 parse O(B)를 수행한다. 각 collector가 독립 실행되는 기존 구조를 유지해 공유 cache/쓰기/locking을 추가하지 않는다. Git subprocess는 유한 timeout과 bounded output으로 감싼다. fixture와 실제 저장소에서 파일 수·스캔 시간만 비교하고 개선률을 추정하지 않는다. coverage 전체 재실행 시간과 scanner 시간은 분리한다.

## 하위 호환성과 side effect

공개 함수 signature, DTO, CLI/MCP schema, durable record와 state 저장 형식은 유지한다. 제품 코드 범위에서 벗어나는 파일 제거로 수치가 달라지는 것은 의도된 수정이며 baseline 숫자를 기계적으로 고정하지 않는다. 추적되지 않은 정상 신규 코드와 Git 없는 디렉터리를 지원한다. root 밖 symlink와 ignored 임시 코드를 읽지 않으며 Git 명령은 read-only다. 새로운 파일 쓰기는 canonical worktree의 소스/테스트/프로젝트 문서와 이슈 산출물뿐이다. migration, DB, provider body 의미 변경은 없다. 롤백은 이 PR revert이며 runtime state migration은 필요 없다.

## 실행 작업과 검증

1. **RED:** outbound quality에 제품 파일 fixture helper와 regression tests를 추가한다. Git repo에 tracked, tracked-but-ignored, nonignored untracked, ignored untracked, staged deletion, 공백/개행 경로, excluded evidence/testdata, 표준 generated/일반 comment, root 밖 symlink를 둔다. non-Git 및 `.git` 파일인 worktree, nested root를 검증한다. 두 collector의 포함/제외를 같은 fixture에서 비교한다. 현재 구현이 evidence inclusion assertion에서 실패한 exact command/exit를 저장한다.
2. **GREEN:** 위 공통 선정 helper와 두 collector 연결을 최소 구현한다. Git 오류/timeout/출력 초과와 read error가 표면에 남는 named test를 둔다. 단순 fallback으로 오류를 삼키지 않는다. source checkout은 수정하지 않는다.
3. **SURFACE:** 실제 composition/CLI quality test에서 DTO와 collection failure를 확인한다. canonical worktree 내부 ignored evidence fixture로 실제 collector 결과를 관측하고 fixture는 제거한다. source checkout의 이전 품질 JSON(`/tmp/issueops-improvements-quality.json`)은 현황 근거일 뿐 새 worktree 결과로 위조하지 않는다.
4. **CLEAN/DOCS/VERIFY:** issueops-slop-clean과 docs review를 거친다. quality 관련 canonical 문서가 현재 기준을 설명하도록 최소 갱신하며 다른 두 병렬 cycle의 CONVENTIONS reviewer 문구/status 문서를 건드리지 않는다. 그 뒤 독립 diff review, full verification, atomic commit/push, draft PR 및 execution complete를 수행한다. 준비 세션은 실제 인계 수신 확인 후 종료한다.

Gate specs (구현 진입에서 `.issueops/issues/521/gates.md`에 한 번 생성):
- G1: 제품 파일 포함·제외와 오류 경계가 보존된다 | CHECK: go test ./internal/adapter/outbound/quality -count=1 | EXPECT: ok
- G2: 실제 quality 명령과 wiring 응답 계약이 유지된다 | CHECK: go test ./cmd/issueops/qualitycli ./cmd/issueops/issueopsapp -run Quality -count=1 | EXPECT: ok
- G3: 계층 의존 방향을 유지한다 | CHECK: go test ./internal/architecture -count=1 | EXPECT: ok

전체 Go/race/vet/build/docs/inspect/self-verify는 프로젝트 single-pass 검증에서 한 번씩 수행한다. named selector가 실제 test를 실행했는지 확인한다. stdout/stderr와 종료 코드는 worktree 밖 증거 디렉터리에 남긴다.
