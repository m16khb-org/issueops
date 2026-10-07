# #548 코드베이스 점검 결과 정비: verified-execution report

- lifecycle: `io-7426b49dc042`, direct execution generation 2
- issue: https://github.com/m16khb-org/issueops/issues/548
- branch: `548-codebase-maintenance-sweep`, base `main` @ `93649a264fd6f972dbb42989113914afc9444f0b`
- 계획: `.issueops/issues/548/plan.md`, 게이트 원장: `.issueops/issues/548/gates.md`
- 상태: 구현 완료 초안(4단계). 5단계 정리에서 확정한다.

## 수용 기준별 결과

| 기준 | 결과 | 근거 |
|---|---|---|
| go1.26.6 이상, 호출되는 취약점 0건 | 충족 | `go.mod` `go 1.26.6`; govulncheck "No vulnerabilities found" (G5) |
| race 전체 통과, MCP 동시성 테스트가 panic 없이 에러 보고, readability 테스트가 wall-clock에 묶이지 않음 | 충족 | G4, G8 |
| quality inspect `collection_status`가 error가 아니고 실패 패키지가 evidence에 남음 | 충족 | G9, `TestInspectNamesFailedCoveragePackages` |
| 문서 drift 정리, `--help` 완전성, PROJECT_AUDIT 재대조 | 충족 | G11, G12, PROJECT_AUDIT 128줄·파서 0건 0경고 |
| 직접 의존성 최신, tidy, CI shellcheck·최신 action | 충족 | G6, G10 |
| 중복 헬퍼 통합, ctx 전파, 테스트 격리 | 충족 | D1·D2·D3·D5 아래 |
| gofmt·vet·golangci-lint·architecture·golden·self-verify | 충족 | G1~G3, G7, G13 |

## 묶음별 변경과 측정

### A. 보안과 검증 신뢰성
- A1: `go.mod` `go 1.26.3` → `1.26.6`, README 두 곳의 Go 버전 표기.
- A2: `mcp_concurrency_test.go`. 원인 재현: GOMAXPROCS=1 테스트 바이너리 12개를 동시에 실행하면 12/12가 `:115` nil 역참조 panic(세션별 고정 10초 컨텍스트 만료 → `ListTools` 에러 → nil 결과 사용). 수정: `listErr`를 먼저 확인하고, 세션별 고정 타이머 대신 테스트 바이너리 deadline에서 30초 여유를 둔 컨텍스트를 쓴다. 같은 조건 12/12 통과.
- A3: `TestCheckLargeBodyIsFast`(200ms 단언) → `TestCheckLargeBodyScalesLinearly`. 본문 크기 배율 대비 Check 할당 바이트 배율을 비교한다(실측 7.35x/7.96x, 배율 4에서 상한 1.5배). 부하와 무관하다.
- A4: `go test -cover ./...` 실패 원인은 `internal/adapter/policy` helper 프로세스였다. 커버리지 빌드 helper가 `GOCOVERDIR` 없이 종료하며 stderr에 경고를 남기고, executor가 환경을 allowlist로 걸러 stderr 정확 비교가 깨졌다. 커버리지 빌드일 때만 helper 요청에 임시 `GOCOVERDIR`을 넘긴다(RED: `go test -cover` 실패 → GREEN). `quality inspect` warning에 `go test`의 `FAIL\t<pkg>` 패키지를 최대 10개 붙인다(`quality.FailedTestPackages`, `coverageWarning`).

