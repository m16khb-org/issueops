# Research: issueops와 비슷한 역할을 하는 오픈소스

## TL;DR

**결론**: issueops의 네 핵심 축을 모두 갖춘 오픈소스는 찾지 못했습니다. 네 축은 같은 PR에 담기는 팀 기록(F1), 이슈 번호에 묶인 사이클(F3), 해시에 묶인 fail-closed 게이트(F4), 내구성 있는 단일 실행 점유(F5)입니다. 축 하나나 둘을 issueops보다 깊게 다루는 프로젝트는 여럿 있습니다. 이슈 사이클과 점유는 Archon과 gh-aw, 팀 기록은 Kage·OwnMem·Agents Remember, 다중 호스트 배포는 rulesync와 `vercel-labs/skills`, 공용 로컬 MCP 서비스는 mcp_agent_mail_rust가 가장 가깝습니다. 네 축 모두에 부분적으로 대응하는 사례는 `plaplanche/ariane` 하나였습니다(별 1개, 2026-10 생성).
**Confidence**: Medium. 프로젝트별 사실은 1차 출처(README·소스)를 두 번 따로 읽어 확인했습니다. 다만 "결합한 프로젝트가 없다"는 부재 주장이고, 키워드 검색의 한계가 있습니다.
**Sources**: 저장소 약 70개와 공식 문서 페이지 8개를 읽었습니다. 핵심 주장 10개 중 반박 검토에서 그대로 성립 6개, 약해짐 4개, 반박 0개입니다.

## Method

- **조사 관점(2026-11-03, 병렬 4개)**
  1. 이슈 → 계획 → 구현 → PR 사이클 도구
  2. 스펙 중심·방법론 프레임워크
  3. 코딩 에이전트용 팀 메모리·지식 저장
  4. 다중 호스트 하네스 인프라: 규칙·스킬·MCP 배포, 오케스트레이션, 격리, 명령 정책
- **비교 기준**: 저장소 `README.md`, `AGENTS.md`, `.issueops/architecture/issueops.md`에서 뽑은 issueops 기능 F1~F10을 기준으로 썼습니다(아래 표).
- **읽은 방식**: 각 저장소의 README와 문서를 읽었습니다. "없음" 주장이 중요한 곳은 얕은 클론이나 tarball을 grep했습니다. 메타데이터는 인증된 `gh api repos/<o>/<r>`와 `releases/latest`로 얻었습니다. 비인증 GitHub API는 중간에 403(rate limit)을 돌려주어 인증 호출로 바꿨습니다.
- **"없음"의 두 단계**
  - `README-absent`: README와 문서에서 찾지 못했다는 뜻입니다.
  - `source-absent`: 명시한 소스 파일이나 트리를 grep해서 찾지 못했다는 뜻입니다.
- **반박 검토**: 조사 에이전트와 다른 새 에이전트가 핵심 주장 10개(C1~C10)를 원문으로 다시 읽고 반박을 시도했습니다. 같은 1차 출처를 두 번 읽은 것이므로 독립 출처 두 개로 세지 않습니다. 이 보고서는 이 상태를 `재확인`으로 표기합니다.
- **접근 제한**: 로그인, 유료 장벽, CAPTCHA는 만나지 않았습니다. `docs.openhands.dev`, `docs.sweep.dev`, `archon.diy`, cmem.ai TeamBrain 페이지는 읽지 않았습니다.

```text
Source fan-out: issue-cycle tools; spec/methodology frameworks; team memory; multi-host harness infra; adversarial counterexample search (gh search, GitHub REST search, web_search)
Source index: GitHub repos (README/source, Medium) + official docs pages (High), all retrieved 2026-11-03
Claim verification: 10 core claims re-verified by a separate reader: 6 survive, 4 weakened, 0 refuted
Access boundary: no auth/paywall/challenge; unauthenticated GitHub API rate-limited (switched to gh api)
```

### issueops 비교 기준

|ID|issueops 기능|
|---|---|
|F1|에이전트가 caution·ADR을 기록 한 건당 파일 하나로 남기고, 코드와 같은 PR에서 사람이 리뷰합니다|
|F2|`route-docs`가 작업 설명과 단어가 겹치는 기록을 최대 다섯 건 고르고, 고른 이유를 돌려줍니다|
|F3|이슈 하나에 묶인 이슈 → 계획 → 구현 → 검증 → PR/MR → 정리 사이클이 있고, GitHub와 GitLab을 지원합니다|
|F4|CLI 게이트가 fail-closed로 동작하고, 승인을 문서 sha256이나 변경 집합 fingerprint에 묶습니다|
|F5|기록 하나에 실행 lease 하나를 둡니다. generation, claim token, 프로세스 영수증, CAS를 쓰고, 원격 생성 결과가 불분명하면 자동 재시도 없이 reconcile합니다|
|F6|Codex·Claude Code·Omo·omp·opencode에 같은 스킬을 사용자 홈 심볼릭 링크로 배포하고, 대상 저장소에는 기본적으로 쓰지 않습니다|
|F7|Go 단일 바이너리이고, 사용자당 하나인 로컬 Streamable HTTP MCP 서비스와 `authorize`가 발급하는 caller capability를 씁니다|
|F8|allowlist·denylist 명령 정책, workspace override, timeout, env redaction, JSONL 감사 로그가 있습니다|
|F9|hook은 문맥만 넣고, 강제는 CLI 게이트가 맡습니다|
|F10|스킬과 템플릿이 한국어 우선입니다|

