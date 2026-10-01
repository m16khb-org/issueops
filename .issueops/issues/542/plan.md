# 감사표 판독 실패와 명시적 0건 구분

- Lifecycle: io-ebcba1cf345f; Issue: https://github.com/m16khb-org/issueops/issues/542
- Branch: 542-audit-parser-validation; base main: f8e7c37fd9ff09d68536aee244cf560560e4d919.
- 사용자 요청: 개선점 1·2·3을 IssueOps로 GPT-6.1 Sol 병렬 인계, 검증 후 머지·cleanup·io-update. 이 사이클은 개선점 2만 소유한다. 준비 세션은 direct canonical worktree를 준비하고 lease를 해제한다. root는 Orca Run run_de330d4c1fef에서 gpt-6.1-sol/high 구현자를 같은 worktree에 한 번만 시작한다. 별도 자동 세션을 띄우지 않는다.
- 허용 경로: internal/adapter/outbound/quality/, internal/application/quality/, internal/domain/quality/, cmd/issueops/qualitycli/, 필수 cmd/issueops/issueopsapp/quality_source_scope_test.go, quality_contract_test.go, quality_helpers_test.go 및 관련 response contract fixture, .issueops/PROJECT_AUDIT.md(형식 설명에 꼭 필요한 경우만). TESTING.md, testing/self-verification.md, testing/unit-and-contract.md는 다른 owner 소유다.
- 완료 범위: 구현·검증·PR 발행·complete; root가 승인된 머지와 post-merge 절차를 감독한다. 기존 다른 변경을 되돌리지 않는다.

## 문제와 성공 기준

source.go:67-99의 CollectAuditItems는 파일 없음에 nil,nil을 반환하고 Priority를 네 번째 열로 읽는다. application/quality/inspect.go:95-135는 warnings를 그대로 연결하며 domain/quality/policy.go:196-208은 warning이 있으면 collection_status=error, health_status=unknown, gate_status=block으로 이미 판정한다. 실제 PROJECT_AUDIT.md Summary Matrix의 Open은 명시적 None이고 Accepted/Resolved/Out-of-scope 이력 표는 Priority 열을 생략한다. 기존 qualitycli fixture :450은 헤더 없는 행이라 변경된 엄격 계약에 맞게 헤더를 추가한다.

성공: 실제 감사 문서 정상 0건; 순서가 다른 유효 헤더의 P0/P1/P2 정확 집계; 유효 P3는 제외; 누락·빈 파일·알 수 없는 형식·잘못된 표는 경고와 unknown/block; 이력 제외; JSON/MCP schema와 AuditItems 함수 signature 유지.

## 적용되는 결정과 주의사항

- AGENTS.md §2 Simplicity First, §3 Surgical Changes: collector의 작은 수정과 실제 결함 테스트만 만든다.
- .issueops/CONSTITUTION.md 제2장 안전/정확성: 읽지 못한 입력을 성공으로 처리하지 않는다.
- .issueops/ARCHITECTURE.md와 .issueops/CONVENTIONS.md 공용 Go core/port 경계: filesystem Markdown 처리는 기존 outbound adapter에 남기고 정책 상태를 중복 구현하지 않는다.
- .issueops/ADR.md Accepted baseline 및 adr/decisions/2026-05-25-plugin-vs-external-worker.md: host adapter에 정책을 복제하지 않고 기존 Go 품질 계산을 사용한다. 현재 MCP catalog에는 quality inspect 도구가 없으므로 신규 도구를 만들지 않고 catalog golden 불변만 확인한다.
- .issueops/CAUTIONS.md 및 cautions/audit-and-process.md §10/18/22: named RED부터 시작하고 nonzero JSON 실패 정보를 보존한다; 현재 공개 CLI 계약만 사용한다.
- .issueops/TESTING.md: focused RED→GREEN→SURFACE→CLEAN 후 completion self-verify 계약을 적용한다. 문서 전반 정리는 다른 독립 사이클 소유다.

## 재사용하는 기존 구현

CollectAuditItems, splitMarkdownRow 및 contract.AuditItem을 확장한다. warnings 반환, Inspect의 signal error, QualityStatuses의 unknown/block을 그대로 쓴다. 신규 DTO/port/status enum/Markdown 라이브러리를 만들지 않는다. 기존 qualitycli 테스트 helpers와 dependency injection으로 다른 collector를 정상 stub하고 audit collector만 실제 구현으로 연결해 원인을 격리한다.

## 설계

1. 파일 읽기 실패는 파일 없음도 audit scan warning으로 반환한다. warning은 문서 경로와 이유를 포함하되 문서 내용을 통째로 출력하지 않는다.
2. 표는 헤더 이름으로 열 index를 매핑한다. 기존 ID/Area/Title/Priority/Size 다섯 열이 모두 있고 중복되지 않아야 하며 순서 변경·추가 열을 허용한다. 각 열 이름의 대소문자/주변 공백만 정규화한다. speculative alias는 추가하지 않는다.
3. 헤더 다음 Markdown separator를 확인하고 각 데이터 행 열 수를 header와 대조한다. ID/Area/Title/Priority/Size의 빈 필수 셀, 잘못된 우선순위, 헤더 중복·누락, separator 누락은 warning으로 드러낸다. P0/P1/P2는 수집하고 P3는 유효하지만 집계하지 않는다. 표가 일부 유효해도 warning이 있으면 오류 상태다.
4. heading이 있는 문서는 현재 Summary Matrix/Open 범위의 명시적 no-open-items 문장(`_None. All triaged P1/P2 items are resolved or accepted-with-rationale below._`) 또는 유효 열린 표를 인정한다. 형식을 전면 마이그레이션하지 않도록 현재 문장을 그대로 지원한다. 열린 항목 범위는 Open heading에서 동급/상위 heading 직전까지다. Accepted/Resolved/Out-of-scope 영역은 헤더에 Priority가 있어도 이력으로 제외한다. section 상태의 구현은 작은 local scan state로 충분하다.
5. heading 없는 문서도 기존 유효 header+separator 감사표를 인정한다. 명시적 None이 없는 빈 문서·텍스트만 있는 문서·헤더 없는 행·인식할 수 없는 표는 warning이다. heading으로 Open 범위가 정해진 문서는 다른 범위의 유효 표로 잘못된 Open 영역을 정상 처리하지 않는다. 명시적 None 뒤에 P0/P1/P2 열린 행이 있으면 문서가 모순이므로 warning이다.
6. 실제 문서에서 역사적 Resolution Plan 같은 다른 표·P1/P2 문구는 열린 감사 항목으로 세지 않는다. PROJECT_AUDIT.md 원문은 가능하면 수정하지 않는다.

