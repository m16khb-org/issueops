# 555 세션 시작 문서 카탈로그 개선과 SubagentStart 훅

## TL;DR

- Summary: SessionStart 카탈로그의 모델용 텍스트를 경로가 포함된 한 형식으로 통일하고, 화면 표시를 레포 이름이 들어간 한 줄로 줄이며, 표준 문서 설명을 "언제 읽는지" 형태로 바꾼다. 같은 카탈로그를 내보내는 `SubagentStart` 훅을 Claude·Codex 기본 설치에 추가한다.
- Deliverables: catalog 렌더 변경, `hook subagent-start` 서브커맨드, 두 설치기의 SubagentStart 등록, golden·테스트 갱신, 새 ADR과 운영·컨벤션 문서 갱신, 이 레포 표준 문서의 frontmatter 동기화
- Effort: Medium
- Parallel: NO (한 owner, 순차 TDD)
- Critical Path: T1 → T2 → T3 → T4 → T5

## 사이클 경계

- lifecycle ID: `io-24725ec94e57`
- issue: https://github.com/m16khb-org/issueops/issues/555
- branch: `555-session-catalog-subagent-start`, base `main` (봉인된 SHA는 record가 소유한다)
- 사용자 요청 범위: 조사 결과를 하나의 이슈로 묶어 `/issueops`로 진행. 종료점은 draft PR 발행과 `execution complete`다. merge와 cleanup은 별도 승인이다.
- 브랜치·worktree 준비 뒤 환경별 자동 세션 인계를 적용한다. 자동 인계는 위 승인 범위를 넓히지 않는다.

## Context

### Original Request

세션 시작 훅이 넣는 project docs 힌트의 품질 개선, 추가 훅 지정, Claude Code와 Codex 훅 관리 차이를 조사하고, 그 결과를 하나의 이슈로 묶어 진행한다. compact 훅과 tool use 훅에 처리할 것이 있는지도 확인한다.

### Interview Summary

- SubagentStart를 같은 이슈에 포함한다(사용자: "하나의 이슈로 묶어서 작업").
- compact·tool use 훅은 조사 결과 추가하지 않는다(근거는 이슈 본문 표).
- `agent-harness` 잔재는 rename 결정(커밋 `53fe530b`: "no compatibility reader or alias ships")에 따라 범위 밖이다.

### Gap Analysis

- 워크트리 세션에서 레포 이름을 경로 basename으로 쓰면 브랜치 이름이 표시된다. worktree에서 원본 checkout을 찾는 판정은 이미 `install.ResolveStableNativeRoot`(`internal/adapter/install/native_root.go:15-61`)가 exec 없이 gitdir·commondir 파일로 수행하므로 그것을 재사용한다(계획 리뷰 1차 지적).
- 로컬 self-verify의 native integration 검사는 실제 `$HOME/.codex/hooks.json`을 새 기대 설정과 비교한다(`internal/adapter/verification/probe/nativeintegration/validation_native_integration_contract.go:54-69`, `internal/adapter/installutil/activation.go:69-72`). 머지 전에는 사용자 설치에 SubagentStart가 없으므로 이 검사는 로컬에서 반드시 실패한다. worktree에서 임시 HOME으로 설치하는 우회도 쓸 수 없다. `scripts/install-native.sh:6`이 Git common-dir의 source `bin/issueops`를 활성화 대상으로 쓰기 때문에, 사용자의 실제 훅 target을 머지 전 빌드로 바꾸게 된다. 그래서 self-verify의 권위 있는 증거는 push마다 도는 CI의 임시 HOME self-verify 단계(`.github/workflows/ci.yml:129-138`)로 삼고, 로컬 실행은 그 한 항목만 실패하는지 확인하는 보조 증거로 둔다(계획 리뷰 1차 지적).
- 롤백할 때 이전 바이너리는 기대 목록에 없는 이벤트의 issueops 그룹을 보면 activation readback에서 `installed hook event %s contains an unexpected issueops group`으로 실패한다(`internal/adapter/installutil/activation.go:57-61`). 그래서 `io update` 자체가 실패한다. 롤백 절차는 설정 파일에서 SubagentStart 그룹을 먼저 지우고 그다음 `io update`를 실행하는 순서여야 한다(계획 리뷰 1차 지적).
- 훅 개수를 "SessionStart 하나"로 서술하는 곳은 컨벤션 외에도 추적 템플릿, 최상위 usage와 golden, bootstrap 렌더 문구, README, stability-audit 스킬에 있다. 이번 범위에 모두 넣는다(계획 리뷰 1차 지적, caution 2026-10-06).
- Codex는 `additionalContext`를 TUI의 `hook context:` 줄에 보여 주고 줄바꿈을 접을 수 있다(`.issueops/cautions/integrations.md` §14). 모델 텍스트를 통일하면 Codex 사용자는 영어 목록을 보게 된다. 이것은 의도한 결과이며 ADR에 기록한다.
- Codex는 새 훅 항목마다 사용자 신뢰 승인(`config.toml`의 `[hooks.state]` trusted_hash)을 요구한다. 설치기는 이 승인을 대행하지 않는다(코드에 trusted_hash 쓰기 없음: `rg trusted_hash internal cmd` 결과 0건). 운영 문서에 한 번 승인이 필요하다고 적는다.
- Claude fork 서브에이전트가 SubagentStart를 발생시키는지, Codex의 agent_type 값이 무엇인지는 확인하지 못했다(미확인 가정). 제외 목록을 대소문자 무시 `explore`, `explorer`, `fork`로 두면 확인 결과와 상관없이 안전하다.