## Findings

### 비교표

●는 있음, ◐는 부분적이거나 방식이 다름, ○는 읽은 범위에서 찾지 못함을 뜻합니다. 근거는 아래 항목별 설명에 있습니다.

|프로젝트|F1 팀 기록|F2 작업별 조회|F3 이슈 사이클|F4 해시 게이트|F5 점유·CAS|F6 다중 호스트|F7 공용 MCP|F8 명령 정책|
|---|---|---|---|---|---|---|---|---|
|**issueops**|●|●|●|●|●|●|●|●|
|Archon|○|○|●|●|●|◐|○|○|
|gh-aw|◐|○|◐|◐|●|◐|○|◐|
|OpenHands extensions|◐|○|●|○|◐|◐|○|◐|
|CCPM|◐|○|●|○|○|◐|○|○|
|Ariane|◐|○|◐|◐|◐|○|○|○|
|spec-kitty|◐|◐|◐|◐|◐|●|○|○|
|spec-kit|◐|○|◐|◐|○|●|◐|○|
|OpenSpec|◐|○|○|◐|◐|●|○|○|
|Backlog.md|◐|◐|◐|○|◐|◐|◐|○|
|superpowers|◐|○|○|◐|○|●|○|○|
|Kage|●|◐|◐|◐|○|●|○|○|
|OwnMem|●|●|○|◐|○|◐|◐|○|
|Agents Remember|●|◐|◐|◐|◐|●|◐|○|
|Keep the Why|●|◐|○|◐|○|●|○|○|
|mcp_agent_mail_rust|○|○|○|○|◐|●|●|○|
|rulesync|○|○|○|○|○|●|○|◐|
|vercel-labs/skills|○|○|○|○|○|●|○|○|
|container-use|○|○|○|○|◐|◐|◐|◐|
|tumf/mcp-shell-server|○|○|○|○|○|○|◐|●|

### 1. 네 축을 모두 갖춘 프로젝트는 찾지 못했습니다

- **주장**: F1, F3, F4, F5를 모두 갖춘 오픈소스는 발견되지 않았습니다. 가장 가까운 사례는 `plaplanche/ariane`이고, 네 축 모두 부분 대응입니다.
- **근거**
  - Ariane(AGPL-3.0, 별 1, 2026-10-06 생성)은 라벨이 붙은 GitHub 이슈를 `ariane/<n>` 브랜치와 worktree에서 실행해 PR까지 보냅니다. `ariane.example.toml`에 "only \"github\" exists today"라고 적혀 있습니다.
  - Ariane의 `docs/adr/0011-learnings-store.md`는 사람이 수락한 학습을 `CLAUDE.md`와 `docs/learnings.md`에 한 커밋으로 넣어 "the single push carries both"라고 설명합니다. 다만 기록 한 건당 파일 하나가 아니라 단일 파일에 쌓고, ADR과 caution을 구분하지 않습니다.
  - 같은 ADR에는 리뷰한 커밋 이후에 규칙 파일, 사실 파일, `work/<n>/` 밖이 바뀌면 "the review is stale"로 멈춘다는 규칙이 있습니다. 승인이 문서 sha256이 아니라 커밋 SHA에 묶입니다.
  - Ariane의 점유는 이슈별 파일 락(`restart.py`의 `locks/<n>.lock`)과 `git push --force-with-lease`뿐입니다. 내구성 있는 lease와 generation fence는 없습니다.
  - 반박 에이전트는 GitHub REST 검색 20개 이상, `gh search repos`, `web_search` 2건을 실행했습니다. 그 결과에서 후보 33개를 클론해 F1·F3·F4·F5 지표어로 grep했습니다.
  - `issueops`라는 이름의 직접 대응물은 없습니다. `gh search repos issueops` 결과인 `github/command`, `github/branch-deploy`, `issue-ops/*`는 이슈를 운영 UI로 쓰는 ChatOps 개념입니다.
- **검증**: 재확인(반박 검토에서 "약해짐"). "없다"가 아니라 "발견하지 못했다"로 써야 합니다.
- **Confidence**: Medium

### 2. 이슈 사이클 도구: Archon과 gh-aw가 점유 설계에서 가장 가깝습니다

- **Archon** (`coleam00/Archon`, MIT, TypeScript, 별 23,810, v0.13.2 2026-10-29)
  - README는 sdlc 팩을 "A software factory: issue in, merged PR out"으로 설명합니다. 실행마다 git worktree를 따로 씁니다.
  - F5: `.archon/workflows/sdlc/.shared/workitem.ts`에 "a run then claims the best waiting item with a conditional write, so two runs never work the same item"이 있습니다. 조건부 쓰기 `statePut(name, value, ifRevision)`은 `.shared/state.ts`에 있고, 충돌하면 `conflict`를 돌려줍니다.
  - F4: `merge-queue/scripts/gate.ts`는 "the trusted review verdict says Ready at `head`, CI is green at `head`"가 아니면 병합을 거부합니다. 병합도 `head`에 고정하고 병합 뒤 다시 읽어 확인합니다.
  - F3 provider: `sdlc/.shared/forge.ts`에 따르면 기본값은 `gh`이고, `ARCHON_SDLC_FORGE=forge`로 forge 플러그인을 고를 수 있습니다. 번들 플러그인은 GitHub용 하나만 확인했습니다. `community/forge/gitlab/adapter.ts`는 @mention과 webhook을 받는 채팅 어댑터이고 sdlc 팩과는 별개입니다.
  - 차이: sdlc 팩 전체에서 ADR, caution, 교훈 기록은 grep 결과 0건이었습니다(F1 없음). generation fence, claim token, 프로세스 영수증은 source-absent입니다.
  - 검증: 재확인(C1, 파일 경로와 GitLab 설명은 보정). Confidence: Medium.
