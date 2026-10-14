# 계획: `gates check`에 실패 게이트 목록 추가 (#12)

## 문제
`issueops gates check`는 미충족 게이트 개수(`total_unmet`)만 보여 준다. 어느 원장의 어느 게이트가
남았는지 보려면 `--json`의 `files[].gates[]`를 직접 훑어야 한다.

## 변경
1. `internal/contract/gates/types.go`의 `CheckResult`에 선택 필드 `FailedGates []string`(`json:"failed_gates,omitempty"`)를
   추가한다. 값은 상태가 `unchecked`나 `evidence_pending`인 게이트마다 `<file>:<gate id>`다. `abandoned`는 해결된
   게이트로 세는 기존 규칙(`Summarize`)을 따라 넣지 않는다.
2. `internal/domain/gates/check_result.go`의 `AddFileResult`가 위 목록을 채운다. 판정 기준은 `FileResult.Gates[].State`
   하나다(`Summarize`와 같은 상태 집합).
3. `cmd/issueops/gatescli/gates.go`의 사람 출력은 `UNMET:` 줄 뒤에 `FAILED: <목록>` 한 줄을 더한다. exit code와
   `ALL MET` 출력은 그대로다.
4. `.issueops/operations/guides/cli-and-mcp.md`의 Task Gate Ledgers 절에 `failed_gates` 필드와 `FAILED:` 줄을 한 문장으로
   적는다. MCP `gates_check`는 같은 DTO를 쓰므로 같은 문장이 두 표면을 함께 설명한다.

## 적용되는 결정과 주의사항
- `.issueops/conventions/cli-mcp-and-output.md` / CLI 컨벤션: "`--json` 출력 field는 snake_case를 사용한다." → 필드 이름은 `failed_gates`.
- `.issueops/conventions/cli-mcp-and-output.md` / CLI 컨벤션: "exit code는 의미 있게 유지한다" → `UnmetError`와 exit code는 바꾸지 않는다.
- `.issueops/testing/unit-and-contract.md` / Go 코드 변경 기본 검증: "`gofmt -l`은 CI의 Format check와 동일한 파일 집합(`git ls-files '*.go'`)을 검사하며, 출력이 없어야 통과한다." → G3.
- `.issueops/testing/unit-and-contract.md` / Contract / Golden tests: golden 대상에 "MCP tool schema와 response shape"와 `cmd/issueops/testdata/response_contracts.golden.json`이 있다. → 선택 필드 추가라 기존 필수 필드는 그대로지만 G5로 golden 테스트를 실행해 확인한다.
- `.issueops/conventions/cli-mcp-and-output.md` / MCP 컨벤션: "CLI와 MCP는 같은 core request/response DTO를 공유한다." "schema 변경은 golden test와 문서 업데이트를 동반한다." → 변경 4와 G5.
- `.issueops/CONVENTIONS.md` / 이슈 산출물 레이아웃: "`gates check` CLI는 계속 전부 보고한다." → 다른 번호의 원장도 목록에 포함한다.
- `.issueops/ARCHITECTURE.md`: 대조했으나 없음(새 패키지·책임 이동 없음).
- `.issueops/adr/2026-08-22-task-gate-ledger.md` / 결정 4: "CLI와 MCP가 같은 contract DTO를 공유한다." `internal/contract/gates` 타입, 스키마 버전 1, exit code 0/1/2. Consequences: "MCP tools/tools.golden/response_contracts.golden/omo 카탈로그 다이제스트가 갱신된다(스키마 변경 절차 준수)." → `failed_gates`는 같은 DTO의 선택 필드로 넣고 스키마 버전과 exit code는 그대로 둔다. G5가 실패하면 의도된 변경인지 확인한 뒤에만 `cli-mcp-and-output.md`의 `-update` 절차로 golden을 갱신한다.
- `.issueops/CAUTIONS.md` / 게이트 원장: CHECK는 한 줄 argv이고 exit 0이며, 리터럴 EXPECT는 출력 한 줄과 같거나 그 줄이 EXPECT로 시작하고 바로 뒤가 공백일 때 맞는다. → 테스트 게이트는 `-v`의 `--- PASS:` 줄을, gofmt 게이트는 저장소에서 쓰는 `GOFMT_CLEAN` 출력 형식을 EXPECT로 쓴다.
- `.issueops/CONSTITUTION.md`: 대조했으나 없음.

## 재사용하는 기존 구현
- `gatesdomain.Summarize`의 상태 분류(`met`/`unchecked`/`evidence_pending`/`abandoned`)를 그대로 쓴다. 새 판정 함수는 만들지 않는다.
- 테스트는 기존 `TestSummarizeFileAndAddFileResultAggregateTotals`의 원장 fixture 형식을 따른다.

## 성능 영향
게이트 수에 선형인 순회 한 번이 `AddFileResult`에 더해진다. 원장당 게이트는 수십 개 이하라 측정하지 않는다.

## 하위 호환성과 side effect
- `failed_gates`는 `omitempty` 선택 필드라 모든 게이트가 met이면 JSON이 지금과 같다. 기존 필드 이름·의미·exit code는 그대로다.
- 사람 출력은 미충족이 있을 때만 한 줄 늘어난다. 이 줄을 파싱하는 내부 호출자는 없다(`rg 'UNMET:'` 결과 `gatescli` 한 곳).
- record schema와 상태 저장은 바뀌지 않는다. 롤백은 커밋 되돌리기로 충분하다.

## 수용 기준
- G1: `go test ./internal/domain/gates/ -run TestAddFileResultListsFailedGates -count=1 -v` → EXPECT `--- PASS: TestAddFileResultListsFailedGates`
  (unchecked·evidence_pending은 목록에 들어가고 met·abandoned는 빠지는지, 여러 원장이 `<file>:<id>`로 구분되는지)
- G2: `go test ./cmd/issueops/gatescli/ -run TestReportPrintsFailedGatesLine -count=1 -v` → EXPECT `--- PASS: TestReportPrintsFailedGatesLine`
  (미충족이 있으면 `FAILED:` 줄이 `UNMET:` 줄 다음에 오고, 모두 met이면 없는지)
- G3: `python3 -c "import subprocess as s;f=s.run(['git','ls-files','*.go'],capture_output=True,text=True).stdout.split();p=s.run(['gofmt','-l']+f,capture_output=True,text=True);print('GOFMT_CLEAN' if p.returncode==0 and not p.stdout.strip() else 'DIRTY'+chr(10)+p.stdout+p.stderr)"` → EXPECT `GOFMT_CLEAN`
- G4: `go test ./internal/domain/gates/ ./cmd/issueops/gatescli/ -count=1` → EXPECT `ok`
- G5: `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1` → EXPECT `ok`