## 적용되는 결정과 주의사항

- `.issueops/adr/2026-08-27-session-start-owns-compaction-context.md` — "기본 설치는 SessionStart 하나만 등록": 이 계획은 SubagentStart를 추가하므로 이 조항을 대체하는 새 ADR이 필요하다. PostCompact 미등록과 legacy hook 삭제 결론은 유지한다.
- `.issueops/adr/2026-08-10-default-hooks-thin-static-context.md` — "hook은 정적 project-doc catalog만 렌더하고 IssueOps state를 읽지 않는다": SubagentStart 훅도 같은 정적 catalog만 읽는다. state·lease·telemetry 접근을 넣지 않는다.
- `.issueops/CONSTITUTION.md:65` — "Codex plugin이나 Claude hook은 core 정책을 우회하지 않는다": 훅은 context만 낸다.
- `.issueops/CONSTITUTION.md:197` — "CLI/MCP contract 변경은 JSON schema/golden test와 실제 command smoke test를 남긴다": 설치 golden 갱신과 실제 hook 실행 smoke를 남긴다.
- `.issueops/CONVENTIONS.md:76` — "hook은 SessionStart 하나만 context-only로 등록": 문서 갱신 대상이다.
- `.issueops/conventions/state-policy-and-hooks.md:43` — 호스트별 hook 출력 분리 규칙(Codex는 readable catalog를 additionalContext에): 모델 텍스트 통일로 바뀌므로 갱신한다. `--json` snake_case DTO 필드는 유지한다.
- `.issueops/conventions/state-policy-and-hooks.md:44` — "frontmatter description은 짧은 영어 category 설명": "무엇 + 언제 읽는지"의 짧은 영어 설명으로 갱신한다.
- `.issueops/conventions/state-policy-and-hooks.md:45` — "Codex/Claude별 hook 설정은 adapter/template에서만, catalog 구성은 공통 CLI/core": 호스트 차이는 `hostprotocol`과 설치기에만 둔다.
- `.issueops/cautions/integrations.md` §14 — Codex는 additionalContext를 TUI에 보여 준다: 모델 텍스트가 사용자에게도 보인다는 전제로 형식을 정한다.
- `.issueops/cautions/integrations.md:14` — native hook 설치는 source `bin/issueops`를 atomic rename으로 활성화하고 이전 target을 캐시한 세션은 재시작: 검증 단계에서 worktree binary를 영구 target으로 설치하지 않는다.
- `.issueops/cautions/2026-08-28-verify-host-cli-flags-against-the-installed-version.md` — 설치된 버전으로 host 동작을 확인: SubagentStart 스키마는 Claude Code 2.1.293과 Codex 0.161.0 바이너리에서 확인했다.
- `.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md` — help text와 문서 주장이 뒤처진다: `hookUsage`와 운영 문서 문구를 함께 갱신한다.
- `.issueops/TESTING.md` — golden 갱신은 `-update` 뒤 diff가 의도한 변경만 담는지 확인한다.

## 재사용하는 기존 구현

- `internal/application/hookprompt/catalog.go` `CatalogService.Build`: 그대로 재사용한다. 레포 이름이 필요한 `FormatUserView`에는 repo 경로를 넘기도록 시그니처만 넓힌다(`func([]Entry) string` → `func(repo string, docs []Entry) string`).
- `internal/adapter/projectdoc/catalog.go:245` `FormatProjectDocCatalog`: 경로 접두사를 지우는 대신 `.issueops/` 경로와 줄 단위 목록을 내도록 수정한다. discovery(`DiscoverProjectDocsReport`)는 바꾸지 않는다.
- `internal/adapter/hookprompt/catalog.go:17` `RenderProjectDocCatalogUserView`: 고정 헤더(`:22`)를 레포 이름과 문서 개수가 들어간 한 줄로 바꾼다. `RenderProjectDocCatalogOmissions`는 유지한다.
- `internal/adapter/hostprotocol/hook.go:5` `FormatHookContext`: Codex 분기(`host == "codex"`면 userView를 additionalContext로)를 지우고, 모델 텍스트는 항상 compact를 쓴다. `systemMessage`는 codex가 아니고 userView가 비어 있지 않을 때만 붙인다(기존 규칙 유지).
- `cmd/issueops/hookcli/hookcatalog/catalog.go:26` `RunSessionStart`: 같은 패턴으로 `RunSubagentStart`를 추가한다. flag 파싱, `resolveRepo`, `hostOf`, `config.PrintJSON`을 재사용한다. live probe 기록(`recordLiveProbeSessionStart`)은 SessionStart 전용이라 호출하지 않는다.
- `cmd/issueops/hookcli/hookinput/hook_input.go:14` `RepoFromHookInput`: cwd 해석을 재사용하고, 같은 파일에 `AgentTypeFromHookInput`을 추가한다.
- `cmd/issueops/hookcli/hook.go:23` `RunHook`: `subagent-start` case를 추가한다. `ISSUEOPS_DISABLE_HOOKS` 처리는 기존 진입부가 그대로 적용한다.
- `internal/adapter/claude/install_hooks.go:83`·`:92`·`:118`과 `internal/adapter/codex/install_hooks.go:72`·`:93`·`:150`: spec 목록에 SubagentStart를 추가하고 known event 목록에 `SubagentStart`를 넣는다. 병합 로직(`mergeClaudeHookConfig`, `mergeHookConfig`)은 수정하지 않는다.
- `internal/adapter/install_contract_matrix_test.go:95`의 `assertAdapterContractGolden`: golden 재생성에 그대로 쓴다.
- `internal/adapter/install/native_root.go:15` `ResolveStableNativeRoot`: worktree 경로를 원본 checkout 경로로 바꾸는 기존 판정이다. `cmd/issueops/issueopsapp/probe_native_wiring.go:12`처럼 cmd 계층 wiring에서 이미 쓰고 있다. `hook_facade.go`에서 레포 이름 해석 함수(`filepath.Base(ResolveStableNativeRoot(repo))`, 오류면 `filepath.Base(filepath.Abs(repo))`)를 만들어 user view 렌더러에 주입한다. adapter/hookprompt에는 새 I/O 함수를 만들지 않는다.
- `internal/adapter/codex/install.go:32-33`, `internal/adapter/claude/install.go:32-33`: 설치기가 추적 템플릿 `configs/codex/hooks.json`·`configs/claude/hooks.settings.json`을 `HooksConfig("./bin/issueops")`로 다시 쓴다. 템플릿은 같은 함수의 출력으로 갱신한다.