- **gh-aw** (`github/gh-aw`, MIT, Go, 별 5,568, v1.0.1 2026-10-27)
  - Markdown 워크플로를 GitHub Actions `.lock.yml`로 컴파일하는 `gh` 확장입니다. Copilot, Claude, Codex, Gemini, Pi 엔진을 샌드박스 job에서 실행합니다.
  - F5: `docs/src/content/docs/reference/work-queue.md`에 "A launch sends at most one dispatch POST: an uncertain response does not authorize another launch."와 "Unknown outcomes must be inspected before retrying."이 있습니다. issueops의 "불분명하면 자동 재시도 없이 reconcile" 원칙과 같은 방향입니다.
  - F4: 원본 frontmatter와 `.lock.yml`이 어긋나면 `gh aw list`가 `COMPILED: No`로 표시하고, CI에서 실패시킬 수 있습니다.
  - 차이: 이슈 번호에 묶인 계획 → 구현 → PR 사이클이 아니라 트리거와 자동화 프레임워크입니다. GitLab 지원은 source-absent입니다. `repo-memory`는 `memory/default` 브랜치에 저장하므로 기능 PR에서 리뷰받지 않습니다.
  - 검증: 재확인(C2). Confidence: Medium.
- **OpenHands extensions** (`OpenHands/extensions`, MIT, Python)
  - 예전 OpenHands issue resolver가 `skills/github-issue-to-pr`, `gitlab-issue-to-mr`, `jira-issue-to-pr` 스킬로 옮겨 왔습니다. 결정적인 Python 스크립트가 clone, commit, push, PR 생성을 맡고, LLM은 코드만 씁니다.
  - F3: GitLab MR까지 지원하는 이슈 → PR/MR 흐름은 비교 대상 가운데 이것만 확인했습니다. 브랜치는 `openhands/issue-<n>`이고 PR 제목에 `[#42]`, 본문에 `Closes #42`를 넣습니다.
  - F5: `references/state-schema.md`는 라벨 이벤트 ID를 idempotency key로 쓰고, 느린 작업 전에 `starting` claim을 저장합니다. 반대로 `scripts/main.py`는 PR 생성 422 응답을 "already exists"로 보고 채택하고, push나 PR 실패를 다음 두 번의 poll에서 재시도합니다. issueops의 원칙과 반대입니다.
  - `OpenHands/openhands-github-action`은 2025-10-08에 archived 상태가 되었습니다.
  - 검증: 1차 출처 단일 확인. Confidence: Medium.
- **CCPM** (`automazeio/ccpm`, MIT, Shell, 별 8,425, 릴리스 없음)
  - PRD → Epic → Task → GitHub Issue → worktree → 병렬 에이전트 흐름의 Agent Skill입니다. 동기화 뒤 작업 파일 `001.md`의 이름을 이슈 번호 `{issue-id}.md`로 바꿉니다.
  - 차이: `skill/ccpm/references/scripts/*.sh` 13개와 참조 문서 전체에서 `flock`, `lock`, `lease`, `claim`, `mutex`가 0건이었습니다. "in-progress"는 `gh issue edit --add-assignee @me --add-label "in-progress"`로 표시하는 소프트 할당입니다. `glab` 지원은 source-absent이고, 해시에 묶인 승인도 없습니다.
  - 검증: 재확인(C5). Confidence: Medium.
- **나머지**
  - `openai/symphony`(별 27.9k)는 사양 문서입니다. §14.3에서 scheduler 상태를 "intentionally in-memory"로 둡니다. tracker는 Linear입니다.
  - `anthropics/claude-code-action`의 `agent-approval-check`는 에이전트 커밋이 있는 PR에서 현재 head SHA와 일치하는 승인만 세는 fail-closed 검사입니다. 점유 기능은 source-absent입니다.
  - SWE-agent는 README에서 개발 중심이 mini-swe-agent로 옮겨 갔다고 밝힙니다. GitHub에만 PR을 엽니다.
  - Sweep은 유료 구독이 필요한 EE 라이선스이고, 팀이 JetBrains 도우미로 방향을 바꿨습니다.
  - opencode는 GitLab 이슈에서 MR을 여는 CI 흐름이 있지만, 브랜치가 `opencode/${CI_JOB_ID}`라 이슈 번호에 묶이지 않습니다.

### 3. 스펙·방법론 프레임워크: 대부분 게이트를 프롬프트로만 강제합니다

