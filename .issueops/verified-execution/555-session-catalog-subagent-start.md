# #555 세션 시작 문서 카탈로그 개선과 SubagentStart 훅: verified-execution report

- lifecycle: `io-24725ec94e57`, direct execution generation 2(generation 1 준비 세션이 release한 뒤 Orca로 띄운 새 Claude 세션이 replace·claim으로 인수)
- issue: https://github.com/m16khb-org/issueops/issues/555
- branch: `555-session-catalog-subagent-start`, base `main` @ `b8acdf1c93a552344ae038c5af8db89a23797106`(구현 진입 시 sync-base preview `merge_needed=false`)
- 계획: `.issueops/issues/555/plan.md`, 게이트 원장: `.issueops/issues/555/gates.md`
- 상태: 5단계 정리에서 확정했다. 6단계 문서 반영과 7단계 검증 결과는 이 파일에 이어 적는다.

## 수용 기준별 결과

| 기준 | 결과 | 근거 |
|---|---|---|
| 모든 호스트의 모델 텍스트가 `.issueops/` 경로가 포함된 같은 catalog | 충족 | G1, G2(실제 바이너리에서 codex·claude `additionalContext` 바이트 일치) |
| 화면 표시는 레포 이름이 든 한 줄, worktree에서도 원본 레포 이름 | 충족 | G3, G3b |
| 표준 문서 설명이 "무엇 + 언제 읽는지" 형태 | 충족 | G4 `TestStandardDocDescriptionsSayWhatAndWhenToRead`, 이 레포 표준 문서 11개 frontmatter 동기화 |
| `hook subagent-start`가 같은 catalog를 `systemMessage` 없이 내고 Explore·explorer·fork는 `{}` | 충족 | G4, G5, G5b |
| Claude·Codex 기본 설치가 SubagentStart를 등록하고 다른 도구 그룹을 보존 | 충족 | G6(설치 golden·병합 보존 테스트), G9, G11 |
| compact·tool use 훅은 추가하지 않음 | 충족 | diff에 PreToolUse·PostToolUse·Stop·PreCompact·UserPromptSubmit 등록 없음, ADR 거절 절 |

## T1. 모델 텍스트 통일

- `FormatProjectDocCatalog`가 첫 줄 `Project docs under .issueops/ (read the ones relevant to the task before acting):`와 문서마다 `- .issueops/<NAME>.md: <설명>` 줄을 낸다. 설명·제목이 모두 없으면 경로만 쓴다. omission은 마지막 줄 `omitted: <summary>`다. discovery·정렬·상한은 그대로다.
- `FormatHookContext`에서 Codex 분기를 지웠다. 두 호스트 모두 compact를 `additionalContext`에 싣고, `systemMessage`는 codex가 아니고 userView가 있을 때만 붙인다(기존 규칙).
- RED: 바뀐 기대값으로 projectdoc·hostprotocol·application·hookcli 테스트 9개 실패 → GREEN.

## T2. 레포 이름 한 줄

- `CatalogService.RepoName func(string) string`을 추가하고 `FormatUserView`를 `func(repoName, docs)`로 넓혔다. 문서가 없으면 `RepoName`을 부르지 않는다.
- `RenderProjectDocCatalogUserView`는 `📚 <repo> · project docs <N>개 (.issueops/)`를 낸다. 이름이 비면 `📚 project docs <N>개 (.issueops/)`다.
- `hook_facade.go`의 `hookRepoName`이 `install.ResolveStableNativeRoot`의 basename을 쓰고, 오류면 `filepath.Abs`의 basename을 쓴다. 새 gitdir 파서나 git 실행은 없다.
- PostCompact(Omo·진단용, 기본 미등록)의 `systemMessage`도 같은 user view라서 한 줄이 된다. Omo 확장은 `--json`의 `compact`만 쓰므로 영향이 없다. 해당 테스트 단언을 한 줄 기준으로 바꿨다.

## T3. 표준 설명과 `hook subagent-start`

- `meta.go` 13개 설명을 `<무엇>; read <언제>.` 형식의 120자 이하 영어 한 문장으로 바꿨다. 형식은 새 테스트가 고정한다.
- `hookinput.AgentTypeFromHookInput`을 추가하고 repo 해석과 같은 `firstValue`를 공유한다(top-level → `hook_input` 중첩).
- `RunSubagentStart`: agent_type이 소문자 기준 `explore`·`explorer`·`fork`면 `{}`, 아니면 `FormatContext(host, "SubagentStart", cat.Compact, "")`. live probe 기록은 부르지 않는다.
- `RunHook` case, `hookUsage`, 최상위 usage와 `cmd/issueops/testdata/usage.golden.txt`를 `session-start|subagent-start|post-compact`로 갱신했다.
- bootstrap이 다른 레포의 OPERATIONS.md에 렌더하는 hook 절을 `## Context hooks`로 바꾸고 subagent-start 줄을 넣었다(`TestRenderedOperationsNamesEveryContextHook`, RED → GREEN).