## 성능 영향

- SessionStart: 경로는 그대로이고 문자열 렌더만 바뀐다. 추가 I/O는 레포 이름을 구하려고 `<repo>/.git`을 한 번 Lstat·읽는 것뿐이다(파일이면 최대 1KiB, 디렉터리면 읽지 않음).
- SubagentStart: 서브에이전트가 시작될 때마다 Go 바이너리를 한 번 실행한다. 이벤트 빈도는 최근 30일 worktree 세션에서 205회였다. 실행 비용은 SessionStart와 같은 catalog 읽기(최대 64개 × 8KiB)다. timeout은 SessionStart와 같은 5초다.
- 측정: 구현 뒤 이 레포에서 `hook subagent-start`를 10회 실행해 중앙값을 기록한다. 목표는 SessionStart 실행 시간의 ±20% 이내다.
- 토큰: 서브에이전트마다 카탈로그 약 1KB(약 300토큰)가 추가된다. 제외 목록에 든 에이전트에는 `{}`를 낸다.

## 하위 호환성과 side effect

- `hook session-start --json` DTO: 필드(`should_inject`, `project_docs`, `compact`, `user_view`, `omitted`)는 그대로다. `compact` 내용이 경로가 포함된 줄 단위 목록으로 바뀌고, Omo 확장(`configs/omo/issueops.js:16-20`)은 이 값을 그대로 모델에 넣는다. 형식이 바뀌어도 Omo 쪽 코드는 바뀌지 않는다.
- Claude 출력: `additionalContext`는 새 compact 형식이 되고, `systemMessage`는 한 줄이 된다.
- Codex 출력: `additionalContext`가 한국어 bullet 목록에서 영어 compact 목록으로 바뀐다. TUI에 보이는 내용도 같이 바뀐다.
- 설치 결과: 두 호스트 설정 파일에 `SubagentStart` 그룹이 하나씩 추가된다. 기존 `SessionStart` 명령 문자열은 그대로라서 Codex에 저장된 기존 신뢰 해시가 유지된다. 새 `SubagentStart` 항목은 Codex에서 한 번 승인해야 실행된다.
- `native_install_contract_matrix.golden.json`과 hookcli·hostprotocol·projectdoc 테스트의 기대값을 갱신한다. `internal/architecture/testdata/ddd_responsibility_inventory.json`에 새 함수가 잡히면 그 테스트의 갱신 절차를 따른다.
- 표준 설명(`meta.go`)이 바뀌면 이후 `project bootstrap --sync`를 실행하는 모든 레포의 frontmatter가 새 문구로 바뀐다. 이 레포의 표준 문서 frontmatter도 이번 변경에 맞춰 동기화한다.
- 롤백: 순서가 중요하다. (1) `~/.claude/settings.json`과 `~/.codex/hooks.json`에서 `hook subagent-start`를 부르는 그룹을 먼저 지운다. (2) 커밋을 되돌린 빌드로 `io update`를 실행한다. 순서를 바꾸면 이전 바이너리의 activation readback이 `installed hook event SubagentStart contains an unexpected issueops group`으로 실패한다(`internal/adapter/installutil/activation.go:57-61`). ADR의 롤백 절에 이 순서와 오류 문자열을 적는다.
- 로컬 self-verify: 머지 전에는 사용자 설치에 SubagentStart가 없어서 native integration 검사가 `Codex thin context hooks …` 항목 하나로 실패한다. 머지 뒤 source checkout에서 `io update`를 실행하면 해소된다. 이 실패는 설치 상태 차이이며 코드 결함이 아니다. 검증 단계는 CI의 임시 HOME self-verify 통과를 권위 있는 증거로 쓴다.
- native integration 성공 픽스처: SessionStart만 담은 픽스처는 새 기대 설정과 맞지 않아 실패한다. `validation_native_integration_test.go:49,159`와 `internal/adapter/verification/probe/validation_mcp_mermaid_native_wrappers_test.go:108`을 SubagentStart가 포함된 설정으로 갱신한다. 오류 문구(`validation_native_integration_contract.go:55`)는 `Codex thin context hooks missing issueops context hook surface`로 바꾸고 `:126`의 기대값도 맞춘다.
- MCP schema, record schema, provider body 계약에는 변경이 없다. LLM 프롬프트 본문(스킬·리뷰어 프롬프트)도 바꾸지 않는다. 카탈로그 텍스트는 정적 문서 목록이고 프롬프트 템플릿이 아니다.