- **주장**: 필수 비교 대상 일곱 개 중 리뷰나 승인을 문서 해시나 diff fingerprint에 묶는 프로젝트는 없었습니다. 코드로 강제하는 범위가 넓은 순서는 spec-kitty, OpenSpec, GSD Core입니다.
- **spec-kitty** (`spec-kitty/spec-kitty`, MIT, Python, 별 1,712)
  - `spec → plan → tasks → next → review → accept → merge` 순서의 CLI 흐름입니다. lane마다 git worktree를 쓰고, 상태 로그는 추가만 합니다.
  - 권한 규칙은 `status/wp_state.py`에 있고, approved·done 전이에는 reviewer와 승인 참조가 필요합니다. `lanes/for_review_gate.py`는 lane 브랜치에 base 이후 구현 커밋이 없으면 `for_review` 전이를 막습니다. `src/charter/hasher.py`는 charter 내용을 SHA-256으로 해시해 낡은 charter를 찾아냅니다.
  - 차이: 승인 가드는 reviewer와 verdict만 확인하고 sha, hash, fingerprint는 보지 않습니다. `DM-*.md` 결정 원장이 있지만 mission accept 시점에 커밋되고, 코드와 같은 PR에서 리뷰한다는 설명은 없습니다. 미션이 이슈 번호에 묶이지 않습니다.
  - 지원 호스트: 12개 slash command 호스트는 사용자 홈에, 5개 스킬 호스트는 저장소 `.agents/skills/`에 설치합니다. omp는 목록에 없습니다.
  - 검증: 재확인(C7). Confidence: Medium.
- **github/spec-kit** (MIT, Python, 별 145,598, v1.2.6 2026-11-02)
  - 40개가 넘는 통합 키가 있고 `omp`와 `opencode`도 포함합니다. `specify init --integration <key>`가 파일을 저장소 안에 씁니다.
  - `src/specify_cli/extensions/__init__.py`의 `execute_hook` 주석은 "The actual execution is delegated to the AI agent."입니다. `optional: false` hook도 `EXECUTE_COMMAND` 텍스트만 출력합니다.
  - workflow `gate` 단계(`workflows/step/gate/__init__.py`)는 실제 코드입니다. 다만 선택값만 저장하고, 검토한 파일을 해시하지 않습니다.
  - MCP 서버는 "Experimental stdio-only MCP adapter"입니다.
  - 검증: 재확인(C8). Confidence: Medium.
- **Fission-AI/OpenSpec** (MIT, TypeScript, 별 73,731)
  - `src/core/archive.ts`는 스펙 검증 실패나 미완료 작업이 있으면 `ArchiveBlockedError`를 던집니다. 확인을 기다리는 동안 입력 디렉터리의 SHA-256 digest를 다시 계산하고, 바뀌었으면 실패합니다.
  - 차이: `docs/concepts.md`는 "no phase gates"라고 밝히고, `docs/team-workflow.md`는 "OpenSpec never touches git"이라고 밝힙니다.
- **Backlog.md** (`MrLesk/Backlog.md`, MIT, 별 7,250)
  - 결정을 `backlog/decisions/` 아래 파일로 남기고 Context, Decision, Rationale 섹션을 둡니다. `MANIFESTO.md`에는 "Fail closed when identity is ambiguous" 원칙이 있어 issueops와 철학이 가깝습니다.
  - 차이: 계획 승인은 사람이 검토하는 지점이고, `src/`에서 "approv" grep 결과가 0건이었습니다. `gh`와 `glab` 연동은 source-absent입니다.
- **GSD Core** (`open-gsd/gsd-core`, MIT, 별 11,178)
  - 이전 저장소 `gsd-build/get-shit-done`(별 64,169)은 archived 상태입니다.
  - PreToolUse hook이 `allow`·`deny`·`crash` 중 하나로 끝나고, hook마다 `HOOK_ON_CRASH`를 선언합니다. issueops와 반대로 hook에서 강제합니다(F9).
- **superpowers** (`obra/superpowers`, MIT, 별 311,276, v7.0.1)
  - README는 16개 호스트를 열거하고 "If you use more than one, install Superpowers separately for each one."이라고 안내합니다.
  - `hooks/hooks.json`에는 `SessionStart`만 있고, 부트스트랩 스킬 문맥만 넣습니다. issueops의 F9와 같은 방식입니다.
  - 게이트는 `<HARD-GATE>` 프롬프트와 `skills/executing-plans/scripts/task-done` 스크립트입니다. 이 스크립트는 테스트가 통과할 때만 원장에 줄을 추가합니다.
  - omp 지원은 source-absent입니다.
- **나머지**
  - BMAD-METHOD(별 54,909)의 `main`은 v7 preview alpha입니다. 리뷰 게이트는 lint 스크립트 뒤에 서브에이전트 리뷰를 붙이는 방식이고, 위험이 낮으면 건너뛸 수 있습니다.
  - cc-sdd의 `spec.json` 승인 플래그는 스킬이 읽고 지키는 값입니다. 코드는 이 값을 강제하지 않습니다.
  - claude-task-master는 MIT + Commons Clause 라이선스이고, 마지막 커밋 이후 약 6개월 동안 새 커밋이 없습니다.
  - Agent OS는 v3에서 스펙 작성과 작업 분해를 plan mode로 넘겼습니다.
  - adrkit은 `adr accept --by @user`에서 quorum을 검사하고, ADR 전체 corpus의 SHA-256 fingerprint를 계산합니다.

### 4. 팀 메모리: 저장소에 두고 PR에서 리뷰하는 도구는 여럿 있습니다

- **주장**: 기록을 저장소에 두고 코드 PR에서 리뷰하는 방식(F1)은 Kage, OwnMem, Keep the Why, Agents Remember, MEX가 지원합니다. 그러나 caution이나 ADR 판정이 없다는 이유로 계획이나 PR을 막는 프로젝트는 찾지 못했습니다.
- **Kage** (`kage-core/Kage`, GPL-3.0, 별 42)
  - README 비교표에 "Memory reviewed in git, same PR as the code (plain files, no DB)"가 있습니다.
  - `kage pr check`는 "stale-catch + graph freshness gate"입니다. 기록이 인용한 파일이 바뀌거나 지워지면 그 기록을 조회 결과에서 뺍니다.
  - `docs/DELEGATION.md`에는 "Leases. Concurrent runs that touch the same files are not yet serialized."라고 적혀 있습니다.
  - 이전 릴리스는 MIT였고 지금은 GPL-3.0-only입니다.
  - 검증: 재확인(C3). Confidence: Medium.
