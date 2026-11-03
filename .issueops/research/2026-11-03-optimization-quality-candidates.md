# Research: issueops 최적화·품질 향상 후보

## TL;DR

**결론**: 이번 조사에서 issueops 개선 후보 30여 개를 찾았습니다. 가장 먼저 고칠 것은 정확성 결함 네 건입니다.

1. read-only 명령 정책이 `find -delete`, `sed -i` 같은 쓰기 인자를 막지 않습니다. 실제로 파일이 지워집니다.
2. `basiccli`의 doctor 테스트가 실제 HOME을 써서, 저장소 규칙을 어기고 CPU를 20배 더 씁니다.
3. `route-docs`의 어간 처리가 `-es`·`-ies` 복수형을 잘못 자릅니다.
4. `execution prepare` 응답이 `artifact_dir`를 빠뜨립니다.

그다음 효과가 큰 것은 검증 시간입니다. CI는 lint, race, self-verify를 job 하나에서 순서대로 돌려 성공 run 중앙값이 544초입니다. self-verify Python 단계의 절반 이상을 테스트 하나가 씁니다. 런타임 성능은 `doctor`(1.9~2.8초)를 빼면 병목이 없습니다.
**Confidence**: Medium. 저장소 안의 결함과 측정은 재현했습니다. 시간 절감 추정치는 load average 60~235인 머신에서 잰 값이라 [INFERENCE]가 많습니다.
**Sources**: 저장소 코드·문서·git 이력, GitHub Actions run 200건, `gh` 이슈·PR 데이터, 로컬 MCP 서비스 로그, 외부 README 6개를 썼습니다. 핵심 주장 11개 중 반박 검토에서 그대로 성립 5개, 보정 6개, 반박 0개입니다.

## Method

- **기준 시점**: HEAD `fc49729de`, 2026-11-03. 저장소 파일과 실제 issueops state는 바꾸지 않았습니다. 쓰기가 필요한 실험은 `/tmp/io-quality/`와 임시 `ISSUEOPS_STATE_DIR`에서 했습니다.
- **기준선**: `issueops quality inspect --repo . --json`(3분 42초 걸림)과 `issueops self-augment --json`을 먼저 실행했습니다.
- **병렬 조사 5개**
  1. 런타임 성능: CLI·MCP 지연, Go init, CPU 프로파일, 테스트 시간
  2. 정적 코드 품질: vet, golangci-lint, staticcheck, deadcode, gocyclo, dupl, 커버리지, 아키텍처 래칫
  3. 검증 파이프라인: self-verify 단계, CI 구조, Python 스위트, 스킬·문서 검사, 불안정성 기록
  4. 사용 근거: git 이력 120일, GitHub 이슈·PR, CI 실패, 실제 사이클 레코드, MCP 서비스 로그
  5. 외부 패턴: 랜드스케이프 보고서와 과거 연구의 미해결 항목, route-docs 정밀도 측정
- **반박 검토**: 조사에 참여하지 않은 새 에이전트가 핵심 주장 11개(R1~R11)를 다시 재현하고 반박을 시도했습니다.
- **도구**: 설치 없이 모듈 캐시로만 실행했습니다. golangci-lint v2.12.2(CI 핀)와 v2.15.0, staticcheck v0.8.2, deadcode v0.52.0, gocyclo v0.6.0, dupl v1.0.0을 썼습니다. 로컬 go는 1.27.1이고, `go.mod`는 `go 1.26.6`입니다.
- **측정 조건**: 측정하는 동안 load average가 60~235였습니다(8코어). 그래서 벽시계 시간보다 CPU 시간과 A/B 상대 비교를 우선했습니다.

```text
Diff inventory: research-only; no repository file changed during measurement (git status clean except this report)
SNR before/after: baseline only, quality inspect code-snr 0.6737 (signal 82,574 / total 122,570 lines)
Secondary metric: gocyclo non-test >12: 173 functions, >20: 6; dupl 88 clusters (non-test 961 lines, 0.78%)
Heuristic caveat: gocyclo/dupl are AST-based; quality inspect branch counts use a different formula and are not comparable
No-input guard: not applicable (whole-repo baseline, 1,421 non-test Go files)
```

## Findings

### 1. 기준선 수치

|항목|값|근거|
|---|---|---|
|`go vet ./...`|0건|CodeQuality|
|golangci-lint v2.12.2(CI 핀)|0 issues|CodeQuality|
|golangci-lint v2.15.0|10건(errcheck 9, modernize 1)|린터를 올릴 때 처리할 항목|
|staticcheck v0.8.2|11건(ST1019 4, ST1005 4, SA1012 3), U1000 0건|ST1005 4건은 고유명사라 오탐|
|deadcode|0건|`-test ./cmd/issueops`|
|비테스트 함수 복잡도(gocyclo)|5,583개 중 >12가 173개, >20이 6개, 최대 34|`gates.LintStatic`|
|비테스트 파일 >500줄 / >800줄|22개 / 2개|최대 `internal/domain/issueopsnext/classify.go` 1,051줄|
|커버리지|305개 패키지 평균 90.1%, <60%는 2개|`internal/adapter/provider` 21.7%, `internal/port` 33.3%|
|아키텍처 래칫|금지 adapter edge 0, 허용 예외 2종|`internal/architecture/dependency_test.go:238-243`|
|`quality inspect`|`health_status: needs_attention`, 후보 0개|소요 3분 42초|
|`self-augment` 후보|30개 모두 `already_satisfied`|자동 후보 목록이 이번 조사의 발견을 하나도 담지 못함|
|테스트 수|12,064개(패키지 397개)|`go test -json ./...`|
|CI 성공 run 소요|중앙값 544초, p90 637초|2026-10-10~11-03, 198건|
|CI step 시간|race 281초, self-verify 268초, lint 75초, vet 19초|run 41568386193|
|self-verify 내부(CI)|go test 158초(59.9%), Python 102초(38.9%), 나머지 24단계 합계 3.2초|같은 run|