## Work Objectives

### Core Objective

모든 호스트에서 모델은 경로와 "언제 읽는지"가 포함된 같은 카탈로그를 받고, 사용자는 레포 이름이 든 한 줄만 보며, 빈 컨텍스트로 시작하는 서브에이전트도 같은 카탈로그를 받는다.

### Deliverables

1. compact 카탈로그 형식 변경과 Codex/Claude 모델 텍스트 통일
2. 레포 이름이 든 한 줄짜리 user view
3. `meta.go` 표준 설명 개정과 이 레포 frontmatter 동기화
4. `issueops hook subagent-start` 서브커맨드
5. Claude·Codex 설치기의 SubagentStart 등록과 golden 갱신
6. 새 ADR, CONVENTIONS·conventions 모듈·운영 가이드·ADR 색인 갱신

### Definition of Done

- 아래 게이트 G1–G11(G5b 포함)이 모두 통과하고, push 뒤 CI 확인 절차가 성공한다.

### Must Have

- `.issueops/`가 없는 레포에서는 두 훅 모두 `{}`를 낸다.
- `ISSUEOPS_DISABLE_HOOKS=1`이면 `subagent-start`도 출력하지 않는다.
- 설치 업그레이드가 다른 도구의 SubagentStart 그룹(예: Orca)을 보존한다.

### Must NOT Have

- 훅에서 IssueOps state, lease, telemetry, 네트워크에 접근하지 않는다.
- PreToolUse, PostToolUse, Stop, PreCompact, UserPromptSubmit 등록을 추가하지 않는다.
- `agent-harness` 이름의 legacy 훅을 인식하는 코드를 넣지 않는다.
- Claude SubagentStart에 `systemMessage`를 내지 않는다(서브에이전트마다 화면에 표시되는 것을 막는다).

## Verification Strategy

- Test decision: TDD. Go `testing`, 기존 테스트 파일에 케이스를 추가한다.
- QA: 실제 바이너리로 hook을 실행하는 smoke와 `install --dry-run`을 에이전트가 실행한다.
- Evidence: `.issueops/issues/555/gates.md`의 EVIDENCE

## Execution Strategy

### Parallel Execution Waves

순차 실행한다. 렌더 계약(T1–T2)을 먼저 고정해야 서브커맨드(T3)와 설치기(T4)의 테스트 기대값이 정해진다.

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|---|---|---|---|
| T1 | — | T2, T3 | — |
| T2 | T1 | T3 | — |
| T3 | T2 | T4 | — |
| T4 | T3 | T5 | — |
| T5 | T4 | — | — |

## TODOs

- [ ] 1. compact 카탈로그 형식과 모델 텍스트 통일
  - What to do: `FormatProjectDocCatalog`가 다음 형식을 내게 한다. 첫 줄 `Project docs under .issueops/ (read the ones relevant to the task before acting):`, 이어서 문서마다 `- .issueops/<NAME>.md: <description or title>` 한 줄. 설명이 없으면 경로만 쓴다. omission은 마지막 줄 `omitted: <summary>`로 붙인다. `FormatHookContext`에서 Codex 분기를 지워 두 호스트 모두 compact를 `additionalContext`에 싣는다.
  - Must NOT do: discovery·정렬·상한(64개, 8KiB)을 바꾸지 않는다.
  - Recommended Agent: deep; Reason: 여러 패키지 테스트 기대값이 함께 바뀐다
  - Parallelization: NO; Wave 1; Blocks T2; Blocked By —
  - References: `internal/adapter/projectdoc/catalog.go:245-275`, `internal/adapter/hostprotocol/hook.go:5-19`, `internal/adapter/hostprotocol/hook_test.go`, `internal/adapter/projectdoc/catalog_test.go`, `internal/application/hookprompt/catalog_test.go`
  - Acceptance Criteria: G1, G2
  - QA Scenarios:
    - Happy: Channel shell. Steps: 이 레포에서 `printf '{"cwd":"%s","source":"startup"}' "$PWD" | ./bin/issueops hook session-start --host codex`와 `--host claude`를 실행한다. Expected: 두 출력의 `hookSpecificOutput.additionalContext`가 바이트 단위로 같고 `.issueops/ADR.md:`를 포함한다.
    - Failure/edge: Steps: 빈 임시 디렉터리를 cwd로 실행한다. Expected: `{}`.
  - Commit: YES; Message: `feat(hook): render one model-facing catalog with .issueops paths for every host`; Files: 위 References의 소스와 테스트