## 실행 단계와 검증

1. RED: adapter에 TestCollectAuditItems… fixtures를 추가하고 현재 코드에서 열 순서 변경, missing, malformed 중 적어도 하나가 실패하는 명령과 출력을 기록한다. 실제 repo PROJECT_AUDIT.md를 읽는 회귀 검증도 추가해 0 items/0 warnings를 확인한다.
2. GREEN: 설계에 따른 좁은 parser 변경. 정상 표, reorder/extra column, P0/P1/P2/P3, actual current zero, None+open row 모순, missing/empty/unrecognized, malformed separator/width/required cell/duplicate header/invalid priority, history section 제외를 테스트한다. 모든 테스트가 parser 구현을 mock하지 않고 파일을 통해 관측한다.
3. SURFACE: application/quality 또는 qualitycli에서 실제 audit collector+정상 stub 다른 collectors로 missing/malformed -> result.OK=false, collection=error, health=unknown, gate=block, audit signal=error, warning 비어 있지 않음을 확인한다. real current document -> 감사 signal ok/0을 확인하며 다른 기존 품질 부채까지 healthy를 주장하지 않는다. 공개 CLI quality 결과를 최소 smoke 또는 기존 RunInspect 테스트로 확인한다. 현재 MCP에는 quality inspect 도구가 없으므로 MCP catalog/schema 불변은 TestMCPToolsGolden로 확인한다. cmd/issueops/issueopsapp/quality_source_scope_test.go, quality_contract_test.go, quality_helpers_test.go의 실제 collector composition fixture에 유효 zero 감사 문서를 명시적으로 준비한다. 기존 성공 assertions를 오류 허용으로 완화하지 않는다. response 계약 dependency도 같은 유효 fixture를 사용하게 한다.
4. CLEAN/docs: 실제 diff ai-slop-clean, focused 재검증; 구현과 읽은 project docs를 대조해 no-change 근거 또는 필요한 좁은 문서 반영을 기록한다. strict readiness와 independent diff review pass 뒤 commit/push·PR·complete로 진행한다.

G1: 감사표 parser 정상/오류 구분 | CHECK: go test ./internal/adapter/outbound/quality -run TestCollectAuditItems -count=1 | EXPECT: ok
G2: collector 오류가 품질 상태로 전달 | CHECK: go test ./internal/application/quality ./internal/domain/quality ./cmd/issueops/qualitycli -count=1 | EXPECT: ok
G3: 기존 품질 패키지 회귀와 race | CHECK: go test -race ./internal/adapter/outbound/quality ./internal/application/quality ./internal/domain/quality ./cmd/issueops/qualitycli -count=1 | EXPECT: ok

G4: 공개 composition 계약 유지 | CHECK: go test ./cmd/issueops/issueopsapp -run 'TestQualityProductionSourceWiringAndCLI|TestQualityContractFixtureSeparatesHealthFromCollection|TestResponseContractsGolden' -count=1 | EXPECT: ok
G5: MCP catalog/schema 불변 | CHECK: go test ./cmd/issueops/contractgolden -run '^TestMCPToolsGolden$' -count=1 | EXPECT: ok

## 성능 영향

품질 inspect의 감사 파일 1회 읽기와 선형 scan을 유지한다. 시간 O(document bytes), 기존 strings.Split 수준 메모리; 헤더 map은 한 표의 열 수에 비례한다. 추가 파일 조회·git/provider 호출·cache 없음. 성능 개선 수치를 주장하지 않는다.

## 하위 호환성과 side effect

AuditItems signature, AuditItem/InspectResult JSON, CLI flags, MCP tool schema, state schema와 remote body 계약을 바꾸지 않는다. 엄격화 때문에 헤더 없는 표·감사 파일 없는 repo는 종전의 성공 0건에서 수집 오류로 바뀌며 이것이 요청된 동작이다. 정상 5열 표의 열 순서 변경·추가 열은 허용한다. 실제 감사 문서의 역사와 명시적 zero를 보존한다. 운영 파일/원격/Git mutation은 collector가 하지 않는다. 신규 dependency와 DB migration 없음. 롤백은 이 PR의 parser/test commit을 되돌리는 것으로 충분하다.

Repo grounding: source.go CollectAuditItems, application Inspect, domain QualityStatuses, qualitycli fixture, actual PROJECT_AUDIT, required docs.
Decision-complete plan: 기존 warning 통로; 이름 기반 5열 표 검증; 현재 Open None 인정; history 제외.
Assumptions/defaults: core 기존 schema 유지, 한 독립 cycle owner, root supervised Orca launch.
Unresolved questions: none blocking.
Acceptance criteria: G1–G5와 실제 문서/CLI 품질 결과 및 MCP catalog 불변.