정적 분석으로 찾을 수 있는 실질 결함은 없었습니다. 개선 여지는 구조, 검증 시간, 정책 경계, 관측성에 있습니다.

### 2. 정확성·안전 결함

#### 2.1 read-only 명령 정책이 쓰기 인자를 막지 않습니다

- **재현**(반박 검토 R1, 그대로 성립)
  - `policy check -- find . -delete`와 `-- sed -i .bak s/a/b/ f.txt`는 `"allowed": true`, `"tier": {"name":"read_only"}`, `"write_allowed": false`로 판정됩니다.
  - 임시 작업 공간에서 `policy run --read-only -- find ./g.txt -delete`를 실행하면 exit 0이고 파일이 실제로 지워집니다.
  - `worker run --read-only -- sed -i .bak s/a/b/ f.txt`는 파일 내용을 바꾸고 `f.txt.bak`을 만듭니다.
  - 판정만 확인한 명령: `awk BEGIN{system(...)}`, `git branch -D`, `find -exec`, `sed 1e`, `git config --global`, `git remote add`, `rg --pre`, `go env -w`, `go list -toolexec`도 `read_only`로 허용됩니다.
- **원인**: `internal/domain/policy/catalog.go:50`의 `readOnlyCommands`에 `find`, `sed`, `awk`가 들어 있습니다. `Classify`(97-108행)는 `argv[0]`의 basename만 보고, `git`과 `go`만 두 번째 토큰을 봅니다. `command_decision.go:109-115`는 인자를 검사하지 않습니다.
- **어긋나는 약속**: MCP `worker_run_read_only` 설명(`internal/contract/mcp/adapter_owned_catalog.go:26`)은 "forces write/network/shell disabled"라고 적습니다. AGENTS.md §10은 "shell 실행 기능은 allowlist/denylist, timeout, cwd, env redaction, audit log를 포함해야 한다"고 정합니다.
- **설계 의도 확인**: argv0만 보는 것이 의도라고 적은 문서는 없습니다. `PROJECT_AUDIT.md`의 CP2는 연쇄 명령, CP3은 shebang을 다루고, 이 문제는 다루지 않습니다.
- **비교**: `tumf/mcp-shell-server`는 같은 인자들(`find -exec`, `git rebase -x` 등)을 기본으로 막습니다(https://github.com/tumf/mcp-shell-server, 2026-11-03 조회).

#### 2.2 doctor 테스트가 실제 HOME을 씁니다

- 규칙(`.issueops/testing/concurrency-and-race.md:48`): "doctor·status처럼 HOME 아래 설정을 읽어 외부를 probe하는 경로를 실행하는 테스트는 HOME을 임시 디렉터리로 둔다."
- `cmd/issueops/basiccli/doctor_composition_test_helpers_test.go:40`은 `os.UserHomeDir()`를 `Doctor.Home`에 넣고, 실제 gateway probe와 lsof를 연결합니다(22행).
- 반박 검토 측정(R2): `-test.run Doctor`를 실제 HOME에서 실행하면 real 11.1~12.1초, user 4.18초, sys 5.0초입니다. 임시 HOME에서는 real 0.52~0.55초, user 0.05초입니다. CPU가 20배 넘게 차이 납니다.
- 테스트 결과가 개발자 머신의 `~/.claude.json`에 묶이는 문제도 함께 생깁니다.

#### 2.3 route-docs 어간 처리 결함

- `internal/domain/projectdoc/records.go:293-306`은 접미사 `ment`, `ing`, `ed`, `s`만 처리합니다. 그래서 `crashes→crashe`, `retries→retrie`, `policies→policie`, `indexes→indexe`가 됩니다(반박 검토 R8).
- 실제 영향: `"stdio processes host"`와 `"stdio process host"`가 서로 다른 기록 목록을 돌려줍니다. "create-pr 직후 프로세스가 죽어도 안전하게"라는 과제에서는 `2026-10-25-recover-a-pr-create-whose-caller-died` ADR이 결과에서 빠졌습니다. `-es`·`-ies` 규칙을 넣으면 2위로 올라옵니다.
- 참고로 route-docs는 단순 단어 겹침이 아닙니다. 용어마다 `ln(N/df)` 가중치를 주고, 겹치는 용어가 2개 이상일 때만 통과시킵니다(`records.go:156-202`). 랜드스케이프 보고서의 F2 정의는 이 가중치를 빠뜨렸습니다.

#### 2.4 `execution prepare` 응답이 `artifact_dir`를 빠뜨립니다

- lease 계약을 issueops 계약으로 바꾸는 변환기가 inbound adapter 세 곳에 복제되어 있습니다.
  - `issueopscompletion/complete.go`와 `issueopslease/release.go`는 본문 41줄이 같습니다.
  - `issueopspreparation/prepare.go:56-61`의 `coreWorkspace`만 `ArtifactDir`를 복사하지 않습니다.
- 반박 검토(R9): 저장되는 record에는 `ApplyDirectCommit`(`internal/application/issueopspreparation/direct_commit.go:25`)이 `ArtifactDir`를 넣습니다. 다음 단계 판정도 record를 읽으므로 동작에는 영향이 없습니다. 그러나 `execution prepare --json` 응답에서 계약 필드가 조용히 빠집니다. 복제가 이미 갈라졌다는 증거입니다.

### 3. 검증 시간과 신뢰성

#### 3.1 CI가 직렬 job 하나입니다

- `.github/workflows/ci.yml`의 `verify` job 하나가 모든 step을 순서대로 실행합니다. lint가 실패하면 race와 self-verify가 돌지 않습니다.
- 이 현상은 이미 기록에 있습니다. `cautions/2026-10-15-linux-exec-text-file-busy.md`에 "lint 실패에 가려 Race test 단계가 한동안 돌지 않아 늦게 드러났다"고 적혀 있습니다.
- CI 실패의 단계별 분포
  - 2026-10-07~10-25 실패 30건 중 lint 15건(50%), self-verify 9건, race 4건, shellcheck 1건, 기타 1건입니다(반박 검토 R4).
  - 다른 표본(실패 15건)에서는 lint가 11건이었습니다.
  - 최근 CI 30회는 모두 성공했습니다.
- CI 안에서 `go test -race`(281초)와 self-verify 안의 비-race `go test`(158초)가 둘 다 돕니다. 깨끗한 checkout에서는 `risk QA tier`가 변경 파일이 없다고 판단해 0ms로 끝나므로, race 결과를 재사용하지 못합니다.

#### 3.2 lint는 문서 규칙만 있고 로컬에서 강제되지 않습니다

- `.issueops/testing/self-verification.md:73`은 golangci-lint를 로컬 battery에 넣으라고 합니다. 그러나 self-verify 27단계에는 golangci-lint, shellcheck, `validate-skill.py`, 문서 구조 검사가 없습니다.
- `.golangci.yml:8`은 lint가 self-verify 경로 밖인 근거로 "AGENTS.md §10"을 가리킵니다. AGENTS.md §10에는 lint라는 단어가 없습니다. 간접적으로 가까운 문장은 "외부 도구가 필요하면 … readiness gate로 요구하지 않는다"입니다.
- 같은 계열의 과거 사고가 있습니다. `cautions/2026-08-26-ci-gofmt-gate-local-battery-drift.md`는 CI에만 있던 gofmt 때문에 main push 8번이 연속으로 실패한 기록입니다.

#### 3.3 Python 단계의 절반 이상을 테스트 하나가 씁니다

- `scripts/meeting_notes_skill_contract_test.py:117`의 `test_synthetic_fixture_family_has_no_identified_meeting_data`가 원인입니다. 이 테스트는 추적 파일 4,438개 전체를 읽고, 지문 길이 11종마다 순수 Python 롤링 해시를 돌립니다.
- 단독 실행하면 user 64.5초, real 69.8초입니다(반박 검토 R5). 같은 환경에서 self-verify Python 단계는 110초였습니다.
- 같은 해시를 Go로 옮긴 실험은 6.4초가 나왔습니다(부하 225, 상수 비교 기준). 지문 값 없이 시간만 잰 실험입니다.
- 이 테스트는 2026-10-18 run 39140575405와 39140579016에서 `.issueops/issues/568/verified-execution-report.md`가 5바이트 지문과 일치해 실패했습니다. 짧은 지문은 오탐을 냅니다.

#### 3.4 Python 환경 실패를 진단하기 어렵습니다

- 2026-11-03 로컬 self-verify는 `Python script tests`에서 실패했습니다. 원인은 `skills/slack-delegate`의 테스트가 `pydantic`을 import하지 못한 것입니다.
- 의존성은 `scripts/python_test_requirements.txt`(`pydantic==2.11.7`)에 선언되어 있습니다. CI는 `uv venv --python 3.13`으로 준비합니다.
- 이 머신의 기본 `python3`는 3.14.8입니다. 3.14에서는 `pydantic==2.11.7`이 `pydantic-core 2.33.2` 소스 빌드에 실패합니다(R10).
- `scripts/python_suite_runner.py:38-39`는 Python 3.10 이상만 확인합니다. 3.10 하한 자체는 문서화된 의도입니다(`.issueops/testing/self-verification.md:139`).
- 실패 결과는 `error: exit status 1`, `failure_cause: unknown`뿐입니다. `ModuleNotFoundError`는 약 34KB stderr 중간에 묻혀 있습니다.
- `.issueops/testing/unit-and-contract.md`는 import 실패를 skip으로 바꾸지 말라고 정합니다. 그래서 개선 방향은 skip이 아니라 진단 문구입니다.

#### 3.5 fail-fast 요약이 미실행을 0점으로 표시합니다

- 위 실행에서는 27단계 중 3단계만 돌았습니다. 실행하지 않은 goal 11개가 `score: 0, passed: false`로 나옵니다.
- `coverage_gaps`와 `failed_step`에 신호는 있습니다. 그러나 goal 항목에는 `not_run` 같은 표지가 없습니다(R11).

#### 3.6 그 밖의 검증 위생

- `project-docs-optimize` checker는 `ok:false`이고 깨진 링크 2건을 보고합니다. 두 파일(`.issueops/issues/561/plan.md:95`, `.issueops/issues/568/verified-execution-report.md:135`)은 TESTING.md 문장을 인용하면서 상대 링크를 함께 옮겼습니다. `.issueops/documentation/README.md`는 historical record의 링크 대상 재작성을 허용합니다. 이 checker는 CI에도 self-verify에도 없습니다.
- 스킬 5개의 본문이 토큰 예산(5000)의 94~98%입니다. `pr-review` 4897, `issueops` 4851, `issueops-plan` 4842, `issueops-implement` 4765, `issueops-create-issue` 4724입니다.
- 문서와 설명이 실제와 4곳에서 어긋납니다.
  - `ci.yml`의 주석과 step 이름은 `--judge none`이라고 쓰지만, 실제 명령은 `--llm-eval=false`입니다.
  - `internal/contract/cli/commands.go:37`의 `model` 설명은 "for claude and codex"인데, 실제로는 호스트 다섯 개를 받습니다.
  - `commands.go:36`의 `mcp` 설명은 stdio proxy만 언급합니다. `--http`, `service`, `authorize`가 빠졌습니다.
  - `.golangci.yml:8`의 AGENTS.md §10 참조가 낡았습니다.
- CI에는 `-shuffle`이 없습니다. 병렬 테스트 관련 caution이 반복되는데(`2026-10-08-parallel-tests-and-non-blocking-flock-leases`, `2026-10-15-linux-exec-text-file-busy` 등), 순서 의존 탐지는 수동 절차뿐입니다.
- `stability-audit` Python 테스트 두 개가 subTest 8개마다 `timeout=1`을 끝까지 기다립니다. 합계 약 8초입니다.

### 4. 런타임 성능

|명령|CPU 중앙값|wall 중앙값(부하 하)|자식 프로세스|
|---|---|---|---|
|`version`|10 ms|28 ms|0|
|`policy check`|10 ms|41 ms|0|
|`list --repo .`|34 ms|127 ms|git 1|
|`project route-docs`|39 ms|220 ms|0|
|`inspect --json`|59 ms|242 ms|git 1|
|`next --json`|61 ms|199 ms|git 4|
|`docs --json`|63 ms|211 ms|git 1|
|`doctor --repo .`|1,154~1,369 ms|1,936~2,832 ms|git 6, lsof 14|

- Go init 합계는 약 1.15ms이고, `-ldflags='-s -w'`를 써도 시작 지연이 바뀌지 않습니다(10.2ms 대 10.6ms). 시작 비용은 최적화 대상이 아닙니다.
- MCP 서비스는 가볍습니다. `tools/list`(도구 51개)는 중앙값 3.8ms, `docs_index`는 41.6ms입니다. 연결 4개에서 초당 68회를 처리했고, 직렬화 잠금은 보이지 않았습니다.
- `doctor`는 `~/.claude.json`의 loopback 게이트웨이 11개를 순서대로 probe하고, 포트 7개마다 lsof를 두 번 부릅니다(`internal/adapter/doctor/observations.go:49-59`, `checks.go:133-153`). 복사본 프로토타입에서는 게이트웨이 병렬화로 wall 중앙값이 1,936ms에서 1,006ms로 줄었습니다. lsof를 두 번으로 묶으면 wall이 1.05초에서 0.12초로 줄었습니다.
- `inspect`·`docs`·MCP `docs_index`는 gitignore된 `.issueops/evidence/`까지 `WalkDir`로 내려갑니다. 이 디렉터리에는 `.issueops` 파일 6,268개 중 5,486개가 있고, 용량은 177MB입니다(`internal/adapter/docs/docs.go:34`). 선택 단계에서 어차피 제외되므로(`internal/domain/docs/selection.go:53-55`) 순회할 이유가 없습니다. 미리 건너뛰면 호출당 약 17ms를 줄입니다.
- `next`는 `rev-parse --git-common-dir`을 두 곳에서 중복 호출합니다(`internal/adapter/issueops/cycle_record_store.go:72`, `internal/adapter/outbound/issueopsinventory/runtime.go:55`).
- `go test`는 sys 시간(4분 26초)이 user(3분 55초)보다 깁니다. 프로세스를 많이 만드는 테스트가 지배합니다. 패키지 시간 상위는 `internal/adapter/issueops` 105초, `cmd/issueops/issueopsapp` 88초(`t.Parallel` 0개), `internal/adapter/provider/gitlab` 56초(`t.Parallel` 0개)입니다.

### 5. 유지보수 비용이 몰리는 곳

- **책임 인벤토리 JSON**
  - `internal/architecture/testdata/ddd_responsibility_inventory.json`은 634KB, 21,381줄입니다. 최근 120일 커밋 2,471건 중 467건(18.9%)이 이 파일을 건드리고, 그중 fix 커밋이 122건입니다.
  - 디렉터리 374개 모두 owner가 하나라서, owner는 경로에서 도출할 수 있습니다. 그러나 (owner, task) 쌍이 여러 개인 디렉터리가 23개 있어 task는 도출할 수 없습니다(R7).
  - ADR `2026-09-30-ddd-responsibility-ownership-across-the-harness`의 배정 규칙은 유지하되 저장 표현을 바꾸는 방향이 가능합니다. 이 경우 보충 ADR이 필요합니다.
- **`next` 라우팅**: `classify.go`는 fix 커밋이 43건이고 `classify_test.go`는 38건입니다. 최근 fix는 "revise 판정 뒤 계획 리뷰로 되돌림", "죽은 holder의 takeover" 같은 상태 조합 누락입니다. 저장소에 `func Fuzz`, `testing/quick`, matrix 테스트가 없습니다.
- **오류 문구 재작업**: fix 제목 813건 중 158건(19%, 키워드 매칭 상한 추정)이 "flag 이름을 말한다", "remedy를 알려 준다" 같은 안내 문구 수정입니다. 10월 24~27일 agent-usability-loop 구간에 몰렸습니다.
- **복잡도 상위**: `gates.LintStatic` 34, `projectdoc.ParseRecord` 27, `issueopsnext.warnings` 25, `syncExecutionBase` 24입니다. 모두 독립 규칙을 한 함수에 나열한 형태입니다. `PROJECT_AUDIT.md`가 노이즈로 확정한 dispatcher 분기와는 다릅니다.
- **복제**
  - 작업 공간 fingerprint 코드: `execution_lease_storage.go:126-191`과 `reseed_inventory.go:175-252`에 약 57줄
  - provider resolver 6개: `internal/adapter/provider/resolve.go`, 이 때문에 커버리지가 21.7%
  - CLI 짝 파일: `remote_assign`과 `remote_label` 등 47~52줄 골격

### 6. 기록 품질(route-docs)

- **정밀도 측정**(ExternalIdeas, 이 저장소의 기록 172건 기준)
  - 현실적인 과제 5개에서 반환 23건 중 관련 기록은 6건(26%)이었습니다.
  - 계획 12건이 인용한 기록 67개를 정답으로 두면 recall@5는 0.19, precision은 0.28입니다.
- **다른 방식과 비교**: BM25F식 가중, 본문 색인, 파일 경로 겹침 변형은 모두 현행을 넘지 못했습니다. 점수 하한을 5로 두면 반환 건수가 17% 줄고 precision이 0.34로 오릅니다(표본 12건).
- **낡은 인용**: 기록이 인용한 파일 경로 중 caution은 24%(127개 중 31개), ADR은 29%(189개 중 55개)가 현재 없습니다. 날짜 기록이라 일부는 의도된 역사 표기입니다. `RouteEffects.Exists`(`internal/application/projectdocs/route.go:25`)가 이미 있어 표시 비용은 작습니다.
- **가장 큰 격차**: route-docs 평가 하네스가 없습니다. 순위를 바꿔도 효과를 잴 수단이 없습니다. OwnMem README의 원칙 "평가 하네스 없이는 순위 변경을 넣지 않는다"가 이 지점에 해당합니다.

### 7. 운영·관측 신호

- **MCP 401**: 서비스 로그의 POST 12,133건 중 1,807건(14.9%)이 401입니다. 10월 6~8일, 15~16일, 11월 2~3일에 몰렸고, 서비스 `ready` 23회 중 15회 뒤에 401 연속 구간이 이어집니다. `cmd/issueops/mcpcli/mcp_http.go:93`은 `status=401 duration_ms=0`만 남겨서, 헤더 누락인지 토큰 불일치인지 구분할 수 없습니다.
- **정체 사이클**
  - 실제 사이클 9건은 모두 `api-servers` 저장소에 있고, 완료는 2건입니다.
  - 3건은 `glab issue create failed: ERROR 404 Not Found`로 3주째 `invoked_unknown` 상태입니다. `next`는 `remote reconcile-issue`를 안내하지만, 레코드의 `failure` 필드만으로는 404 원인을 알 수 없습니다.
  - 4건은 2026-09-18~28 이후 plan 단계에 멈춰 있습니다.
- **호스트 신원 모호성**: 서브에이전트 환경에서는 `PI_SESSION_ID`와 `ISSUEOPS_OMP_SESSION_ID`가 서로 다른 값으로 함께 설정됩니다. 그래서 `next`가 "native session identity is unavailable"을 경고합니다. lease와 claim 신원에 걸린 문제이므로, 고칠 때 보안 검토가 필요합니다.
- **품질 지표의 사각지대**
  - 9월 이후 작업이 main 직접 커밋으로 옮겨 갔습니다. PR merge는 8월 156건에서 9월 18건, 10월 20건으로 줄었습니다.
  - bug 이슈는 8월 88건에서 9월 10건, 10월 8건으로 줄었지만, 같은 기간 main에 쌓인 fix 커밋은 10월에만 316건입니다.
  - 결함이 이슈로 남지 않으므로 GitHub 지표로는 품질 추세를 볼 수 없습니다.
- **자동 후보 목록 고갈**: `self-augment` 후보 30개가 모두 `already_satisfied`이고, `quality inspect`도 후보 0개를 냅니다. 이번 조사에서 찾은 신호(fix 핫스팟, CI 실패 단계, 정책 결함)를 자동 목록이 수집하지 못합니다.

## 후보 목록

우선순위 기준
- P0: 정확성이나 안전 경계가 어긋남
- P1: 측정된 비용이 크고 위험이 낮음
- P2: 효과는 있으나 선행 결정이나 측정이 필요함
- P3: 효과가 작거나 보류

|ID|우선순위|제목|변경 대상|기대 효과|위험|검증|
|---|---|---|---|---|---|---|
|Q1|P0|read-only 정책에 인자 단위 쓰기 판정 추가|`internal/domain/policy/catalog.go`, `command_decision.go`, `adapter_owned_catalog.go:26` 설명|`find -delete`, `sed -i/e/w`, `awk system`, `git branch -D/-m`, `git config`(쓰기형), `git remote add/set-url`, `rg --pre`, `go env -w`, `-toolexec`를 `write_not_allowed`로 판정|`sed -n`, `find -name`, `git config --get` 같은 읽기 형태를 잘못 막을 위험. 인자 차단과 허용 목록에서 제거 중 하나를 고르는 결정 필요|표 테스트, `policy run --read-only -- find . -delete` 거부 확인|
|Q2|P0|doctor 테스트 HOME 격리|`cmd/issueops/basiccli/doctor_composition_test_helpers_test.go:40`, `basic_composition_test.go:23`|CPU 약 9.2초 → 0.4초, 결과가 개발자 머신에 묶이지 않음|낮음(테스트만 바뀜)|`go test ./cmd/issueops/basiccli -count=1`을 실제 HOME과 임시 HOME에서 비교|
|Q3|P0|route-docs 어간 `-es`·`-ies` 처리|`internal/domain/projectdoc/records.go:293` `latinTerm`|복수형 과제에서 빠지던 기록 복구(T2 사례)|기존 `route_test.go` 기대 순서가 바뀔 수 있음|M2 하네스, `go test ./internal/domain/projectdoc`|
|Q4|P0|lease 변환기 단일화와 `ArtifactDir` 누락 수정|`internal/contract/issueopslease/record.go`에 변환 함수, `complete.go`·`release.go`·`prepare.go` 복제 삭제|`execution prepare --json` 응답의 필드 누락 해소, 약 90줄 제거|출력 동등성 테스트 필요|`go test ./internal/adapter/inbound/... ./internal/contract/issueopslease ./internal/architecture`|
|V1|P1|CI를 `lint`·`race`·`self-verify` 병렬 job으로 분리|`.github/workflows/ci.yml`, `scripts/ci_workflow_test.py`|wall 약 544초 → 약 310초 [INFERENCE], 앞 단계 실패가 뒤 단계를 가리지 않음|필수 check 이름 변경, setup 중복|`python3 -m unittest scripts.ci_workflow_test`, `gh run view --json jobs`|
|V2|P1|지문 검사 테스트 가속|`meeting_notes_skill_contract_test.py:117`을 Go로 옮기거나 길이별 상수 비교로 재작성|Python 단계 CPU 약 64초 절감 [INFERENCE: CI 미측정]|지문 값을 정확히 옮겨야 하고, 일치 입력을 넣었을 때 실패하는 부정 대조 테스트 필요. 5바이트 지문 오탐 정책은 별도 결정|신규 테스트, `python3 scripts/python_suite_runner.py`|
|V3|P1|CI와 같은 lint 로컬 스크립트|`scripts/`에 `GOTOOLCHAIN` 고정 golangci-lint v2.12.2와 `GOOS=linux` 변형, `.golangci.yml:8` 참조 정정|CI 실패의 최대 원인(30건 중 15건) 사전 차단|버전이 다르면 거짓 통과. self-verify에 넣는 것은 AGENTS.md §10과 충돌할 수 있어 스크립트 방식 권장|스크립트 exit 코드, 의도적 위반 파일로 실패 확인|
|V4|P1|Python 전제 조건 진단 문구|`scripts/python_suite_runner.py`|`PRECONDITION FAILED: … needs module 'pydantic' … use Python 3.13 venv` 한 줄과 실패 스위트 목록|낮음(skip으로 바꾸지 않고 실패 유지)|`python3 -m unittest scripts.python_suite_runner_test`|
|V5|P1|`stability-audit` 테스트의 timeout 대기 제거|`skills/stability-audit/scripts/e2e_stability_audit_test.py:521-545`|약 8초 절감|timeout 경로 테스트는 유지해야 함|`python3 -m unittest … --durations 6`|
|V6|P1|문서 구조 검사 깨진 링크 2건 수정과 CI 편입|`.issueops/issues/561/plan.md:95`, `568/verified-execution-report.md:135`, CI lint job|문서 위반이 PR에서 드러남, 약 7초 추가|historical record 문구는 그대로 두고 링크 대상만 고쳐야 함|checker `ok:true`|
|R1|P1|`doctor` 게이트웨이 병렬 probe와 lsof 일괄 호출|`internal/adapter/doctor/observations.go`, `checks.go`|wall 약 1.9초 → 약 1.0초, lsof wall 1.05초 → 0.12초|`lsof -p` 다중 pid 파싱 테스트 필요|`go test ./internal/adapter/doctor ./cmd/issueops/basiccli`, 전후 `doctor` 시간 비교|
|R2|P1|문서 순회에서 `.issueops/evidence` 건너뛰기|`internal/adapter/docs/docs.go:34`, 도메인의 제외 판정 공유|호출당 약 17ms, evidence가 늘어날수록 커짐|낮음(선택 결과는 같음)|evidence 아래 `.md` fixture 테스트, `docs --json` 전후 비교|
|M1|P2|route-docs 평가 하네스|`internal/domain/projectdoc/`에 정답 세트와 recall·precision 테스트|이후 순위 변경(Q3, 점수 하한, 낡은 인용 표시)을 측정으로 판정|정답이 이 저장소 기록에 묶임|`go test ./internal/domain/projectdoc -run Route`|
|M2|P2|`next` 상태 공간 matrix 테스트|`internal/domain/issueopsnext`|에이전트 QA가 사후에 찾던 데드엔드를 CI에서 먼저 잡음 [INFERENCE]|불가능 조합 표 유지 비용|이후 60일 `fix(next)` 건수 비교|
|M3|P2|책임 인벤토리 저장 표현 축소|`internal/architecture/ddd_responsibility_test.go`, 인벤토리 JSON|owner를 경로에서 도출하고 task 예외만 저장해 파일 크기 약 70% 감소 [INFERENCE]|보충 ADR 필요. task는 도출 불가|`go test ./internal/architecture/...`, 이후 건드림 비율 비교|
|M4|P2|규칙 나열형 함수 분해|`gates.LintStatic`, `syncExecutionBase`, `issueopsnext.warnings`, `classify.go` 분할|복잡도 34/24/25 → 약 8/12/6 [INFERENCE]|오류 문구와 경고 순서 보존. `classify.go` 분할은 인벤토리 재생성 필요|해당 패키지 테스트와 골든|
|M5|P2|복제 정리|provider resolver 제네릭화, fingerprint 코드 단일화|약 110줄 제거, provider 커버리지 21.7% → 80% 이상 [INFERENCE]|fingerprint 두 구현의 값 동등성 테스트 먼저 필요|`go test -cover ./internal/adapter/provider` 등|
|M6|P2|오류 remedy 존재 검사|CLI 거부 코드 생성 지점에 대한 표 테스트|문구 재작업(fix 19%) 감소 [INFERENCE]|범위를 정하지 않으면 거짓 양성|`go test ./cmd/issueops/... -run RemedyPresent`|
|V7|P2|self-verify 요약에 `not_run` 표시|`internal/application/selfverify/self_verify_summary*.go`, `internal/contract/selfverify`|실패 단계와 미실행 단계 구분|JSON 계약 버전과 해시, golden 갱신|`go test ./internal/application/selfverify ./internal/contract/...`, `contract check`|
|V8|P2|문서·설명 어긋남 정리|`ci.yml`의 `--judge none`, `commands.go:36-37`, `response_contracts.golden.json`|설명과 실제 동작 일치|golden 해시 변경|`go test ./cmd/issueops/... -run Golden`|
|O1|P2|MCP 401 진단 필드|`cmd/issueops/mcpcli/mcp_http.go:93`|`auth=missing\|mismatch`와 클라이언트 종류로 401 원인 특정|토큰 값은 남기지 않아야 함|`go test ./cmd/issueops/mcpcli`|
|O2|P2|정체 사이클과 provider 오류 원인 보존|`list --json`의 `idle_days`, `invoked_unknown` 레코드에 `glab` 하위 명령·HTTP 상태|방치된 사이클과 404 원인이 드러남|redaction 필요, golden 갱신|provider fake로 404 주입 테스트|
|O3|P2|자동 후보 목록에 핫스팟·CI 신호 추가|`internal/adapter/outbound/quality`|`quality inspect`가 fix 핫스팟과 CI 실패 단계를 후보로 냄|`git log` 비용, 이름 변경 처리|`quality inspect --json` 새 신호 확인|
|R3|P3|`next`의 git 호출 중복 제거|`cycle_record_store.go:72`, `issueopsinventory/runtime.go:55`, `issueops_next_wiring.go:191,197`|git 4회 → 2회, CPU 약 30ms [INFERENCE]|요청 안에서 다시 관측해도 안전한지 먼저 확인|PATH shim으로 호출 수 재측정|
|V9|P3|self-verify 독립 단계 병렬 실행|`internal/application/selfverify/steps.go`, `loop.go`|V2 없이 약 263초 → 약 160초 [INFERENCE]|문서의 "risk QA 이전 중단" 계약과 충돌, 부하 시 CPU 경합|깨끗한 머신에서 3회 `elapsed_ms` 비교|
|V10|P3|CI `-shuffle=on`과 runner 이미지 고정|`ci.yml`|순서 의존 결함 조기 발견|초기 불안정, 최신 OS 신호 손실|로컬 `go test -race -shuffle=on -count=3`|
|S1|P3|토큰 예산 근접 스킬 본문 분리|`pr-review`, `issueops`, `issueops-plan`, `issueops-implement`, `issueops-create-issue`|다음 편집에서 검증 실패 방지|분리 후 참조 누락|`python3 scripts/validate-skill.py skills/<name>`|
|I1|P3|호스트 신원 변수 충돌 처리|`executioncmd`의 호스트 신원 판별|claim 실패 재시도 감소 [INFERENCE]|보안 경계. 우선순위를 잘못 정하면 다른 호스트의 lease를 가져갈 수 있음|`go test ./cmd/issueops/issueopscli/executioncmd -run Whoami`|

**채택하지 않는 것**
- `-ldflags='-s -w'`와 init 최적화: 시작 지연이 바뀌지 않습니다.
- route-docs 순위를 BM25F나 n-gram으로 교체: 이 저장소의 정답 세트에서 현행보다 낫지 않았습니다.
- OwnMem식 기록 quota: ADR이 append-only 이력 원칙을 따릅니다.
- sandbox-runtime 내장: AGENTS.md §10이 외부 도구 기능을 core에 복제하지 않는다고 정합니다. 사용자가 설치한 도구를 감싸는 선택 기능으로만 검토할 수 있습니다.
- 파서 fuzz: `ParseRecord` 계열을 20초 동안 159,704회 실행해 크래시 0건이었습니다. 정책·명령 파서는 시험하지 않았습니다.
- `go test` 긴 꼬리 패키지의 `t.Parallel` 추가: 8코어에서는 CPU 총량이 wall을 정합니다. flock lease 경로의 병렬화는 `cautions/2026-10-08-parallel-tests-and-non-blocking-flock-leases.md`가 금지합니다. Q2처럼 CPU 총량을 줄이는 쪽이 먼저입니다.

## Cross-Check Results

|상태|개수|뜻|
|---|---|---|
|재현 후 성립|5|R1 정책 결함, R2 HOME 격리, R3 evidence 순회, R5 지문 테스트, R8 어간 결함|
|재현 후 보정|6|R4 lint 비중(73% → 50%), R6 깨진 링크(심각도 낮음), R7 인벤토리(owner와 task 구분), R9 `ArtifactDir`(응답만 누락), R10 Python 버전(3.10 하한은 의도), R11 not_run(다른 신호는 있음)|
|반박됨|0|없음|
|단일 측정|나머지 성능 수치|부하 하 측정. 절대값이 아니라 상대 비교로 읽어야 함|

## Adversarial Review

- 조사에 참여하지 않은 에이전트 한 명이 R1~R11을 다시 재현했습니다.
- **R1**: `awk system()`, `git branch -D`, `find -exec`도 같은 경로로 허용된다는 점을 추가로 확인했습니다. 인자 차단과 허용 목록에서 제거 중 무엇을 택할지는 코드에서 도출되지 않는다고 판단했습니다.
- **R4**: "CI 실패의 73%가 lint"는 표본 15건 기준이었습니다. 실패 30건 기준으로는 50%입니다. 최근 30회는 모두 성공해서, 문제가 지금도 계속되는지는 불명확합니다.
- **R5**: 지문 테스트의 CPU는 단독 실행 기준 user 64.5초입니다. 처음 보고한 약 90초는 부하 하 측정이었습니다.
- **R9**: 저장 상태는 정상입니다. 동작 결함이 아니라 응답 계약 불일치로 등급을 낮췄습니다.
- **R10**: Python 3.10 하한은 문서화된 의도입니다. 문제는 핀된 `pydantic==2.11.7`이 3.14에서 빌드되지 않는 점과, 실패 진단이 이를 알려 주지 않는 점입니다.

## Disputed / Unresolved

- **Q1의 방식**: 인자 규칙을 추가할지, `find`·`sed`·`awk`를 read-only 허용 목록에서 뺄지 정해야 합니다. 앞의 방식은 사용성이 좋고 규칙 유지 비용이 있습니다. 뒤의 방식은 단순하지만, 읽기 용도로 쓰던 호출자가 깨질 수 있습니다.
- **CI 구조 변경**: 필수 check 이름과 브랜치 보호 규칙이 어떻게 연결되어 있는지 확인하지 않았습니다.
- **lint 문제의 지속 여부**: 최근 30회 CI는 모두 성공했습니다. V3의 긴급도는 다음 실패 추이를 보고 판단해야 합니다.
- **사이클 지표 보존 0건**: ADR 2026-10-23의 사이클 결과 보존 코드는 있습니다. 그러나 이 머신의 state에는 보존 행이 0건입니다. 정리를 마친 사이클이 없어서인지 쓰기 경로 결함인지 구분하지 못했습니다.

## Open Questions

- self-verify `risk QA tier`의 race 단계 시간은 측정하지 못했습니다. V9의 기대 이득이 이 값에 달려 있습니다.
- `measure_efficiency_test` 8건이 느린 이유와, Python 스위트가 Go 산출물에 의존하는지는 끝까지 읽지 않았습니다.
- `glab` 404의 근본 원인(프로젝트 경로, 권한, 엔드포인트)은 레코드만으로 알 수 없습니다.
- MCP workspace 도구(`harness_inspect`, `project_docs_route`)의 HTTP 지연은 `mcp authorize`가 state를 쓰기 때문에 측정하지 않았습니다.
- 병합 충돌 빈도는 측정하지 못했습니다. M3의 효과를 판단하려면 이 값이 필요합니다.

## Source Index

조회일은 모두 2026-11-03입니다.

|#|출처|설명|유형|권위|
|---|---|---|---|---|
|1|`issueops quality inspect --repo . --json`|기준선 신호|실행 출력|High|
|2|`issueops self-augment --json`|후보 목록 30개|실행 출력|High|
|3|`internal/domain/policy/catalog.go:50,97-108`, `command_decision.go:109-115`|read-only 판정|소스|High|
|4|`internal/contract/mcp/adapter_owned_catalog.go:26`|`worker_run_read_only` 설명|소스|High|
|5|`.issueops/testing/concurrency-and-race.md:48`|HOME 격리 규칙|저장소 문서|High|
|6|`cmd/issueops/basiccli/doctor_composition_test_helpers_test.go:22,40`|실제 HOME 사용|소스|High|
|7|`internal/domain/projectdoc/records.go:156-202,293-306`|route-docs 점수와 어간|소스|High|
|8|`internal/adapter/inbound/issueopspreparation/prepare.go:56-61`, `issueopscompletion/complete.go`, `issueopslease/release.go`|변환기 복제|소스|High|
|9|`.github/workflows/ci.yml`, `.golangci.yml:8`|CI 구조와 lint 설정|소스|High|
|10|GitHub Actions run 200건, run 41568386193, 39140575405, 39140579016|CI 시간과 실패 단계|CI 기록|High|
|11|`scripts/meeting_notes_skill_contract_test.py:117`|지문 테스트|소스|High|
|12|`scripts/python_suite_runner.py:38-39`, `scripts/python_test_requirements.txt`|Python 실행기와 의존성|소스|High|
|13|`.issueops/testing/self-verification.md:73,139,151`, `.issueops/testing/unit-and-contract.md`|검증 규칙|저장소 문서|High|
|14|`internal/adapter/doctor/observations.go:49-59`, `checks.go:133-153`|doctor probe|소스|High|
|15|`internal/adapter/docs/docs.go:34`, `internal/domain/docs/selection.go:53-55`|문서 순회|소스|High|
|16|`internal/architecture/testdata/ddd_responsibility_inventory.json`, `ddd_responsibility_test.go:62-83`|책임 인벤토리|소스|High|
|17|`git log --since=120.days`|커밋 유형, churn, fix 핫스팟|git 이력|High|
|18|`gh issue list`, `gh pr list`|이슈·PR 추세|GitHub 데이터|High|
|19|`~/.local/state/issueops/mcp-http/server.log`|MCP 401 분포|로컬 로그|Medium|
|20|`issueops list --json`, `review-metrics --repo …/api-servers --json`|실제 사이클|실행 출력|Medium|
|21|`.issueops/PROJECT_AUDIT.md`|선행 감사 결과(CP2, CP3, `branch_candidate_functions`)|저장소 문서|High|
|22|`.issueops/adr/2026-09-30-ddd-responsibility-ownership-across-the-harness.md`|인벤토리 규칙|ADR|High|
|23|`.issueops/cautions/2026-08-26-ci-gofmt-gate-local-battery-drift.md`, `2026-10-15-linux-exec-text-file-busy.md`, `2026-10-08-parallel-tests-and-non-blocking-flock-leases.md`|CI와 테스트 불안정 기록|caution|High|
|24|https://github.com/tumf/mcp-shell-server|인자 단위 명령 차단|README|Medium|
|25|https://github.com/grpcer/ownmem|평가 하네스 원칙, 조회 이유 출력|README|Medium|
|26|https://github.com/kage-core/Kage|낡은 기록 제외 방식|README|Medium|
|27|https://github.com/oliver-zehentleitner/keep-the-why|기각 사유 기록 실험(20세션)|README|Medium|
|28|https://github.com/anthropics/sandbox-runtime|OS 수준 격리|README|Medium|
|29|https://raw.githubusercontent.com/github/gh-aw/main/docs/src/content/docs/reference/work-queue.md|불확실한 dispatch 재시도 금지|문서|Medium|
|30|`.issueops/research/2026-11-03-similar-open-source-landscape.md`|비교 대상 목록|저장소 연구|Medium|