- `internal/architecture/testdata/ddd_responsibility_inventory.json`을 `-update-ddd-inventory`로 갱신했다. diff는 새 심볼 `hookRepoName`·`RunSubagentStart`·`AgentTypeFromHookInput`·`firstValue`와 `firstRepoKey` → `firstKey` 이름 변경뿐이다.

## T4. 설치기

- Claude·Codex spec에 `SubagentStart`/`subagent-start`/timeout 5를 추가하고 known event 목록에 `SubagentStart`를 넣었다. 병합 로직은 바꾸지 않았다. Claude matcher는 쓰지 않는다.
- 설치 golden(`-update-adapter-contract`) diff는 네 hook 파일 내용에 SubagentStart 그룹 추가와 그 sha256 변경뿐이다. 추적 템플릿 `configs/{codex/hooks.json,claude/hooks.settings.json}`은 golden의 `./bin/issueops` 출력과 바이트 단위로 같다.
- native integration 오류 문구를 `Codex thin context hooks missing issueops context hook surface`로 바꿨다. 성공 픽스처에 SubagentStart를 넣었고, 거부 케이스들도 SubagentStart를 갖춘 상태에서 원래 결함 하나로만 거부되게 했다. "SessionStart without SubagentStart" 거부 케이스를 추가했다.
- child-host smoke 스크립트 contract 두 곳에 SubagentStart를 넣고 subcommand를 이벤트 매핑으로 바꿨다. 가짜 설치기와 source 픽스처 두 곳을 같이 고쳤다(RED: 스크립트 테스트 5개 이상 실패 → GREEN).
- 설치 완료 출력 `Codex SessionStart hook:` → `Codex context hooks:`.
- QA: 임시 HOME에서 `issueops install --dry-run --json --mcp-transport stdio --path-mode skip` 실행. exit 0, `dry_run: true`, 계획에 `codex_user_hooks_config`·`claude_user_settings`·두 템플릿이 있고 임시 HOME에 파일이 생기지 않았다. worktree·source checkout status 변화 없음. dry-run JSON은 파일 내용을 담지 않으므로 내용은 golden과 G9로 확인한다.

## T5. 문서

- 새 ADR `.issueops/adr/2026-10-08-subagent-start-catalog-and-unified-model-text.md`, ADR 색인 추가와 2026-08-27 항목의 부분 대체 표시.
- "SessionStart 하나" 서술 갱신: CONVENTIONS, conventions 모듈 2개, operations 가이드 2개, CAUTIONS 색인과 lifecycle 모듈, architecture 모듈 3개와 ARCHITECTURE, testing 모듈, README 두 개, stability-audit 스킬.
- 이 레포 표준 문서 11개의 frontmatter description을 새 표준 문구로 동기화했다(`EnsureMetaFrontmatter`와 같은 형식, line 3만 교체). `PROJECT_AUDIT.md`·`SUB_AGENT_PATTERNS.md`는 표준이 아니라 그대로다. ADR.md·OPERATIONS.md는 이전에 표준과 다른 설명을 갖고 있었고 이번에 표준 문구가 됐다.
- `operations/guides/hosts.md`가 250줄 예산을 1줄 넘어 새 bullet을 기존 줄에 합쳤다(250줄).

## ai-slop-clean