- [ ] 2. 레포 이름이 든 한 줄 user view
  - What to do: `CatalogService`에 `RepoName func(string) string` 필드를 추가하고, `FormatUserView` 시그니처를 `func(repoName string, docs []Entry) string`로 넓힌다. `Build`는 `RepoName`이 있으면 그 결과를 넘기고, 없으면 빈 문자열을 넘긴다(application 계층은 `path/filepath`를 import하지 않는다: `internal/architecture/dependency_test.go:1459-1460`). 렌더러는 이름이 비면 `📚 project docs <N>개 (.issueops/)`로 낸다. `catalog.go:9`의 "discovering documents is the only I/O here" 주석은 주입된 `RepoName`도 I/O를 한다는 사실에 맞게 고친다. 출력은 `📚 <repo-name> · project docs <N>개 (.issueops/)` 한 줄이다. omission이 있으면 기존처럼 `⚠` 줄을 덧붙인다. `hook_facade.go`(cmd 계층)가 `RepoName` 클로저로 `filepath.Base(ResolveStableNativeRoot(repo))`를 주입하고, 오류가 나면 같은 클로저 안에서 `filepath.Base(filepath.Abs(repo))`를 쓴다.
  - Must NOT do: git 프로세스를 실행하지 않는다. gitdir 파서를 새로 만들지 않는다.
  - Recommended Agent: quick; Reason: 렌더 함수 하나와 배선 한 곳이다
  - Parallelization: NO; Wave 1; Blocks T3; Blocked By T1
  - References: `internal/adapter/hookprompt/catalog.go:17-35`, `internal/application/hookprompt/catalog.go`(Build), `cmd/issueops/issueopsapp/hook_facade.go:13-27`, `internal/adapter/install/native_root.go:15-61`, `cmd/issueops/issueopsapp/probe_native_wiring.go:12`, `internal/adapter/hookprompt/catalog_test.go`
  - Acceptance Criteria: G3
  - QA Scenarios:
    - Happy: Steps: 이 레포 source checkout과 `issueops.worktrees/555-…` worktree에서 각각 `--host claude`로 실행한다. Expected: 두 경우 모두 `systemMessage`가 `📚 issueops · project docs `로 시작하는 한 줄이다.
    - Failure/edge: Steps: `.git`이 없는 임시 디렉터리에 `.issueops/ADR.md`만 만들고 실행한다. Expected: 디렉터리 basename이 표시되고 오류가 없다.
  - Commit: YES; Message: `feat(hook): show the repository name in a one-line catalog notice`; Files: 위 References

- [ ] 3. 표준 설명 개정과 `hook subagent-start`
  - What to do: (a) `meta.go`의 `docMetaDescriptions` 13개를 "무엇 + 언제 읽는지" 형태의 짧은 영어 한 문장으로 바꾼다(예: `CONVENTIONS.md`: `Coding conventions and layer boundaries; read before writing or restructuring code.`). (b) `hookinput`에 `AgentTypeFromHookInput`을 추가한다. (c) `RunSubagentStart`를 추가한다. agent_type이 대소문자 무시로 `explore`, `explorer`, `fork` 중 하나면 `{}`를 내고, 그 밖에는 `FormatContext(host, "SubagentStart", cat.Compact, "")`를 낸다. user view를 넘기지 않으므로 `systemMessage`는 생기지 않는다. (d) `RunHook`에 case를 추가하고 `hookUsage`, 최상위 usage(`internal/adapter/inbound/catalog/cli/usage.go:47`)와 `cmd/issueops/testdata/usage.golden.txt:111`을 `hook session-start|subagent-start|post-compact`로 갱신한다.
  - Must NOT do: live probe 기록을 호출하지 않는다. PostCompact 경로는 바꾸지 않는다.
  - Recommended Agent: deep; Reason: CLI 표면 추가와 문서 메타데이터 계약 변경이 함께 있다
  - Parallelization: NO; Wave 1; Blocks T4; Blocked By T2
  - References: `internal/domain/projectdoc/meta.go:11-33`, `internal/domain/projectdoc/meta_test.go`, `cmd/issueops/hookcli/hook.go:23-55`, `internal/adapter/inbound/catalog/cli/usage.go:47`, `cmd/issueops/testdata/usage.golden.txt:111`, `cmd/issueops/hookcli/hookcatalog/catalog.go:26-53`, `cmd/issueops/hookcli/hookinput/hook_input.go:14-29`, `cmd/issueops/hookcli/hook_catalog_test.go`, `cmd/issueops/hookcli/hookcatalog/catalog_test.go`
  - Acceptance Criteria: G4, G5, G5b
  - QA Scenarios:
    - Happy: Steps: `printf '{"cwd":"%s","agent_type":"general-purpose"}' "$PWD" | ./bin/issueops hook subagent-start --host claude`. Expected: `hookSpecificOutput.hookEventName == "SubagentStart"`, `additionalContext`가 session-start와 같고 `systemMessage` 키가 없다.
    - Failure/edge: Steps: agent_type을 `Explore`로 바꾸고, 따로 `ISSUEOPS_DISABLE_HOOKS=1`로도 실행한다. Expected: 각각 `{}`와 빈 출력.
  - Commit: YES; Message: `feat(hook): add a SubagentStart catalog hook and when-to-read doc descriptions`; Files: 위 References

