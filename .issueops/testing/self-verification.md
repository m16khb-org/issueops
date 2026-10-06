# Single-pass self-verification contract

[← TESTING.md](../TESTING.md) owns the test-strategy index and minimum
completion gate. This document defines the document-stage verification battery,
self-verify QA gate, all-or-nothing single-run contract, standalone verification
policy, and web-fetch live parity.

## 문서 단계 검증

문서만 변경해도 문서 링크·구조와 관련 지침의 일치 여부를 확인하고 아래
[최종 검증 battery](#최종-검증-battery)의 단일 self-verify 결과를 남긴다.
설치·bootstrap apply·state 쓰기는 문서-only 최소 완료 기준에 추가하지
않는다. self-verify가 수행하는 기존 내부 smoke와 native integration은 유지한다.
실행용 binary가 없거나 stale이면 먼저 빌드하며, 이 준비 build는 self-verify의
검증용 임시 build와 구분한다.

## 선택적 운영 명령 예시

아래 목록은 전체를 순서대로 실행하는 필수 battery가 아니다. 관련 기능을 변경할
때 필요한 명령만 선택한다. 설치·bootstrap apply·state 저장·promote·
self-augment는 해당 작업의 승인 범위와 격리된 HOME/state에 따라 실행한다.
문서 변경만을 이유로 사용자 홈 설치 상태를 바꾸지 않는다.

```bash
find . -maxdepth 3 -type f | sort
find .issueops -maxdepth 1 -type f -name '*.md' | sort
grep -R "외부 Go 하네스\|Go\|MCP\|Codex\|Claude" -n AGENTS.md CLAUDE.md .issueops
python3 scripts/validate-skill.py skills/atomic-commit-push
go test ./... -count=1
go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
go build -o bin/issueops ./cmd/issueops
./scripts/install-native.sh
./bin/issueops bootstrap --dry-run
./bin/issueops install --json
./bin/issueops install --dry-run --json
./bin/issueops inspect --json
./bin/issueops docs --json
./bin/issueops guard check --staged --json
printf '{"cwd":"%s","source":"compact"}' "$PWD" | ./bin/issueops hook session-start --host claude
./bin/issueops policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/issueops policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
tmp_state="$(mktemp -d)"
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops doctor --repo . --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state write --key smoke --value "ok" --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state read --key smoke --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state list --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state prune --max-age 720h --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state doctor --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state maintain --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --save-state --state-key self-verify-smoke --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify history --prefix self-verify --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify history --prefix self-verify --retention-limit 1 --prune-retention --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify compare --baseline-key self-verify-smoke --candidate-key self-verify-smoke --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify promote --from-key self-verify-smoke --baseline-key self-verify-baseline --json
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
./bin/issueops self-augment --cycles=1 --target-score=95 --json
./bin/issueops self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/issueops self-augment lesson --candidate reflexion-state-memory --lesson "test lesson" --next-action "test next action" --state-key self-augment-lesson-test --json
./bin/issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
grep -R "Conventional Commit\|Lore:" -n AGENTS.md .issueops/COMMIT_POLICY.md skills/atomic-commit-push/SKILL.md
```

## 최종 검증 battery

완료 보고의 최종 검증 battery는 같은 revision, 같은 환경, 같은 입력 상태에서 나온 하나의 evidence bundle로 기록한다. 범위는 검증 대상 base-to-head diff와 보존된 작업 범위이며, clean working tree에 이미 커밋된 Go 변경도 포함한다. 위험 tier 이름이나 working-tree 상태만으로 필요한 Go 정적·race 검증의 성공을 추정하지 않는다.

기본 완료 battery의 소유 관계는 다음과 같다.

- `./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json`이 self-verify가 실제로 수행한 test/build/golden/docs/inspect 증거를 소유한다. 현재 step 목록은 `gofmt`, `Python script tests`, `go test ./... -count=1`, contract golden의 full-test 포함 관계, `go build -o <temp>/issueops ./cmd/issueops`, `doctor --static-only --json`, `inspect smoke`, `docs index smoke`를 포함한다.
- 최종 battery에서 self-verify JSON이 같은 revision·환경·입력에서 통과했고 그 step 결과가 완전하면 전체 `go test ./... -count=1`을 별도 책임으로 다시 실행하지 않는다. `go build -o bin/issueops ./cmd/issueops`, `./bin/issueops docs --json`, `./bin/issueops inspect --json`과 전체 Go 테스트에 포함된 contract golden도 별도로 반복하지 않는다.
- `go vet ./...`와 `go test -race ./... -count=1`은 Go 변경 기본 검증의 별도 구성원이다. self-verify의 `risk QA tier`가 그 명령을 실제 실행한 step을 같은 bundle에 담았을 때만 포함 관계로 인정한다. 실제 step 증거에 필요한 명령이 없으면 누락된 명령을 같은 battery에서 별도 실행한다.
- 최종 battery 명령은 `gofmt -l $(git ls-files '*.go')`, `go vet ./...`, `go test -race ./... -count=1`, `./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json` 중 이번 변경 범위가 요구하는 모든 항목을 포함한다. self-verify가 소유한 test/build/docs/inspect 증거를 재사용할 때도 vet/race 결과는 `unit-and-contract.md`의 Go 변경 기준을 따른다.

IssueOps benchmark fixtures must stay repo-agnostic. They score portable workflow evidence rather than one target repository's domain facts: domain invariants vs exact/equivalent mechanisms, API-doc gate evidence, live runtime evidence matrices, review-feedback accountability, and completion hygiene. A deterministic benchmark passes only when `average_score == 100`, `minimum_score == 100`, and `critical_failure_count == 0`.

관련 변경의 추가 확인:

아래 native integration 명령은 host 연결을 변경했을 때 확인하는 운영 예시다.
문서-only 완료를 위해 별도 설치나 중복 smoke를 요구하지 않는다.

Native integration smoke:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md
test -f ~/.claude/skills/atomic-commit-push/SKILL.md
codex mcp get issueops
claude mcp list | grep issueops
```

- `AGENTS.md`와 `CLAUDE.md`가 같은 source of truth를 가리키는가
- `.issueops/`의 링크가 실제 파일과 맞는가
- plugin vs worker 결정과 Go 선택이 여러 문서에서 충돌하지 않는가
- shared skill 원본(`skills/*`)과 user-level host 연결(`~/.codex/skills/*`, `~/.claude/skills/*`)이 drift 없이 같은 대상을 가리키는가
- 커밋 정책이 `AGENTS.md`, `.issueops/COMMIT_POLICY.md`, `atomic-commit-push` skill에서 충돌하지 않는가

## CI 검사 소유권

CI의 clean checkout에서는 다음 소유 관계로 각 검사를 한 번 실행한다.

| 검사 | 실행 소유자 | 환경·횟수 |
| --- | --- | --- |
| Python discovery | self-verify의 `Python script tests` | 준비한 Python 환경·임시 HOME, root 1회와 스킬별 1회 |
| Go test match self-test | self-verify의 `Go test match guard` | Python 다음, 1회 |
| 일반 Go 전체 테스트 | self-verify의 `go test` | 임시 HOME, 1회 |
| Contract golden | 성공한 전체 Go 테스트에 포함 | 별도 중복 실행 없음 |
| Go race | CI의 독립 `Race test all packages` 단계 | runner 기본 HOME, 1회 |
| Native integration | 설치 후 self-verify의 `native integration` | 같은 임시 HOME/CODEX_HOME, 1회 |
| Build | CI의 `Build`와 self-verify의 `go build` | 설치용 binary와 검증용 temp binary를 각각 생성 |

임시 HOME 설치와 self-verify는 한 CI 블록에서 실행하며, 설치 실패와 Python·Go
검사 실패는 블록의 비영 종료 코드로 CI에 전파한다. EXIT trap은 성공·실패 모두에서
임시 HOME을 정리한다. race 실패도 독립 단계의 비영 종료 코드로 전파한다.

runner 기본 HOME과 임시 HOME의 결과를 서로 재사용하지 않는다. CI 밖에서 실행하는
standalone self-verify도 Python·Go·golden·native integration 검사를 그대로 수행하며,
별도 skip 옵션이나 이전 SHA·dirty 상태·환경의 결과를 가져오는 경로는 없다. dirty
Go 변경에 따른 risk QA의 성공 race 재사용은 기존 계약대로 같은 self-verify run
안에서만 적용한다. CI 명령은 `--llm-eval=false`를 명시한다.

## 자기 검증 QA gate

`issueops self-verify`에는 테스트와 QA gate가 포함된다. QA gate는 루프 문서, `GENIUS_THINK.md`, shared skill metadata, native integration 설치 상태, redaction audit, bounded stdout/stderr metadata, Mermaid 문서 lint를 확인하고, 모든 목표 점수가 95점을 초과해야 종료할 수 있다. Mermaid lint는 `GENIUS_THINK.md`의 따옴표/`<br/>` 규칙을 기준으로 문서 다이어그램의 파싱 오류를 조기에 방지한다.


## 저장소 Python 검사

`Python script tests`는 gofmt 다음, risk QA와 전체 Go 검사 전에 실행한다. PATH의
`python3`로 버전을 확인한 뒤 같은 `sys.executable`로 CI와 동일한 suite runner를
실행한다:

```bash
python3 scripts/python_suite_runner.py
```

최소 runtime은 Python 3.10이다. publish helper의 union annotation은 Python 3.9에서
import 오류를 낸다. 버전은 stdout에 표시하고, 3.10 미만이면 stderr 진단과 함께 discovery
전에 실패한다. 실행 파일이 없으면 기존 command runner의 executable-not-found 오류로
단계가 실패한다. timeout은 버전 확인과 discovery를 합쳐 5분이며 출력 budget을 유지한다.
runner는 root suite를 한 번 실행하고 스킬마다 CWD/import가 격리된 process를 사용한다.
파일별 수집 수와 전체 suite 수를 출력한다. 기존 optional root skip 의미는 보존하지만
스킬의 import 실패·빈 테스트 파일·실패·skip으로 통과시키지 않는다.
기존 Slack 의존성을 포함한 환경 준비는
[Python 검사 환경](unit-and-contract.md#python-검사-환경)을 따른다.

그다음 `Go test match guard`가 `bash scripts/verify-go-test-match-test.sh`를
30초 한도로 실행한다. 이 guard는 실제 Go/race 검사의 대체 증거가 아니며,
누락·실패하면 기본 fail-fast 모드에서는 risk QA 이전에 중단한다.
재실행 명령도 같은 script를 가리킨다.

단계 실패는 전체 OK와 termination을 거부한다. `test_suite` 점수와 `test suite contract`
coverage는 Python과 Go test-match guard 증거를 모두 요구한다.
기본 step 목록과 summary contract의 version/hash는 native CLI 출력이 기준이다.
현재 계약 정의는 `internal/domain/selfverify/contract.go`의 `ContractValue`가 소유하며, 기존 JSON 필드와
snapshot schema v1은 유지한다. labels 자체는 hash 입력이 아니므로 version 변경으로
검사 범위를 구분한다. 역사 요약은 history/compare로 읽을 수 있고 새 단계는 added/missing
label로 비교한다. 후보 계획의 verification QA는 현재 contract version/hash와
termination 판정이 일치하는 성공 요약만 완료 근거로 인정한다.

## Golden 및 binary drift 결과 판정

성공한 full-suite는 golden 증거로 재사용한다. fallback은
`cmd/issueops/contractgolden`과 `cmd/issueops/issueopsapp`에서
TestCLIUsageGolden, TestMCPToolsGolden, TestMCPResourcesGolden,
TestResponseContractsGolden만 `go test -json`으로 실행한다. 네 테스트 각각의
run/pass와 두 package의 pass가 모두 있어야 통과한다. zero-match, skip, fail,
잘못되거나 잘린 JSON은 성공 증거가 아니다.

Binary drift 단계는 temp binary로 doctor를 실행하되 관측 대상은
`root/bin/issueops`다. 정확히 한 `binary_drift` check의 명시적인 healthy bool을
확인한다. stale은 실패하고, fresh와 미빌드 skip은 통과한다. 다른 doctor 경고는
이 단계의 실패로 바꾸지 않는다. 누락·중복 check, bool 누락, 잘못되거나 잘린 JSON은
실패한다. doctor의 JSON exit code와 generic command runner의 계약은 유지한다.

## 부분 검증 상태 금지 (all-or-nothing verification)

다단계 검증 시나리오에서 한 단계라도 실패하면 이전 통과를 재사용하지 말고 1단계부터 전체를 다시 실행한다.

완료 보고의 evidence는 마지막으로 "전 단계 통과"한 단일 run에서만 가져온다. 서로 다른 run의 부분 통과를 조합하지 않는다.

실패, 취소, revision 또는 환경 drift, prompt-only LLM 평가, incomplete result는 완료 evidence가 아니다. 실패 뒤 다음 시도는 최종 검증 battery 첫 항목부터 다시 시작하며, 실패 전 step의 부분 통과를 새 bundle에 섞지 않는다.

재실행 비용이 커도 부분 통과를 "검증됨"으로 기록하지 않는다. 비용이 문제면 시나리오를 더 작은 독립 시나리오로 나눈다.

## Standalone Verification Policy

Agent-harness tests must verify harness core and native integration contracts without requiring external toolchains, external accounts, or companion MCP servers. External companion tools are not prerequisites for install/update/self-verify readiness.

Standard deterministic self-verify commands pin `--llm-eval=false`, so ambient `ISSUEOPS_SELF_VERIFY_LLM_EVAL=gate` cannot turn the project gate into the prompt-only diagnostic path. `advisory` or `gate` currently render a read-only evaluator prompt without sending a Z.AI request; without an ingested external verdict, `gate` is expected to remain non-passing. After an interrupted or prompt-only attempt, record the explicit override and rerun the verification sequence from the first gate.

Keep fixtures for external-tool data as plain local input and verify only the harness boundary that consumes it. Do not add tests that clone, install, patch, or register external tools during normal verification.

## Web-Fetch Live Parity

The default web-fetch battery is deterministic and must not require network access. Opt-in live parity uses `ISSUEOPS_WEBFETCH_LIVE=1` with `testdata/webfetch/live/public-fixtures.json`; follow `.issueops/operations/web-fetch-live-parity.md` before interpreting live results or comparing them with a generic baseline executable.