- **OwnMem** (`grpcer/ownmem`, Apache-2.0, 별 425)
  - `.ownmem/*.md`를 커밋하고 "reviewed like code"로 다룹니다.
  - README의 `recall` 출력 예는 `score=0.875 lanes=exact,bm25f,ngram`과 `matched deploy,more,out,staging,...`입니다. 고른 이유를 함께 돌려주는 점이 `route-docs`와 가장 가깝습니다. 순위는 단어 겹침이 아니라 BM25F와 n-gram으로 매깁니다.
  - `ownmem audit`는 기록 corpus가 quota를 넘으면 실패합니다. 기록을 남겨야 한다는 강제는 없습니다.
  - 검증: 재확인(C9). Confidence: Medium.
- **Agents Remember** (`Foxfire1st/agents-remember`, MIT, Python, 별 39)
  - `docs/features.md`: "Start is ledger-gated. … If that commit is unmapped, start blocks."
  - 마무리 순서는 소스 품질 게이트 → 코드 커밋 → 온보딩 스탬프 → "blocking memory-quality gate" → 메모리·원장 커밋입니다. 소스 품질 게이트가 실패하면 코드·메모리·원장이 모두 커밋되지 않습니다. 메모리 품질 게이트가 실패하면 코드는 이미 커밋되어 있고 메모리 커밋만 막힙니다.
  - commit, push, PR, merge, cleanup이 각각 별도 승인 게이트입니다. 이슈와 MR 번호에 묶는다는 설명은 읽은 범위에 없습니다.
  - 검증: 재확인(C4, 두 게이트를 하나로 합친 원래 서술을 보정). Confidence: Medium.
- **Keep the Why** (`oliver-zehentleitner/keep-the-why`, MIT, 별 233)
  - README: "a `context/` update ships in the same commit or PR as the code change it explains".
  - `keep-the-why-lint`가 CI에서 구조를 검사합니다. 기록 작성은 에이전트가 스스로 판단합니다.
  - 자체 실험 결과를 싣고 있습니다. 기각 이유가 없으면 20세션 중 10세션에서 7번 기각된 단순화를 다시 제안했고, 기록 한 건이 있으면 0번 제안했습니다.
- **MEX** (`mex-memory/mex`, MIT, 별 1,789): git으로 공유하는 `.mex/` Markdown과 로컬 FTS 검색을 씁니다. README는 "Drift is a review signal"이라고 밝히고, 기록 작성을 강제하지 않습니다.
- **저장소 밖에 기록을 두는 도구**
  - claude-mem(별 110,801)은 `~/.claude-mem/` SQLite와 Chroma를 씁니다.
  - mem0는 Mem0 Platform API 키가 필요합니다.
  - ByteRover(별 4,948)는 `.brv/`를 `.gitignore`에 넣고 클라우드로 공유합니다. 저장소는 archived 상태입니다.
  - basic-memory는 기본 위치가 `~/basic-memory`입니다.
  - 이 도구들은 PR 리뷰 흐름 밖에 있습니다.
- **Cline Memory Bank**: 공식 문서에 "I MUST read ALL memory bank files at the start of EVERY task"라고 적혀 있습니다. 작업별로 기록을 고르지 않습니다.
- **ADR 도구**: log4brains는 2024-12 이후 활동이 없습니다. adr-tools의 마지막 릴리스는 2018년입니다. 둘 다 에이전트 연동이 없습니다.

### 5. 하네스 인프라: 배포, 공용 MCP, 명령 정책을 각각 따로 다룹니다

- **mcp_agent_mail_rust** (`Dicklesworthstone/mcp_agent_mail_rust`, 별 230, v0.3.40 2026-11-01)
  - 공용 로컬 HTTP MCP 서비스라는 점에서 F7에 가장 가깝습니다. README에 "Server starts on 127.0.0.1:8765"가 있고, `HTTP_BEARER_TOKEN`과 `Authorization: Bearer`를 설명합니다.
  - 설치기가 Claude Code, Codex CLI, Gemini CLI, Oh My Pi, Copilot CLI를 감지해 HTTP MCP로 연결합니다.
  - 차이: 베어러 인증은 선택입니다("an unset token leaves HTTP authentication off for trusted-local operation"). 파일 예약은 "advisory leases, not hard locks"이고, 에이전트가 무시할 수 있습니다.
  - 라이선스는 "MIT License (with OpenAI/Anthropic Rider)"이고, GitHub API는 NOASSERTION으로 표시합니다. Python판 `mcp_agent_mail`은 설명란이 "Deprecated: superseded by mcp_agent_mail_rust"입니다.
  - 검증: 재확인(C6, 인증이 선택이라는 점을 보정). Confidence: Medium.
