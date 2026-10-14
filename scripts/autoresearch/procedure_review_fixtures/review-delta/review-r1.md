# 1라운드 계획 리뷰 결과 (새 컨텍스트 서브에이전트, 판정: revise)

대상: 같은 디렉터리의 `plan.md`(이슈 #12, `gates check` 실패 게이트 목록).

## 필수 결함 1: 필드 이름이 기존 집계 필드와 어휘가 다르다
- 위치: 변경 1(`FailedGates`/`failed_gates`)과 그 이름을 쓰는 모든 곳.
- 발생 조건: `CheckResult`의 기존 집계는 `total_unmet`이고 사람 출력도 `UNMET:`이다. 같은 집합을
  `failed_gates`라고 부르면, CHECK 실행 오류(`check_error`)가 난 게이트만 모은 목록으로 읽힌다.
- 위반한 계약: 계획 자신이 "판정 기준은 `Summarize`와 같은 상태 집합"이라고 적었다. 같은 집합에는 같은 어휘를 쓴다.
- 최소 수정: JSON 필드를 `unmet_gates`, Go 필드를 `UnmetGates`로 바꾼다. 사람 출력 줄은 `UNMET GATES: <목록>`으로 한다.

## 필수 결함 2: G2가 CLI JSON 출력을 관찰하지 않는다
- 위치: 수용 기준 G2.
- 발생 조건: G2는 사람 출력 테스트만 돌린다. `--json` 응답에 새 필드가 실제로 실리는지는 어느 게이트도 보지 않는다.
- 최소 수정: G2를 다음으로 바꾼다. `CHECK: issueops gates check --dry-run --json --file testdata/unmet.md`, `EXPECT: /"unmet_gates"/`.
  `--dry-run`이면 CHECK를 실행하지 않고 원장 상태만 계산하므로 테스트용 원장으로 안전하게 확인할 수 있다.

## 필수 결함 3: G1이 verbose가 아닌 실행에서 실패한다
- 위치: 수용 기준 G1.
- 발생 조건: CI가 `-v` 없이 같은 테스트를 돌리면 `--- PASS:` 줄이 나오지 않는다.
- 최소 수정: G1 EXPECT를 두 형식을 모두 받는 정규식으로 바꾼다:
  `- G1: 목록 테스트 | CHECK: go test ./internal/domain/gates/ -run TestAddFileResultListsUnmetGates -count=1 | EXPECT: /^ok|--- PASS: TestAddFileResultListsUnmetGates/m`