- 범위: 이번 diff의 파일과 직접 관련된 파일.
- 주장 정리(unsupported-claim): ADR 근거 절의 "205회 모두 빈 컨텍스트에서 시작했다"를 "fork가 아닌 서브에이전트는 빈 컨텍스트에서 시작한다"로 낮췄다. 거절 절의 `--id` 유지 수치가 catalog 재주입의 근거처럼 붙어 있던 문장을 둘로 나눴고, raw `gh pr create` 2건을 "정상 처리됐다"고 단정한 부분을 관측 건수만 남겼다(인계 자료의 측정 범위).
- 약한 산출물(weak-artifact): report의 RED 기준선 시각 `04:3xZ`를 실제 `gates check` 기록 시각으로 바꿨다.
- 주장 재확인: 코드 주석과 ADR의 host 버전 사실을 설치본에서 다시 확인했다. `claude --version` 2.1.293, `codex --version` codex-cli 0.161.0, 같은 Codex 바이너리에 `subagent-start.command.input`·`subagent-start.command.output` schema 문자열이 있다.
- 유지: `RunSubagentStart`의 flag 파싱 세 줄과 stdin 읽기는 `RunSessionStart`·`RunPostCompact`와 같은 반복이다. 공통 helper로 묶으면 계획이 바꾸지 않기로 한 PostCompact 경로까지 고치게 되므로 기존 관례대로 둔다. host 버전을 적은 주석 세 곳(hookcatalog, 두 설치기)은 각 파일의 결정 근거라 남긴다.
- 측정(`*.go *.sh *.py`의 base 대비 추가 줄과 untracked Go 파일, 주석을 잡음으로 분류): 정리 전 SNR 0.865(signal 339, noise 53, total 392, 20자 넘는 중복 줄 21), 정리 후 같은 값이다. 이번 정리는 코드 줄을 바꾸지 않았다.
- 범위 밖 발견: 없음.
- 7단계 구현 리뷰 뒤 추가 정리(weak-artifact): 리뷰가 비차단 참고로 짚은 낡은 주석 두 곳을 고쳤다. `internal/contract/hookprompt/types.go`의 DTO 주석이 Compact를 "one-line menu", UserView를 "readable list"로 설명했고, `RunPostCompact` 주석이 "readable catalog"를 systemMessage로 싣는다고 적었다. 바뀐 동작(Compact는 경로 목록, UserView는 한 줄 알림)에 맞췄다. 계획이 인용한 caution 2026-10-06(help text와 주장이 뒤처진다)의 함정이라 게시 전에 고쳤다. 코드 동작은 그대로이며 G4·G7을 pending으로 되돌려 다시 실행했다(05:17:34Z, 13/13).

## 프로젝트 문서 반영

- 문서 → 구현: CONSTITUTION(:65 hook은 core 정책을 우회하지 않는다, :197 contract 변경은 golden과 실제 command smoke), CONVENTIONS와 `conventions/state-policy-and-hooks.md`(hook 설정은 adapter/template, catalog 구성은 공통 CLI/core), ADR 2026-08-10(정적 catalog만, state 접근 없음)과 대조했다. 위반 없음. application 계층은 `path/filepath`를 import하지 않고 RepoName을 주입받는다(`internal/architecture` 의존성 테스트 통과, G7).
- 구현 → 문서: 계획 T5의 ADR과 서술 갱신은 4단계에 들어갔고 최종 diff와 맞는지 다시 확인했다. 계획의 `## 적용되는 결정과 주의사항`에 없던 함정 하나를 새 주의사항으로 남겼다: `.issueops/cautions/2026-10-08-changing-a-standard-doc-description-does-not-change-the-cata.md`(`issueops project append --kind caution`). catalog는 frontmatter description을 먼저 읽으므로 `meta.go`만 바꾸면 기존 레포의 hook 출력은 `--sync` 전까지 그대로다. append가 색인 줄을 넣지 않아 `.issueops/CAUTIONS.md` 표에 한 줄을 직접 넣었다.
- 두 번째 새 주의사항 `.issueops/cautions/2026-10-08-tracked-intent-copies-quote-the-raw-request-and-local-self-v.md`: 추적 intent 사본은 요청 원문을 그대로 담고, 커밋 전 로컬 self-verify는 추적 파일 검사를 보지 못한다(CI 실패에서 찾음). 게이트 재실행 함정(충족 게이트는 다시 돌지 않는다)은 `cautions/2026-09-25-gate-check-15-minute-cap-under-host-load.md`에 이미 있어 새로 적지 않았다.
- 검증: `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1` → ok(드리프트 없음, `-update` 불필요). G8·G10을 pending으로 되돌려 다시 실행 → `DOCS_OK`, `NO_STALE_CLAIMS`(05:05:32Z). docs checker 위반은 base부터 있던 `.issueops/adr/roadmap.md` 하나뿐이다.

## 하위 호환성과 side effect

- 파일: 다음 `io update` 때 `~/.claude/settings.json`과 `~/.codex/hooks.json`에 SubagentStart issueops 그룹이 하나씩 생긴다. 다른 도구 그룹은 보존된다. Codex는 새 항목을 한 번 신뢰 승인해야 실행한다.
- 출력: Codex `additionalContext`가 한국어 bullet에서 영어 경로 목록으로, Claude `systemMessage`가 목록에서 한 줄로 바뀐다. `--json` DTO 필드는 그대로다.
- 다른 레포: 이후 `project bootstrap --sync`가 표준 문서 frontmatter를 새 문구로, OPERATIONS.md hook 절을 새 문구로 바꾼다.
- 토큰: 서브에이전트마다 catalog 1,599바이트(이 레포 기준)가 추가된다. 계획 추정(약 1KB)보다 크다. 문서 13개의 설명이 길어진 탓이다.
- state·lease·telemetry·네트워크 접근 없음(`TestRunHookContextEventsAcrossIsolatedWorktreesDoNotCreateHarnessState`에 subagent-start 포함).
- 원격·record·MCP schema 변경 없음.