### C. 의존성·CI·저장소 위생
- C1: x/sync 0.23.0, x/term 0.46.0, x/sys 0.48.0, goja 2026-10-06, sqlite v1.60.1, x/text v0.42.0(goja가 요구하던 v0.3.8을 명시 require로 상향). `go mod tidy -diff` 빈 출력, `go mod verify` 통과. sqlite 변경 기록에서 기본 동작 변화는 DSN 파라미터 사전 검증뿐이고 현재 DSN(`_pragma`, `_txlock`, `mode=ro`)은 유효하다. sqlstore race 테스트 통과. 바이너리 크기 31,915,810 → 31,953,298 바이트.
- C2: checkout v7, setup-go v7, golangci-lint-action v9, setup-uv v10.2.0(v8부터 major 태그 미발행이라 정확한 태그로 고정). 입력 이름 변화 없음, 공통 breaking은 node24 런타임.
- C3: CI에 `shellcheck install.sh scripts/*.sh` 단계. 기존 지적 3건 수정(SC2181은 `if ! emit_receipt`, SC2329는 trap으로 호출되는 함수라 disable 주석).
- C4: `.gitignore`의 bare `evidence`(어느 위치의 `evidence` 파일·디렉터리든 숨김) → 런타임 증거 루트 네 곳(`/evidence/`, `.issueops/evidence/`, verified-execution 스킬의 `.issueops/verified-execution/evidence/`와 `/.verified-execution/evidence/`)만 무시한다. 저장소의 모든 디렉터리 아래 가상 `evidence/x.txt` 705개를 base 규칙과 새 규칙으로 `git check-ignore --no-index` 비교하면 base는 705개를 숨기고 새 규칙은 위 루트만 숨긴다. 코드가 증거를 쓰는 경로(conformance `--evidence-dir` 기본값, child-host smoke)는 모두 `.issueops/evidence/` 아래다. 구현 리뷰 1차가 verified-execution 증거 루트 누락을 지적해 추가했다(처음 측정한 "ignored 집합 동일"은 해당 디렉터리가 디스크에 없어 드러나지 않았다). 첫 게이트 실행에서 `internal/holdoutdeleak`의 `TestEvidenceAnswerTreeStaysUntracked`가 실패했다(A4의 실패 패키지 진단이 바로 지목). 이 테스트는 `evidence`라는 줄의 글자를 대리 조건으로 보고 있었고, 지키는 불변식인 holdout 정답 트리(`.issueops/evidence/`) ignore는 새 규칙에서도 유지된다. 테스트를 `git check-ignore --no-index`(전역·시스템 설정 배제)로 실제 ignore 여부를 보도록 바꿨고, `.issueops/evidence/` 줄을 지우면 실패하는 것을 확인했다. `testdata/pioneer-holdouts/README.md`의 규칙 서술도 맞췄다.

### D. 코드 품질과 테스트 격리
- D1: `cmd/issueops/jsonout`(`Print`, `PrintTo`). 16개 패키지의 사본은 `var printJSON = jsonout.Print`(webfetchcli는 `PrintTo`)로 바꿔 호출 지점을 그대로 두었다. issueopsapp의 `printJSONTo`는 `printJSON`이 더 이상 호출하지 않아 테스트 한 곳만 남았으므로 지웠다(같은 동작은 `jsonout_test.go`가 검사).
- D2: 같은 루프의 `containsString`/`contains` 10개 → `slices.Contains`(호출 37곳). `firstNonEmpty` 4개는 모두 공백 trim을 해서 `cmp.Or`와 동작이 달라 유지.
- D3: glab 가드(덮어쓸 파일이 fixture 가짜인지 확인), projectcli `TestMain`의 임시 `ISSUEOPS_STATE_DIR`, conformance 테스트 `t.Setenv`, channel store 테스트의 죽은 줄 2개 제거, `t.Chdir` 2곳, `testsupport.IsolateGitConfig`(임시 전역 설정에 테스트 identity, `GIT_CONFIG_NOSYSTEM=1`)를 저장소 생성 헬퍼 13개에서 호출.
- D4: 단독 race 실행 기준 `TestCLIHelpers` 11.46s → 3.89s, `TestRunStatusWritesTextAndJSON` 5.10s → 0.24s. 원인은 doctor가 실제 HOME의 `~/.claude.json` loopback MCP gateway를 HTTP probe하고 lsof를 돌리는 것(호출당 약 2.6s)이었다. 테스트 HOME을 임시 디렉터리로 둔다. `TestHandleSelfLoopMCPToolCallCoversLocalPayloads`는 단독 1.67s라 전체 실행의 41s는 병렬 CPU 경쟁이다. 변경하지 않음. `policy_run_bounds_test.go`의 2s sleep은 자식 종료를 쓰기 부재로 증명하는 데 필요해 유지.
- D5: `ArtifactVerificationService.Validate`가 받은 ctx로 읽기 트랜잭션을 연다. RED: 취소된 ctx에서 레코드를 읽음 → GREEN: 읽기 전 `context.Canceled`. durable 기록(`Record`의 `WithoutCancel`, `issue_create.go`)은 그대로.