- [ ] 4. 설치기 SubagentStart 등록과 golden
  - What to do: Claude·Codex spec 목록에 `{Event: "SubagentStart", Subcommand: "subagent-start", Timeout: 5}`를 추가하고, 두 known event 목록에 `"SubagentStart"`를 넣는다. `claudeHookCommand`·`codexHookCommand`가 `subagent-start`에도 `--host`를 붙이게 한다. 설치 golden을 `-update`로 재생성하고 diff가 SubagentStart 추가만 담는지 확인한다. 설치 완료 출력의 `Codex SessionStart hook:` 문구를 `Codex context hooks:`로 바꾼다. 추적 템플릿 `configs/codex/hooks.json`·`configs/claude/hooks.settings.json`을 새 `HooksConfig("./bin/issueops")` 출력과 같게 갱신한다. child-host smoke 스크립트 `scripts/verify-child-host-smoke.sh:425-446`의 contract 두 개에 `"SubagentStart": f"'{binary}' hook subagent-start --host codex|claude"`를 넣고, `:444`의 `subcommand = "session-start"` 하드코딩을 이벤트→subcommand 매핑(`SessionStart`→`session-start`, `SubagentStart`→`subagent-start`)으로 바꾼다. 그 스크립트 테스트의 픽스처 두 곳에도 SubagentStart 그룹을 쓴다: 가짜 설치기 `fakeInstallScript`(`internal/adapter/hostprobe/child_host_smoke_script_test.go:766-779`)와, 활성화 전 source 설치 상태를 만드는 `writeManagedSurfaceFixture`(같은 파일 `:710-721`)다. 두 픽스처의 Codex `hooks.json`과 Claude `settings.json`에 `"SubagentStart":[{"hooks":[{"type":"command","command":"'<binary>' hook subagent-start --host codex|claude","timeout":5}]}]`를 넣는다. source 픽스처를 빼면 스크립트가 변경 전 검사(`scripts/verify-child-host-smoke.sh:863`)와 복원 뒤 검사(`:749`)에서 `current activation does not belong to source-root`로 실패한다. 설치기 주석 `internal/adapter/claude/install_hooks.go:84-86`, `internal/adapter/codex/install_hooks.go:73-75,146-149`의 "SessionStart alone carries the catalog"·"only adds SessionStart" 서술도 고친다. native integration 성공 픽스처(`validation_native_integration_test.go:49,159,172-188,204-208`, `validation_mcp_mermaid_native_wrappers_test.go:108`)에 SubagentStart를 넣고, 오류 문구(`validation_native_integration_contract.go:55`)와 기대값(`:126`)을 `Codex thin context hooks missing issueops context hook surface`로 바꾼다.
  - Must NOT do: Claude matcher를 쓰지 않는다(제외는 훅 안에서 한다). trusted_hash를 쓰지 않는다. worktree에서 임시 HOME으로 native install을 실행하지 않는다(source `bin/issueops`가 교체된다).
  - Recommended Agent: deep; Reason: 두 설치기와 golden, 병합 보존 테스트가 걸린다
  - Parallelization: NO; Wave 1; Blocks T5; Blocked By T3
  - References: `internal/adapter/claude/install_hooks.go:83-124`, `internal/adapter/codex/install_hooks.go:57-103,150-158`, `internal/adapter/claude/install.go:32-33`, `internal/adapter/codex/install.go:32-33`, `configs/codex/hooks.json`, `configs/claude/hooks.settings.json`, `internal/adapter/claude/install_test.go:32,127`, `internal/adapter/codex/install_test.go:114,143`, `internal/adapter/install_contract_matrix_test.go:95`, `internal/adapter/testdata/native_install_contract_matrix.golden.json`, `cmd/issueops/installcli/install_native_output.go:41`, `internal/adapter/verification/probe/nativeintegration/validation_native_integration_contract.go:54-69`, `internal/adapter/verification/probe/nativeintegration/validation_native_integration_test.go:49,126,159`, `internal/adapter/verification/probe/validation_mcp_mermaid_native_wrappers_test.go:108`, `scripts/verify-child-host-smoke.sh:425-446`, `internal/adapter/hostprobe/child_host_smoke_script_test.go:710-721,766-779`
  - Acceptance Criteria: G6, G9, G11
  - QA Scenarios:
    - Happy: Steps: 임시 HOME에서 `./bin/issueops install --dry-run --json`. Expected: Claude settings와 Codex hooks 계획에 `SubagentStart`와 `hook subagent-start --host <host>`가 있다.
    - Failure/edge: Steps: 다른 도구의 SubagentStart 그룹이 있는 설정으로 병합 테스트를 실행한다. Expected: 그 그룹이 순서와 내용 그대로 남고 issueops 그룹은 하나만 존재한다.
  - Commit: YES; Message: `feat(install): register the SubagentStart catalog hook for Claude and Codex`; Files: 위 References

