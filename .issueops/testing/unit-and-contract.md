# Unit, integration, fixture, golden, and contract tests

[← TESTING.md](../TESTING.md) owns the test-strategy index and minimum completion
gate. This module owns unit/integration/fixture/golden/contract standards and
the Go-code basic verification that backs them. Race, process, lock, and
nondeterminism procedures live in [concurrency-and-race.md](concurrency-and-race.md);
CLI/MCP/host parity lives in [cli-mcp-and-hosts.md](cli-mcp-and-hosts.md); the
IssueOps execution vertical contract lives in
[issueops-execution.md](issueops-execution.md).

## Go 코드 변경 기본 검증

Go 코드를 추가하면 다음 기본 검증을 실행한다. 최종 검증 battery에서 self-verify가 실제로 실행했거나 명시적으로 재사용한 항목은 중복 실행하지 않는다. `risk QA tier`는 선택한 `base_ref`의 tree부터 관측 HEAD tree까지의 tracked diff와 staged·unstaged·untracked 경로를 합쳐 기존 분류기로 판정한다. 기준을 생략한 standalone 호출은 working-tree 범위를 유지한다. IssueOps 사이클은 exact record의 `branch_prepare.base_sha`를 전달한다. 잘못된 기준이나 Git 관측 실패는 성공 또는 full-suite coverage로 처리하지 않는다. 검증 범위에 Go 변경이 있는데 risk tier가 `go vet ./...` 또는 `go test -race ./... -count=1`를 실제 실행하지 않았으면 누락된 명령은 별도 실행한다.

```bash
gofmt -l $(git ls-files '*.go')
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/issueops ./cmd/issueops
```

`gofmt -l`은 CI의 Format check와 동일한 파일 집합(`git ls-files '*.go'`)을 검사하며, 출력이
없어야 통과한다. `self-verify`의 `gofmt` 단계도 working tree 변경 여부와 관계없이 이 검사를
항상 실행한다(`risk QA tier`는 변경분에만 반응한다). CI는 gofmt가 실패하면 이후
test/race/self-verify 결과를 표시하지 않으므로, 로컬 검증 게이트도 CI와 동일해야 한다
(2026-08-26 lesson).

`go vet`은 harness의 기본 정적 분석 게이트이며, CI는 `.golangci.yml` 설정에 따라 그 위에
golangci-lint v2.12.2도 실행한다. linter 집합은 결함, 죽은 코드, 불필요한 추상화, 표준 대체가
있는 옛 관용구, exported API의 문서 누락을 잡는 것만 고른다.

- 결함: asasalint, bidichk, bodyclose, durationcheck, errcheck, errorlint, fatcontext,
  govet(fieldalignment·shadow를 뺀 전체 analyzer), makezero, nilnesserr, reassign,
  rowserrcheck, sqlclosecheck, staticcheck(SA·S1)
- 죽은 코드·불필요한 추상화: iface(같은 메서드 집합의 interface 중복), ineffassign, unparam,
  unused, wastedassign
- 옛 관용구: copyloopvar, intrange, modernize, usestdlibvars, usetesting
- 위생: misspell, nolintlint(`//nolint`에 linter 이름과 사유를 모두 적는다)
- 문서: revive의 `exported`(exported 식별자는 이름으로 시작하는 doc comment를 갖고, 이름이 패키지
  이름을 반복하지 않는다)와 `package-comments`(패키지마다 package comment 하나) 규칙만 켠다.
  테스트 파일은 제외한다. 이 두 규칙을 숨기는 `comments` 제외 preset은 쓰지 않는다
  ([2026-11-05 ADR](../adr/2026-11-05-gate-exported-api-doc-comments-with-revive.md)).