### B. 문서와 도움말
- B1: usage에 `state maintain`, `contract conformance baseline|live|replay|serve`, `project append|commit-suggest|lint-diagnose`, self-verify `--base-ref`·`--collect-all-steps`, IssueOps 카탈로그에 `benchmark reliability|consensus`. usage golden은 줄 추가와 self-verify 한 줄의 플래그 추가만.
- B2: OPERATIONS·runtime 명령 목록(26 harness 명령, contract schema 기준 CLI 63·MCP 50), README.en 개수·명령표, 없는 심볼·테스트 이름, worker 모델 서술, 소유 문서 링크, TECH_STACK 의존성·upstream, OPERATIONS 스킬 목록, CAUTIONS 색인 표기, hexagonal-core 테스트 이름.
- B3: ADR 6건 superseded 표기(ADR.md 색인, adr/README.md 사유), roadmap 역사 기록 표기. record 본문 불변.
- B4: PROJECT_AUDIT 384 → 128줄. 제거로 닫힘 17건, 현재 근거로 해결 10건, 표에 없던 P2 2건(CP3, W6)을 범위 밖 표로, #548 수정 6건을 집계 제외 표로. 상세 절은 `archive/issueops-audit.md` Appendix C. 파서(`CollectAuditItems`) 판독 0건 0경고.
- B5: stability-audit hook 검사를 템플릿에 실제 등록된 hook(현재 SessionStart)으로 한정하고 설치본의 잔존 issueops hook을 실패로 본다. 타사 hook은 실행하지 않는다. 테스트 두 파일을 `e2e_stability_audit_test.py` 하나로 합쳤다(30건).
- B6: `provider/resolve.go` 주석, quality contract fixture 경로(golden 2줄). `internal/architecture` core 가드와 `guard/paths.go`는 불변(G14). `HookFailureLogFile`과 `hook-metrics.jsonl`은 기존 사용자 state 디렉터리의 파일을 doctor가 알려진 파일로 인식하는 데 필요해 유지.
- 범위 보강(사용자 요청 2026-10-07): `qualitycatalog` 후보 두 개의 WhyNow·Evidence가 해결된 감사 항목을 현재형으로 "flags"라 적던 것을 "(P1, resolved)"로 고쳤다(response golden 4줄). `documentation/README.md`의 archive 설명, `mcp_sdk_server.go` 주석. `documentation/AUDIT.md`는 스스로 밝힌 2026-08-11 스냅샷이라 유지.

## ai-slop-clean