- **스킬·규칙 배포**
  - `vercel-labs/skills`(별 37,816)는 issueops와 같은 방식입니다. `docs/commands.md`는 "Symlink (Recommended) — Creates symlinks from each agent to a canonical copy"라고 설명하고, `-g`로 사용자 홈에 설치합니다. 기본으로 telemetry를 보내고, `DISABLE_TELEMETRY=1`로 끕니다. omp 항목은 찾지 못했습니다.
  - rulesync(별 1,578)는 규칙, MCP, 스킬, hook, 권한 설정을 약 60개 도구에 생성합니다. `--global` 범위를 지원하고 oh-my-pi도 목록에 있습니다. 출력은 심볼릭 링크가 아니라 복사본입니다.
  - Ruler(별 2,956)는 README에 "Ruler never writes MCP configuration files outside your project root"라고 적혀 있습니다. issueops와 반대로 저장소 안에 씁니다.
- **명령 정책과 격리**
  - `tumf/mcp-shell-server`는 `ALLOW_COMMANDS`, 인자 강화 규칙(`find -exec`, `git rebase -x` 등 차단), 30초 timeout, 최소 환경 변수, `shell_execution_audit` 이벤트를 제공합니다. 설정은 환경 변수뿐이고, README는 "This package is not a sandbox"라고 밝힙니다.
  - `anthropic-experimental/sandbox-runtime`은 macOS seatbelt와 Linux bubblewrap으로 파일시스템과 네트워크를 OS 수준에서 막습니다. 명령 정책보다 아래 층에서 동작합니다.
  - container-use(별 4,082)는 에이전트마다 Dagger 컨테이너와 git 브랜치를 줍니다. `repository/flock.go`의 프로세스 간 락은 git 작업 충돌만 막고, lease는 없습니다. MCP는 stdio만 지원합니다.
- **hook으로 강제하는 사례(F9와 반대)**: cupcake는 OPA/Rego 정책을 hook 경로에서 평가합니다. 런타임 연동이 없는 호스트는 다루지 못한다고 README가 밝힙니다. agent-coord는 Claude Code `PreToolUse` hook으로 편집을 막고, Codex는 "cooperates voluntarily"라고 적습니다.
- **점유 설계**
  - singular-lite는 `.singular-state/leases/` JSON lease와 pid 생존 확인을 씁니다.
  - dibs는 토큰으로 보호하는 만료 lease와 heartbeat를 씁니다.
  - `iradulovic/software-factory`는 PostgreSQL 기반 claim과 lease를 씁니다.
  - generation fence와 프로세스 영수증(pid, 시작 시각, 실행 파일)을 함께 쓰는 사례는 찾지 못했습니다.
- **유지 상태**: vibe-kanban은 2026-04-10 "Goodbye bloop" 공지로 회사가 문을 닫았고, 커뮤니티가 유지합니다. crystal은 2026-02에 deprecated 상태가 되었습니다.

### 6. issueops에만 있는 부분과 다른 곳에도 있는 부분

|issueops 특징|다른 프로젝트 사례|판정|
|---|---|---|
|문서 반영 판정(`updated` 또는 `no-change`와 읽은 문서)이 없으면 PR 발행을 막음|기록 작성 자체를 게이트로 강제하는 사례 없음. Agents Remember는 메모리 품질 게이트로 메모리 커밋만 막음|issueops에만 있음|
|계획에 "적용되는 결정과 주의사항" 섹션이 없으면 거부|스펙 프레임워크의 필수 섹션 검사는 대부분 프롬프트로만 강제. spec-kit `check_prerequisites.py`는 파일 존재만 확인|issueops에만 있음|
|승인을 문서 sha256·변경 fingerprint에 묶음|Archon(`head` SHA), claude-code-action `agent-approval-check`(head SHA), Ariane(커밋 SHA), leadkit(커밋 SHA), OpenSpec(archive 입력 digest)|커밋 SHA 바인딩은 흔함. 문서 단위 sha256 승인은 issueops에만 있음|
|원격 생성 결과가 불분명하면 자동 재시도 없이 reconcile|gh-aw work queue가 같은 원칙을 문서화함. OpenHands extensions는 반대로 재시도함|gh-aw와 공유|
|generation fence + claim token + 프로세스 영수증 lease|Archon(revision CAS), singular-lite(pid 생존 확인), dibs(토큰 lease), software-factory(PostgreSQL lease)|조합은 issueops에만 있음|
|사용자당 공용 HTTP MCP + caller capability 파일|mcp_agent_mail_rust(베어러, 선택), scoped-mcp(에이전트마다 proxy 프로세스), hermes-mcp-gateway(OAuth, Hermes 전용)|부분 사례 있음|
|hook은 문맥만 넣음|superpowers, OwnMem은 같음. GSD Core, cupcake, agent-coord는 hook으로 강제|방식이 같은 사례 있음|
|GitHub와 GitLab을 같은 사이클에서 지원|OpenHands extensions(GitLab MR), opencode(GitLab CI), Archon(forge 플러그인, 옵트인)|부분 사례 있음|
|한국어 우선 스킬|번역 README(spec-kit, OwnMem, Kage)는 있으나 한국어 우선 설계는 없음|issueops에만 있음|

## Cross-Check Results

|상태|개수|뜻|
|---|---|---|
|재확인 후 성립|6|C2 gh-aw, C3 Kage, C5 CCPM, C7 spec-kitty, C8 spec-kit, C9 OwnMem|
|재확인 후 약해짐|4|C1 Archon(파일 경로, GitLab 설명), C4 Agents Remember(게이트 순서), C6 mcp_agent_mail(인증 선택), C10 핵심 결론(Ariane 부분 근접)|
|반박됨|0|없음|
|1차 출처 단일 확인|나머지 프로젝트별 사실|조사 에이전트 한 명만 읽은 README·소스|
|Disputed|0|독립 출처끼리 충돌한 주장 없음|