revive의 나머지 규칙과 다른 문체·미세 성능 linter(gocritic, perfsprint, predeclared, stylecheck(ST),
quickfix(QF))는 버그를 잡지 않고 코드만 흔들어서 켜지 않는다. unconvert는
`syscall.Stat_t` 필드 타입이 darwin과 linux에서 달라 한쪽에 필요한 변환을 다른 쪽에서 지적하므로
켜지 않는다. 테스트 파일에서는 errorlint와 unparam을 끈다. 테스트는 오류 동일성을 일부러
`==`로 비교하고, fake는 주입 시그니처를 따라야 하기 때문이다. 의도한 예외는 설정을 넓히지 않고
해당 줄에 사유를 붙인 `//nolint:<linter> // <사유>`로 남긴다. 나머지 제외 preset과 전체 issue
출력 설정은 유지한다. 로컬 린터는 CI와 같은 `scripts/lint-local.sh`로 실행한다. 이 스크립트가
버전(v2.12.2)을 고정하고 `go.mod`의 toolchain(`GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')"`)으로
`golangci-lint run ./...`을 실행한다. CI runner가 Linux이므로 Linux가 아닌 머신에서는 `GOOS=linux`
변형도 함께 실행한다(플랫폼 build tag 분기에서만 unused가 되는 심볼을 찾는다). 린터는 해당 Go 버전
이상으로 빌드해야 하며, 더 최신인 로컬 Go의 표준 라이브러리를 구버전 analyzer에 넘기지 않는다.
이 도구는 install/update/self-verify
readiness 경로에는 필요하지 않고, CI와 개발자 게이트에서만 사용한다.

작은 변경이면 targeted test를 먼저 실행한 뒤, 완료 전에 영향 범위에 맞춰 전체 테스트를 실행한다.

### Architecture fitness ratchet

```bash
go test ./internal/architecture -count=1
```

이 테스트는 `go list -json ./...`의 direct production import inventory를 두 번 수집해 정렬 결과가 byte-stable인지 확인한다. synthetic case는 rule name과 `importer -> imported` 진단을 고정하고, real graph는 unconditional layer rule과 sorted legacy baseline의 new/stale edge를 함께 검증한다.

### Operational-health and stability delegation

- Pure classifier tests verify invocation-only preservation, duplicate/incomplete inventory failures, and exact resource ownership.
- Stability audit unit tests must verify that it calls the freshly built top-level `doctor`, requires exit zero plus JSON `ok=true` and `healthy=true`, and stores only bounded issue code/summary failure details.
- Final live reconciliation verification runs `python3 skills/stability-audit/scripts/e2e_stability_audit.py --cleanup-stale --json` only after the external recovery manifest/journal is sealed and cleanup readbacks are complete. Reset ambiguity follows forward recovery, never an inferred rollback.

domain/application 패키지를 변경하면 최소한 다음 targeted 검증을 실행한다:

```bash
go test ./internal/domain/... ./internal/application/... -count=1
go test ./cmd/issueops/issueopsapp -count=1
```

CLI/MCP contract를 의도적으로 바꾼 경우에만 golden 파일을 갱신한다. Codex/Claude native 설치 adapter 계약을 바꾼 경우에는 adapter matrix golden도 함께 갱신한다.

```bash
go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1
go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -update-adapter-contract -count=1
```

## 테스트 작성 기준

### Well-structured tests

- 변경된 공개 동작과 계약을 직접 검증하되, 구현 세부사항에 과도하게 의존하지 않는다.
- 실패 시 원인을 좁힐 수 있는 fixture 이름, assertion, 에러 메시지를 둔다.
- 저장소 내부 심볼을 찾는 contract는 indexed repo에서 CodeGraph를 우선하고, 비인덱스 repo에서만 `rg`나 직접 읽기를 사용한다. local symbol discovery에 web search를 fallback으로 사용하지 않는다.
- regression test에는 재발했던 입력, false case, 기대 결과를 명확히 담는다.
- 기존 helper와 style을 재사용하고, golden/snapshot 변경은 의도와 범위를 설명한다.

### Poorly-structured tests

- 실제 요구사항과 무관한 내부 구조만 고정한다.
- 통과를 위해 production behavior를 약화하거나 오류 처리를 숨긴다.
- 실패해도 원인을 알 수 없는 거대한 fixture나 broad snapshot만 둔다.
- 테스트 이름이 “works” 수준이라 어떤 계약을 지키는지 알 수 없다.