- [ ] 5. ADR과 문서 반영
  - What to do: `.issueops/adr/2026-10-08-subagent-start-catalog-and-unified-model-text.md`를 쓴다(결정, 근거 수치, 제외한 훅과 근거, Codex 신뢰 승인, 순서가 정해진 롤백 절차와 오류 문자열, 로컬 self-verify가 머지 전에 한 항목 실패하는 이유). `ADR.md` 색인에 추가하고 2026-08-27 항목에 부분 대체 표시를 단다. `CONVENTIONS.md:76`, `conventions/state-policy-and-hooks.md:42-45`, `operations/guides/hosts.md`의 Codex·Claude hook 절을 갱신한다. bootstrap이 모든 레포에 렌더하는 문구(`internal/adapter/projectdocs/project_docs_render_workflow.go:57-59`의 "turns both into a no-op")와 그 출력을 기대하는 테스트를 갱신한다. `README.md:64-65`, `README.en.md:182,215-216,243,254`, `skills/stability-audit/SKILL.md:61`의 "SessionStart 하나" 서술을 고친다. 같은 서술이 있는 `.issueops/conventions/state-policy-and-hooks.md:41`, `.issueops/conventions/go-and-packages.md:156`, `.issueops/operations/guides/skills-and-hosts.md:25`, `.issueops/CAUTIONS.md:29`, `.issueops/cautions/issueops-lifecycle.md:60`, `.issueops/architecture/hexagonal-core.md:53`, 설치 표면 표 `.issueops/architecture/host-integration.md:13-14`·`.issueops/architecture/runtime.md:20-21`·`.issueops/ARCHITECTURE.md:101`, `.issueops/testing/cli-mcp-and-hosts.md:70`("managed event set이 정확히 `SessionStart` 하나")도 SubagentStart를 포함하도록 고친다. ADR과 날짜가 붙은 이력 기록은 고치지 않는다. 이 레포 표준 문서들의 frontmatter `description`을 새 표준 문구로 동기화한다(`EnsureMetaFrontmatter`와 같은 결과).
  - Must NOT do: 문서 본문 구조를 바꾸지 않는다. 표준 표에 없는 문서(`PROJECT_AUDIT.md`, `SUB_AGENT_PATTERNS.md`)의 설명은 바꾸지 않는다.
  - Recommended Agent: quick; Reason: 문서 갱신이다
  - Parallelization: NO; Wave 1; Blocks —; Blocked By T4
  - References: `.issueops/adr/2026-08-27-session-start-owns-compaction-context.md`, `.issueops/ADR.md:68`, `.issueops/CONVENTIONS.md:76`, `.issueops/conventions/state-policy-and-hooks.md:42-45`, `.issueops/operations/guides/hosts.md:40-46,83-85`, `internal/adapter/projectdocs/project_docs_render_workflow.go:57-59`, `README.md:64-65`, `README.en.md:182,215-216,243,254`, `skills/stability-audit/SKILL.md:61`, `.issueops/conventions/state-policy-and-hooks.md:41`, `.issueops/conventions/go-and-packages.md:156`, `.issueops/operations/guides/skills-and-hosts.md:25`, `.issueops/CAUTIONS.md:29`, `.issueops/cautions/issueops-lifecycle.md:60`, `.issueops/architecture/hexagonal-core.md:53`, `.issueops/architecture/host-integration.md:13-14`, `.issueops/architecture/runtime.md:20-21`, `.issueops/ARCHITECTURE.md:101`, `.issueops/testing/cli-mcp-and-hosts.md:70`
  - Acceptance Criteria: G7, G8, G10
  - QA Scenarios:
    - Happy: Steps: `uv run --directory skills/project-docs-optimize python -m scripts.check --root "$PWD" --mode check --json`. Expected: 새 위반 0건(기존 `adr/roadmap.md` 예산 초과 1건은 범위 밖으로 유지).
    - Failure/edge: Steps: `rg -n "SessionStart 하나만|SessionStart\` 하나만" .issueops/CONVENTIONS.md .issueops/conventions`. Expected: 결과 없음.
  - Commit: YES; Message: `docs(issueops): record the SubagentStart catalog hook decision`; Files: 위 References

## 게이트