프로젝트별 사실의 출처는 해당 프로젝트의 README와 소스 하나뿐입니다. 그래서 반박 검토를 통과해도 Confidence를 High로 올리지 않았습니다.

## Adversarial Review

- **검토자**: 조사에 참여하지 않은 새 에이전트 한 명이 2026-11-03에 `/tmp/refute/*` 얕은 클론과 `gh api`로 원문을 다시 읽었습니다.
- **C1 Archon**: 점유와 병합 게이트는 성립했습니다. `ifRevision`은 `workitem.ts`가 아니라 `state.ts`에 있습니다. "sdlc 팩은 `gh`만 씀"은 틀렸고, forge 플러그인을 옵트인으로 고를 수 있습니다. 이 보고서는 보정한 내용을 반영했습니다.
- **C4 Agents Remember**: 원래 서술은 "메모리 품질 게이트가 실패하면 코드와 메모리가 모두 커밋되지 않는다"였습니다. 실제로 그렇게 동작하는 것은 소스 품질 게이트입니다. 보정한 내용을 반영했습니다.
- **C6 mcp_agent_mail**: 베어러 인증이 선택 사항이라는 점을 추가했습니다.
- **C10 핵심 결론**: 반례 탐색에서 Ariane을 찾았습니다. 결론을 "어떤 프로젝트도 없다"에서 "발견하지 못했다. Ariane이 부분적으로 근접한다"로 고쳤습니다.
- **메타데이터 보정**: Kage는 GPL-3.0입니다(이전 릴리스는 MIT). Python판 mcp_agent_mail은 Deprecated 상태입니다.

## Disputed / Unresolved

- **지원 호스트 수**: spec-kitty `docs/api/supported-agents.md`는 같은 페이지에서 17개와 16개를 함께 씁니다. 정확한 수를 정하려면 설치기 소스를 읽어야 합니다.
- **별 수 시점 차이**: 첫 답변에 인용한 검색 요약은 spec-kit 104,718, superpowers 293,553이었습니다. 2026-11-03 API 값은 각각 145,598, 311,276입니다. 이 보고서는 API 값을 씁니다.
- **Archon의 상태 저장 원자성**: `--if-revision`과 `STATE_CONFLICT` 종료 코드까지만 확인했습니다. 저장소 자체의 원자성은 확인하지 않았습니다.

## Open Questions

- Archon과 gh-aw의 F8 명령 정책은 확인하지 않았습니다.
- spec-kitty `dispatch`가 governance 문맥을 어떻게 고르는지는 README에만 근거합니다. route-docs와 비교하려면 소스를 읽어야 합니다.
- `steveyegge/beads`, `steveyegge/gastown`, Nimbalyst(crystal 후속), coordinaut는 읽지 않았습니다.
- 다음 비교 후보 세 가지가 남아 있습니다. 결정은 아니고 조사 질문입니다.
  1. OwnMem의 조회 이유 출력과 quota 방식이 `route-docs`의 단어 겹침보다 정확한지 같은 작업 묶음으로 비교하기
  2. Kage의 "인용 파일이 바뀌면 기록을 빼는" 방식과 README의 "기록이 코드와 어긋나도 자동으로 찾지 않는다"는 한계 비교하기
  3. gh-aw work queue 모델(`specs/work-queue/*Witness.cfg`)과 issueops lease 모델의 불변식 비교하기

## Source Index

모든 항목의 조회일은 2026-11-03입니다. 저장소 항목은 README와, 위 Findings에 경로를 적은 소스 파일을 읽었습니다.