- core policy와 adapter transport를 분리해서 테스트한다.
- CLI/MCP/worker가 같은 core DTO를 사용하는지 contract test로 확인한다. 설치 경로는 `core.InstallNative` + `port.HostInstaller` adapter 단위 테스트와 `internal/adapter/testdata/native_install_contract_matrix.golden.json` matrix fixture로 고정한다.
- command execution은 실제 위험 명령 대신 fake runner로 검증한다.
- fake는 대상 게이트와 **같은 fail-closed 규율**을 따른다. 모르는 입력에 성공을 반환하는 fake는 새 검사를 우회시킨다. #153에서는 `fakeRemoteBranchGit`의 default가 exit 0을 반환해 신규 ancestry 검사가 조용히 통과했다. 게이트가 fail-closed인데 fake가 fail-open이면 테스트가 게이트를 무력화한다. 처리하지 않는 입력은 명시적으로 실패시킨다.
- 픽스처가 **실환경 순서를 재현하는지** 확인한다. #149의 브랜치 충돌 사전 확인은 로컬 브랜치를 만드는 픽스처로 GREEN이 되었지만, 당시 IssueOps 정식 순서였던 `gh issue develop`은 원격 브랜치만 만들었으므로 실환경에서는 우회되었다. #176은 그 단계를 `createLinkedBranch`로 대체했으며, 원격만 만드는 특성은 같다. 픽스처가 만든 상태가 사용자가 실제로 도달하는 상태와 같은지 확인한다. 통과하는 테스트가 올바른 테스트라는 뜻은 아니다.
- **shape 불변식 테스트는 우연한 수치를 고정하지 않는다.** #176에서 `TestStepsKeepTheirExistingShape`가 "3단계"를 불변식으로 검사했지만, 실제 계약은 첫 단계가 MCP이고 마지막이 `fail`이며 사이에 provider CLI `fallback_api`가 오고 `Order`가 연속이라는 것이었다. 단계를 하나 늘리는 정당한 변경이 그 테스트를 깨뜨렸다. 이름이 "shape"인 테스트는 구조를 검사하고, 개수·인덱스는 그 구조가 요구할 때만 고정한다.
- filesystem test는 temporary directory를 사용하고, workspace root 밖 접근 거부를 검증한다.
- secret redaction test는 token-like fixture가 로그/응답에 남지 않는지 확인한다.
- Python 검사는 `python3 scripts/python_suite_runner.py`가 소유한다. root `scripts/*_test.py`를 한 번 실행하고, 각 스킬의 `scripts/`·`tests/`에서 `test_*.py`·`*_test.py`를 중복 없이 찾아 별도 interpreter process로 실행한다. CI는 self-verify의 같은 단계를 사용하며 별도 root suite를 중복 실행하지 않는다. `verify-skill-shell.py`는 스캔 루트 바로 아래에서 `SKILL.md`가 있는 디렉터리를 가리키는 심링크만 로컬 외부 스킬 링크로 분류해 건너뛴다(설치기와 `inspect`의 `ListSkillNames`도 같은 규칙을 사용한다). 중첩 심링크와 `SKILL.md`가 없는 링크는 계속 `symlink-not-allowed` 위반이다.
- shipped skill의 executable shell fence는
  `python3 scripts/verify-skill-shell.py`로 syntax, swallowed failure,
  fabricated zero, unsafe command expansion/word splitting, destructive
  annotation을 검사한다. 설명용 Markdown `text` fence는 실행 대상으로 보지 않는다.
  명시한 input path가 없거나 skill contract Markdown을 찾지 못하면 exit 2로 실패하며,
  `--help`는 usage를 출력한다. CI path typo는 `0 file(s)`로 성공 처리되지 않는다.
- Parent issue create 회귀 테스트는 provider 호출 전 intent 존재, concurrent begin
  차단, proven non-invocation만 retry, started/unknown 자동 재시도 차단,
  zero/one/many marker candidate, delayed candidate, title/body digest mismatch,
  live verification failure, URL+receipt atomic write, dry-run no-mutation을
  검증한다. Provider command test는 start failure와 post-start ambiguity를
  `IssueProviderCreateError.Invoked`로 구분한다.