- G1 CHECK: `go test ./internal/adapter/projectdoc/... ./internal/adapter/hostprotocol/... ./internal/application/hookprompt/... -count=1` EXPECT: exit 0
- G2 CHECK: `printf '{"cwd":"%s","source":"startup"}' "$PWD" | ./bin/issueops hook session-start --host codex | jq -r .hookSpecificOutput.additionalContext > /tmp/g2c && printf '{"cwd":"%s","source":"startup"}' "$PWD" | ./bin/issueops hook session-start --host claude | jq -r .hookSpecificOutput.additionalContext | cmp - /tmp/g2c && grep -q '^- .issueops/ADR.md: ' /tmp/g2c` EXPECT: exit 0
- G3 CHECK: `go test ./internal/adapter/hookprompt/... ./cmd/issueops/issueopsapp/... -count=1` EXPECT: exit 0
- G4 CHECK: `go test ./internal/domain/projectdoc/... ./cmd/issueops/hookcli/... -count=1` EXPECT: exit 0
- G5 CHECK: `printf '{"cwd":"%s","source":"startup"}' "$PWD" | ./bin/issueops hook session-start --host claude | jq -r .hookSpecificOutput.additionalContext > /tmp/g5s && printf '{"cwd":"%s","agent_type":"general-purpose"}' "$PWD" | ./bin/issueops hook subagent-start --host claude > /tmp/g5a && jq -e '.hookSpecificOutput.hookEventName == "SubagentStart" and (has("systemMessage") | not) and (.hookSpecificOutput.additionalContext | length > 0)' /tmp/g5a && jq -r .hookSpecificOutput.additionalContext /tmp/g5a | cmp - /tmp/g5s` EXPECT: exit 0
- G5b CHECK: `for t in Explore explorer FORK; do printf '{"cwd":"%s","agent_type":"%s"}' "$PWD" "$t" | ./bin/issueops hook subagent-start --host claude | jq -e '. == {}' || exit 1; done` EXPECT: exit 0
- G6 CHECK: `go test ./internal/adapter/claude/... ./internal/adapter/codex/... ./internal/adapter/ ./internal/adapter/verification/... -count=1` EXPECT: exit 0
- G9 CHECK: `jq -e '.hooks.SubagentStart[0].hooks[0].command | test("hook subagent-start --host codex$")' configs/codex/hooks.json && jq -e '.hooks.SubagentStart[0].hooks[0].command | test("hook subagent-start --host claude$")' configs/claude/hooks.settings.json` EXPECT: exit 0
- G10 CHECK: `! rg -n 'session-start\|post-compact \[|둘뿐이다|turns both into a no-op|register only .SessionStart|Hooks provide only .SessionStart|owns exactly .SessionStart|SessionStart. 하나|SessionStart.만 등록|SessionStart. context hook만|catalog 주입뿐|only adds SessionStart|SessionStart alone carries|context-only \(.SessionStart. project-doc catalog\)|정확히 .SessionStart. 하나' internal cmd configs .issueops/CONVENTIONS.md .issueops/conventions .issueops/operations .issueops/CAUTIONS.md .issueops/cautions/issueops-lifecycle.md .issueops/architecture .issueops/ARCHITECTURE.md .issueops/testing scripts README.md README.en.md skills` EXPECT: exit 0
- G11 CHECK: `test "$(rg -c 'hook subagent-start --host (codex|claude)\"' scripts/verify-child-host-smoke.sh)" = 2 && ! rg -n 'subcommand = "session-start"' scripts/verify-child-host-smoke.sh && go test ./internal/adapter/hostprobe/ -run ChildHostSmoke -count=1` EXPECT: exit 0
- G7 CHECK: `go test ./... -count=1` EXPECT: exit 0
- G8 CHECK: `uv run --directory skills/project-docs-optimize python -m scripts.check --root "$PWD" --mode report --json | jq -e '[.violations[] | select(.path != ".issueops/adr/roadmap.md")] | length == 0'` EXPECT: exit 0

G2·G5·G5b는 worktree에서 `go build -o ./bin/issueops ./cmd/issueops`로 만든 바이너리를 쓴다. 이 바이너리를 영구 hook target으로 설치하지 않는다.

로컬 `./bin/issueops self-verify --base-ref "$BASE_SHA" …`는 보조 증거로 실행한다. 실패 항목이 native integration의 `Codex thin context hooks missing issueops context hook surface` 하나뿐이고 원인이 사용자 설치에 SubagentStart가 없기 때문임을 기록한다. 그 밖의 실패가 있으면 결함으로 처리한다. 권위 있는 self-verify 증거는 아래 push 뒤 CI 확인이다.

## push 뒤 CI 확인 (게이트 원장 밖)

8단계 push 뒤, `execution complete` 전에 다음을 확인한다. 이 확인은 verify 단계 시점에 커밋과 push가 없어 원장 게이트로 둘 수 없으므로 원장에 넣지 않는다.

```bash
gh run list --branch 555-session-catalog-subagent-start --workflow CI --limit 1 --json headSha,conclusion,url \
  | jq -e --arg h "$(git rev-parse HEAD)" '.[0].headSha == $h and .[0].conclusion == "success"'
```

실행 중이면 끝날 때까지 기다린다. 성공하면 run URL을 PR 본문의 검증 절과 verified-execution report에 남긴다. 실패하면 publication을 멈추고 implement로 돌아가 고친 뒤 다시 정리·검증·push한다. 이 CI 실행에는 임시 HOME self-verify 단계(`.github/workflows/ci.yml:129-138`)가 포함된다.

## Final Verification Wave

- F1 Plan Compliance Audit: T1–T5가 계획대로 수행됐는지 확인한다.
- F2 Code Quality Review: AI slop 정리 단계에서 dead code와 과한 추상화를 제거한다.
- F3 Real Manual QA: G2, G5와 `install --dry-run` 결과를 증거로 남긴다.
- F4 Scope Fidelity Check: Must NOT Have 항목이 diff에 없는지 확인한다.

## Commit Strategy

TODO마다 커밋 하나씩, Conventional Commit subject와 Lore body(`COMMIT_POLICY.md`)를 쓴다. 커밋과 push는 `atomic-commit-push` 단계가 맡는다.

## Success Criteria

이슈 #555의 완료 기준 여섯 개와 G1–G11(G5b 포함)이 모두 통과하고, push 뒤 CI 확인 절차가 성공한다.