## 성능 측정

- 2026-10-08T04:44Z, darwin/arm64, scratchpad 빌드 바이너리, 이 레포 cwd, 12회 중 앞 2회를 버린 10회 중앙값: `session-start --host claude` 18.4ms, `subagent-start --host claude`(general-purpose) 19.0ms(+3%, 목표 ±20% 이내), `subagent-start`(Explore, 제외) 13.7ms.
- SessionStart의 추가 I/O는 `<repo>/.git` Lstat 한 번(worktree면 gitdir 파일·commondir 읽기)이다.

## 검증 기록

- 게이트 결과는 원장 `.issueops/issues/555/gates.md`의 EVIDENCE가 소유한다. 2026-10-08 첫 `--write` 실행(04:45:38Z)에서 13개 모두 충족했다.
- 계획의 게이트 명령은 파이프·`&&`를 써서 command policy를 지나지 못하므로, 같은 판정을 `python3 -c` 래퍼로 옮겼다. 바이너리 게이트(G2·G3b·G5·G5b)는 매번 임시 디렉터리에 새로 빌드해 stale `./bin/issueops`를 피한다. G3b(worktree에서 레포 이름 한 줄)는 계획 T2 QA를 게이트로 만든 것이다. G4에 usage golden 패키지(`./internal/adapter/inbound/catalog/cli/...`, `./cmd/issueops/`)를 더했다. G10의 `\|`·`\(`는 정책을 지나도록 `.`로 바꿨다.
- 구현 진입 RED 기준선(`gates check`, 2026-10-08T04:30:39Z): 행동 게이트 G2·G3b·G5·G5b·G9·G10·G11 미충족, 기존 단위 테스트 게이트는 통과.
- `gates check`는 이미 충족된 게이트를 다시 실행하지 않는다(#552 기록). 5단계 정리 뒤 13개를 모두 pending으로 되돌려 다시 실행했다(04:49:13Z). 이때 G7이 `TestDDDResponsibilityInventoryMatchesSource`로 실패했다. 인벤토리를 T2 뒤에만 갱신하고 T3의 새 심볼을 빠뜨렸는데, 04:45 실행이 G7을 다시 돌리지 않아 드러나지 않았던 것이다. 인벤토리를 갱신한 뒤 G7을 다시 실행해 13개 모두 충족했다(04:59:19Z). 다른 게이트는 인벤토리 파일을 읽지 않는다.
- 로컬 `self-verify --base-ref <봉인 base> --seed=100 --target-score=95 --llm-eval=false --json`(scratchpad 빌드 바이너리, scratchpad uv venv Python 3.13을 PATH 앞에 둠, 2026-10-08T05:06:36Z~05:16:44Z): 25단계 중 실패는 `native integration` 하나이고 오류는 `Codex thin context hooks missing issueops context hook surface`다. 사용자 설치 `~/.codex/hooks.json`에 아직 SubagentStart가 없어서이며 계획이 예상한 실패다. 그 밖의 실패는 없다. 이 실행은 위 주석 두 곳을 고치기 전 트리에서 돌았다(주석만 다르다). 권위 있는 증거는 push 뒤 CI의 임시 HOME self-verify다.
- push 뒤 첫 CI run(2026-10-08T05:24Z 시작)이 임시 HOME self-verify 단계에서 실패했다. 원인은 Python 루트 스위트의 `meeting_notes_skill_contract_test.test_synthetic_fixture_family_has_no_identified_meeting_data`다. 추적 사본 `.issueops/issues/555/intent.md`의 원문 요청 인용에 다른 비공개 레포 이름이 들어 있었고, 이 테스트는 추적 파일에 그 이름이 있으면 실패한다. 로컬 self-verify가 통과한 것은 이 검사가 `git ls-files`만 읽고 당시 사본이 미추적이었기 때문이다. 추적 사본에서 그 이름만 `<다른 레포>`로 바꿨다(봉인 원본은 ignored로 두고, 사본을 다시 쓰는 전이는 `implement`·`ai-slop-clean`뿐이라 `pr` 전이가 되돌리지 않는다). 수정본을 커밋한 임시 detached worktree에서 `scripts/python_suite_runner.py`를 실행해 루트·스킬 스위트가 모두 통과했다. 이 함정을 새 주의사항으로 남겼다.
- 7단계 병렬 실행(`parallel_speed`): self-verify(약 10분)와 구현 리뷰 서브에이전트(약 9분)를 같은 fingerprint에서 동시에 돌려 벽시계 약 9분을 줄였다.