### Python 검사 환경

Slack 스킬 테스트에 필요한 기존 PEP 723 의존성은
`scripts/python_test_requirements.txt`에 고정한다. 개인 uv cache 경로를
`PYTHONPATH`에 넣거나 import 실패를 skip으로 바꾸지 않는다.
root suite의 기존 optional local-file skip은 unittest 의미대로 유지하지만,
스킬의 import 오류·빈 테스트 파일·실패·skip은 검사 실패다.

```bash
python_env="$(mktemp -d)"
uv venv --python 3.13 "$python_env"
uv pip install --python "$python_env/bin/python" -r scripts/python_test_requirements.txt
PATH="$python_env/bin:$PATH" python3 scripts/python_suite_runner.py
PATH="$python_env/bin:$PATH" ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

CI도 `RUNNER_TEMP`의 환경을 사용하여 checkout이나 사용자 홈을 오염시키지 않는다.
이는 저장소 테스트 환경 준비이며 CLI/MCP의 운영 의존성이나 native 설치기의
자동 설치 항목이 아니다.

### route-docs 순위 측정

`project route-docs`의 순위 규칙(`internal/domain/projectdoc/records.go`)을 바꾸기 전과 후에 `go build -o /tmp/issueops ./cmd/issueops && python3 scripts/route_docs_eval.py measure --binary /tmp/issueops`를 실행하고, 두 번의 recall@5와 precision을 커밋 Lore `Verify`에 함께 적는다. 이 도구는 `testdata/route-docs/plan-citations.json`(커밋된 계획이 인용한 cautions·adr 기록)을 기준으로 삼는 측정 도구이며 CI 게이트가 아니다. 기록이 자주 늘어나므로 절대값은 달라지고, 같은 저장소 상태에서 변경 전후를 비교할 때만 의미가 있다. fixture는 `python3 scripts/route_docs_eval.py build-fixture`로 다시 만든다. 점수 계산과 fixture 파싱은 `scripts/route_docs_eval_test.py`가 가짜 runner로 검증한다.

### OS 중립 fixture (Windows)

`go test ./...`는 darwin·linux·windows에서 모두 통과해야 한다. fixture는 `internal/testsupport`의 helper로 OS 차이를 흡수한다.

- 디스크에 만들지 않는 절대경로 fixture는 `testsupport.AbsPath(...)`를 쓴다. `/repo/...` 문자열은 Windows에서 절대경로가 아니다.
- fake `gh`·`glab`·`git` 같은 명령은 `testsupport.WriteFakeCommand(t, dir, name, "#!/bin/sh\n...")`로 설치한다. Windows에서는 Git for Windows bash가 스크립트를 실행하는 `name.exe` shim이 생기고, argv(따옴표·`@{u}`·glob·줄바꿈 포함), stdin, 종료 코드가 그대로 전달된다. 스크립트에는 shebang을 둔다.
- 홈 격리는 `testsupport.SetHome`을 쓴다(Windows는 `USERPROFILE`도 바꾼다).
- owner-only 단언은 `osfile.Perm(path, info)`로 한다. 공개 권한이나 읽기 불가 fixture는 `testsupport.MakePermissive`, `testsupport.MakeUnreadable`로 만든다. `os.Chmod`는 Windows에서 읽기 전용 속성만 바꾼다.
- 경로를 이어 붙여 JSON·TOML을 만들지 않는다(`json.Marshal`, `TOMLString`).
- Windows 의미가 실제로 다른 경우에만 skip하거나 build tag로 제한하고, 이유를 skip 메시지에 적는다. 예: 사용 중인 cwd 삭제, 파일 아래 경로가 `ENOTDIR`가 아닌 not-exist로 보고됨, cmux(Unix socket 앱), `--adopt-command-file`, `scripts/*.sh` 실행 테스트.
- Windows 테스트는 owner-only ACL을 가진 `TMP`/`TEMP`에서 돌린다. 사용자 프로필 밖의 `TMP`가 다른 principal에게 상속 ACE를 주면 owner-only 검사가 실패한다.

## Contract / Golden tests

다음 항목은 golden test 대상으로 삼는다.

- `issueops inspect --json` output shape
- `issueops docs --json` output shape
- `issueops policy check/fake-run` allow/deny/fake execution output shape
- `issueops guard check` portable anti-pattern output shape
- MCP tool schema와 response shape
- in-process MCP smoke response
- `cmd/issueops/testdata/usage.golden.txt`
- `cmd/issueops/testdata/mcp_tools.golden.json`
- `cmd/issueops/testdata/mcp_resources.golden.json`
- `cmd/issueops/testdata/response_contracts.golden.json`
- `issueops loop start/record-attempt/status/stop` CLI/MCP schema and response-contract entries
- `internal/adapter/testdata/native_install_contract_matrix.golden.json` — Codex/Claude user-global 기본 설치와 project-local opt-in 계약
- `issueops self-verify` 10회 반복 결과
- `issueops self-verify`의 `risk QA tier` step과 `risk_qa` goal score
- `issueops self-verify --json`의 `summary.contract`/`goal_scores`/`coverage_gaps`/`failure_class`/`rerun_commands`/`step_duration_stats` field
- `issueops self-verify candidates --json` candidate curriculum export and state save/read smoke
- `issueops self-verify compare` step budget p95 regression fixture for labels outside `slowest_steps`
- `issueops self-verify` install dry-run smoke for temp HOME/CODEX_HOME/ISSUEOPS_ROOT no-write assertions
- `scripts/release-repro-smoke.sh` clean-machine release install reproducibility smoke
- `scripts/release-build-matrix.sh` cross-platform release build matrix smoke
- `issueops self-verify --save-state` summary checkpoint serialization
- `issueops self-verify history` summary checkpoint discovery and retention dry-run/confirm safety
- `issueops self-verify` native integration fixture for Claude MCP conflicting-scope warning classification
- `issueops self-verify compare` summary checkpoint regression comparison
- `issueops self-verify promote` dry-run/confirm baseline promotion
- `issueops self-augment --json` planner/candidate curriculum
- `issueops inspect/doctor/docs/preflight/policy/state` 실제 JSON response normalization 결과
- `issueops doctor --json` comprehensive diagnostics output shape
- `issueops state write/read/list/prune/doctor/maintain` output shape
- `issueops prune` unreadable-row total과 bounded `{id,code}` diagnostics
- `issueops remote create-issue/reconcile-issue` intent and adoption fields
- command policy allow/deny 결과
- command policy catalog table과 `CommandPolicySummary().catalog` 노출
- state write/read/prune/doctor/maintain serialization and WAL/permission repair
- redaction 결과

Golden file은 읽기 쉽게 작게 유지하고, schema를 변경하면 의도와 migration을 문서화한다. 실제 CLI/MCP JSON response golden은 timestamp, temp path, audit id, git sha, home/harness path를 `$TIMESTAMP`, `$STATE_DIR`, `$WORKSPACE`, `$GIT_REPO`, `$GIT_SHA`, `$HOME`, `$ISSUEOPS_ROOT`, `$AUDIT_ID` 같은 placeholder로 정규화한 뒤 비교한다.

## Contract/audit/worker verification

CLI/MCP DTO를 변경하면 `issueops contract check --json`과 golden test를 실행해 command name, MCP tool name, required response field가 기계가 읽을 수 있는 형태로 유지되는지 확인한다. policy audit 동작을 변경하면 JSONL record가 append-only인지, secret-like argument가 redacted되는지 검증한다. generic worker를 변경할 때는 no-shell MVP 범위인 enqueue/status/list/cancel을 테스트한다.

## Lifecycle state tests

- project lifecycle namespace tests must use `ISSUEOPS_STATE_DIR` with `t.TempDir()` and must not write runtime state under the target repo.
- bootstrap tests should verify dry-run plans `projects/<repo-id>/project.json` without creating it, and normal `project bootstrap` writes lifecycle profile metadata in user-state.
- hook tests should cover fallback behavior when lifecycle state is missing/corrupt so prompt routing remains useful.
- doctor tests should cover repo-local `.issueops/state/` and namespace mismatch warnings.