|#|URL|설명|유형|권위|
|---|---|---|---|---|
|1|https://github.com/coleam00/Archon|sdlc 팩, `workitem.ts`, `state.ts`, `gate.ts`, `forge.ts`|README·소스|Medium|
|2|https://github.com/github/gh-aw|`work-queue.md`, `workflow-editing.md`, `repo-memory.md`|README·문서|Medium|
|3|https://github.com/OpenHands/extensions|`github-issue-to-pr`, `gitlab-issue-to-mr`, `state-schema.md`, `main.py`|README·소스|Medium|
|4|https://github.com/OpenHands/automation|`dispatcher.py`, `event_router.py`|README·소스|Medium|
|5|https://github.com/automazeio/ccpm|`skill/ccpm/` 스크립트 13개, 참조 문서|README·소스|Medium|
|6|https://github.com/openai/symphony|`SPEC.md` §7, §11, §14.3|사양|Medium|
|7|https://github.com/anthropics/claude-code-action|`agent-approval-check/README.md`, `docs/security.md`|README·문서|Medium|
|8|https://github.com/openai/codex-action|README, `docs/security.md`|README|Medium|
|9|https://github.com/SWE-agent/SWE-agent|`sweagent/run/hooks/open_pr.py`|README·소스|Medium|
|10|https://github.com/sweepai/sweep|README, LICENSE, `docs/pages/faq.mdx`|README|Medium|
|11|https://opencode.ai/docs/gitlab|opencode GitLab 흐름|공식 문서|High|
|12|https://github.com/plaplanche/ariane|`ariane.example.toml`, ADR 0011·0032, `restart.py`, `git.py`|README·소스|Medium|
|13|https://github.com/MattGrundy/leadkit|커밋 SHA 승인|README|Medium|
|14|https://github.com/iradulovic/software-factory|PostgreSQL claim·lease|README|Medium|
|15|https://github.com/github/spec-kit|`extensions/__init__.py`, `workflows/step/gate/__init__.py`, `docs/integrations/omp.md`|README·소스|Medium|
|16|https://github.com/Fission-AI/OpenSpec|`src/core/archive.ts`, `docs/concepts.md`, `docs/team-workflow.md`|README·소스|Medium|
|17|https://github.com/bmad-code-org/BMAD-METHOD|`reviewer-gate.md`, `bmad-build/workflow.md`|README·소스|Medium|
|18|https://github.com/obra/superpowers|`hooks/hooks.json`, `executing-plans/scripts/task-done`|README·소스|Medium|
|19|https://github.com/buildermethods/agent-os|`inject-standards.md`, `project-install.sh`|README·소스|Medium|
|20|https://github.com/eyaltoledano/claude-task-master|LICENSE, `tdd-workflow/quickstart.mdx`|README·문서|Medium|
|21|https://github.com/MrLesk/Backlog.md|`MANIFESTO.md`, `src/utils/mcp-client-setup.ts`|README·소스|Medium|
|22|https://github.com/spec-kitty/spec-kitty|`status/wp_state.py`, `lanes/for_review_gate.py`, `charter/hasher.py`|README·소스|Medium|
|23|https://github.com/gotalab/cc-sdd|`kiro-impl/SKILL.md`, `spec.json`|README·소스|Medium|
|24|https://github.com/open-gsd/gsd-core|`docs/ARCHITECTURE.md`, 설치 문서|README·문서|Medium|
|25|https://github.com/mbeacom/adrkit|`packages/core/src/fingerprint/index.ts`|README·소스|Medium|
|26|https://github.com/ashbrener/spec-kit-arch-governance|README|README|Low|
|27|https://github.com/kage-core/Kage|README, `docs/DELEGATION.md`|README·문서|Medium|
|28|https://github.com/grpcer/ownmem|README, `docs/TECHNICAL.md`|README·문서|Medium|
|29|https://github.com/oliver-zehentleitner/keep-the-why|README|README|Medium|
|30|https://github.com/Foxfire1st/agents-remember|README, `docs/features.md`|README·문서|Medium|
|31|https://github.com/mex-memory/mex|README|README|Medium|
|32|https://github.com/donatienmigue/TeamBrain|README|README|Low|
|33|https://github.com/blain3white/team-memory|README|README|Low|
|34|https://github.com/sventorben/decider|README|README|Low|
|35|https://docs.cline.bot/features/memory-bank|Cline Memory Bank|공식 문서|High|
|36|https://github.com/campfirein/byterover-cli|README, `src/server/utils/gitignore.ts`|README·소스|Medium|
|37|https://github.com/thedotmack/claude-mem|README|README|Medium|
|38|https://github.com/basicmachines-co/basic-memory|README|README|Medium|
|39|https://github.com/mem0ai/mem0|README|README|Medium|
|40|https://docs.mem0.ai/integrations/claude-code.md|mem0 Claude Code 연동|공식 문서|High|
|41|https://github.com/modelcontextprotocol/servers|`src/memory/README.md`|README|Medium|
|42|https://github.com/thomvaill/log4brains|README|README|Medium|
|43|https://github.com/npryce/adr-tools|README|README|Medium|
|44|https://agents.md/|AGENTS.md 형식|공식 문서|High|
|45|https://code.claude.com/docs/en/memory.md|Claude Code 메모리|공식 문서|High|
|46|https://github.com/intellectronica/ruler|README|README|Medium|
|47|https://github.com/dyoshikawa/rulesync|README|README|Medium|
|48|https://rulesync.dyoshikawa.com/reference/file-formats|rulesync 파일 형식|공식 문서|High|
|49|https://github.com/vercel-labs/skills|README, `docs/commands.md`, `docs/agents.md`|README·문서|Medium|
|50|https://github.com/numman-ali/openskills|README|README|Medium|
|51|https://agentskills.io/specification|Agent Skills 사양|공식 문서|High|
|52|https://github.com/intellectronica/skillz|README|README|Medium|
|53|https://github.com/dagger/container-use|`repository/flock.go`, `mcpserver/singletenant.go`, `configure_claude.go`|README·소스|Medium|
|54|https://github.com/BloopAI/vibe-kanban|README, `crates/mcp`, `worktree_manager.rs`|README·소스|Medium|
|55|https://www.vibekanban.com/blog/shutdown|"Goodbye bloop" 공지|공식 공지|High|
|56|https://github.com/smtg-ai/claude-squad|`session/git/worktree.go`, `config/state.go`|README·소스|Medium|
|57|https://github.com/stravu/crystal|`mcpPermissionBridge.ts`, `permissionManager.ts`|README·소스|Medium|
|58|https://github.com/tumf/mcp-shell-server|README, `command_validator.py`|README·소스|Medium|
|59|https://github.com/anthropic-experimental/sandbox-runtime|README(일부)|README|Medium|
|60|https://github.com/Dicklesworthstone/mcp_agent_mail_rust|README|README|Medium|
|61|https://github.com/Dicklesworthstone/mcp_agent_mail|README|README|Medium|
|62|https://github.com/xiperinc/singular-lite|README|README|Low|
|63|https://github.com/MisterChief95/dibs|README|README|Low|
|64|https://github.com/ThatHunky/agent-coord|README|README|Low|
|65|https://github.com/TadMSTR/scoped-mcp|README|README|Low|
|66|https://github.com/eqtylab/cupcake|README|README|Medium|
|67|https://github.com/rmax-ai/hermes-mcp-gateway|README|README|Low|