- 범위: 이번 diff의 파일과 직접 관련된 파일.
- 제거: dead-code 1건. issueopsapp의 `printJSONTo` 별칭과 그것만 검사하던 테스트 줄이다(D1 이후 production 호출자 0).
- 유지: 추가한 주석(부하에 따른 세션 지연, executor의 env 정리, 실제 HOME probe, 전역 git 설정 배제 등 코드만으로 드러나지 않는 이유), projectcli·statuscli의 비슷한 `TestMain` 두 개(다루는 환경 변수가 다르고 두 곳뿐), trim하는 `firstNonEmpty` 4개(`cmp.Or`와 동작이 다름).
- 측정(코드 파일 `*.go *.py *.sh *.yml`의 추가 줄과 untracked 파일, 주석·print 줄을 잡음으로 분류): 정리 전 SNR 0.938(signal 1108, noise 72, total 1180), 정리 후 SNR 0.938(signal 1106, noise 72, total 1178). 지운 2줄이 모두 signal이라 비율은 그대로다. 남은 잡음 72줄은 위에서 유지한 이유 설명 주석과 shellcheck 지시문이고, 기준(≥0.75)을 이미 넘으므로 개선 요구는 "waived with justification"으로 둔다. markdown은 제목 줄이 잡음으로 잘못 분류돼 범위에서 뺐다.
- 범위 밖 발견: `internal/adapter/codex/install_hooks.go`의 `codexLifecycleHookEvents`, `benchmarkartifact` 예시 문구의 `internal/core` 경로(아래 남은 일).

## 계약 표면과 side effect

- CLI JSON·MCP schema·도구 이름·record schema: 변경 없음. `mcp_tools.golden.json` 불변.
- golden: usage(줄 추가, self-verify 한 줄 플래그 추가), response_contracts(경로 문자열 2줄, 후보 문구 4줄).
- `quality inspect` warning 문자열에 실패 패키지가 덧붙는다. finding id·severity 불변, warning을 파싱하는 코드 없음.
- 파일: 새 패키지 `cmd/issueops/jsonout`, 새 헬퍼 `internal/testsupport/git_config.go`, 삭제 `skills/stability-audit/scripts/test_e2e_stability_audit.py`. architecture inventory는 의도한 심볼만 변경.
- 툴체인: `GOTOOLCHAIN=local`이면서 Go 1.26.6 미만인 환경은 빌드가 거부된다.
- 원격·DB: 변경 없음. DB 마이그레이션 없음.
- 롤백: 묶음별 커밋(A, C, D, B) 단위 revert.

## 검증

게이트 원장 `.issueops/issues/548/gates.md`의 EVIDENCE가 단일 실행 결과다.

- 환경: darwin/arm64, Go 1.26.6(`GOTOOLCHAIN=auto`가 받음), golangci-lint v2.12.2(`go run`으로 go.mod 툴체인 빌드), shellcheck 0.11.0.
- Python 테스트 의존성: 로컬 python3에 `scripts/python_test_requirements.txt`(pydantic, typer)가 없어 self-verify의 Python 단계가 `skills/slack-delegate` 테스트 import에서 실패했다. 이번 변경과 무관하며, CI와 같이 uv venv(Python 3.13)에 설치하고 PATH 앞에 둔 상태로 원장을 실행했다.
- 원장 수정: CHECK의 `\n` escape를 원장 파서가 줄바꿈으로 풀어 SyntaxError가 났으므로 `chr(10)`으로 바꿨다. G9 EXPECT는 Go regexp가 지원하지 않는 lookahead를 쓰고 있어 `COLLECTION_STATUS=ok`(값은 ok·error 두 가지)로 바꿨다. G12는 계획 문구대로 현행 `*.md`만 보고, 날짜 기록 디렉터리와 "없음·제거됨·당시 경로·경로 이동" 등으로 역사 언급임을 밝힌 줄을 제외한다. PROJECT_AUDIT의 제거된 서브시스템 표에서 표지가 없던 경로 셀 3개에 `(removed)`를 붙였다. 기대 결과를 완화한 수정은 없다.
- 900초 정책 상한: race 전체(G4)와 self-verify(G13)를 포함해 상한 안에서 끝났다.

## 남은 일

- 후속 후보: `internal/adapter/codex/install_hooks.go`의 `codexLifecycleHookEvents`에 남은 Stop·UserPromptSubmit 등은 예전 hook 정리용으로 보이며 이번 범위에서 다루지 않았다.
- `cmd/issueops/issueopscli/benchmarkartifact`의 예시 작업 분해 문구에 `internal/core/...` 경로가 남아 있다. benchmark fixture 텍스트라 점수 영향 검토 없이 바꾸지 않았다.
