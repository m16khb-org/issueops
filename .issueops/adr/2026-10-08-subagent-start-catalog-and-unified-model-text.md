# 2026-10-08 — SubagentStart carries the project-doc catalog, and every host gets the same model text

← [ADR index](../ADR.md)

**결정:** Codex와 Claude의 기본 설치는 `SessionStart`와 `SubagentStart` 두 context hook을 등록한다. `issueops hook subagent-start`는 시작하는 서브에이전트에 `SessionStart`와 같은 모델용 catalog를 주고, `agent_type`이 대소문자 무시 `explore`·`explorer`·`fork`면 `{}`를 낸다. 모델용 `additionalContext`는 호스트와 상관없이 같은 텍스트다. 첫 줄 `Project docs under .issueops/ (read the ones relevant to the task before acting):` 뒤에 문서마다 `- .issueops/<NAME>.md: <설명>` 한 줄을 둔다. 사용자 화면에는 Claude `SessionStart`의 `systemMessage` 한 줄(`📚 <레포 이름> · project docs <N>개 (.issueops/)`)만 보인다. 표준 문서 설명(`meta.go`)은 `<무엇>; read <언제>.` 형식의 짧은 영어 한 문장이다.

이 기록은 [2026-08-27 SessionStart owns compaction context](2026-08-27-session-start-owns-compaction-context.md)의 "기본 설치는 `SessionStart` 하나만 등록한다" 조항을 대체한다. 같은 기록의 `PostCompact` 미등록과 legacy hook 삭제 결론은 그대로다. [2026-08-10 default hooks are thin static context only](2026-08-10-default-hooks-thin-static-context.md)의 "hook은 정적 project-doc catalog만 렌더하고 IssueOps state를 읽지 않는다"는 새 hook에도 똑같이 적용된다.

**근거 (설치된 host 바이너리와 transcript로 확인, 2026-10-08):**

- Claude Code 2.1.293은 `SubagentStart` 입력에 `agent_type`을 주고 출력의 `hookSpecificOutput.additionalContext`를 받는다. codex-cli 0.161.0의 내장 schema에도 subagent-start output의 `additionalContext`·`systemMessage`가 있다.
- 최근 30일 worktree 세션에서 서브에이전트가 205회 시작했다(그중 general-purpose 190회). fork가 아닌 서브에이전트는 빈 컨텍스트에서 시작하므로, 메인 세션만 catalog를 받으면 서브에이전트는 `.issueops/` 문서가 있다는 사실을 모른다.
- 이전 Codex 출력은 한국어 bullet 목록을 `additionalContext`에 실었고, Claude는 경로가 빠진 한 줄 목록을 실었다. 호스트마다 모델이 받는 텍스트가 달랐고, 어느 쪽에도 문서를 열 경로가 없었다.
- 워크트리 세션에서 경로 basename을 레포 이름으로 쓰면 브랜치 디렉터리 이름이 나온다. 레포 이름은 `install.ResolveStableNativeRoot`로 원본 checkout을 찾아 정한다(exec 없이 `.git` gitdir·commondir 파일만 읽는다). Git 메타데이터를 읽을 수 없으면 디렉터리 이름을 쓴다.
- 이 레포에서 바이너리를 10회 실행한 중앙값(2026-10-08T04:44Z): `session-start` 18.4ms, `subagent-start` 19.0ms(+3%), 제외 대상 `Explore` 13.7ms. 서브에이전트마다 추가되는 catalog는 1,599바이트다.

**결과:**

- 두 호스트 설정 파일에 `SubagentStart` issueops 그룹이 하나씩 생긴다. upgrade는 다른 도구의 `SubagentStart` 그룹(예: Orca)을 순서와 내용 그대로 보존한다. Claude `SubagentStart`에는 matcher가 없다. 제외 판정은 hook 안에서 한다.
- Codex는 새 hook 항목을 사용자가 한 번 신뢰 승인해야 실행한다(`config.toml`의 `[hooks.state]`). 설치기는 그 승인을 대신 쓰지 않는다. 기존 `SessionStart` 명령 문자열은 바뀌지 않아 저장된 신뢰는 유지된다.
- Codex TUI의 `hook context:` 줄은 `additionalContext`를 그대로 보여 주므로, Codex 사용자는 이제 영어 경로 목록을 본다. 의도한 결과다.
- `SubagentStart`는 어느 호스트에도 `systemMessage`를 내지 않는다. 서브에이전트마다 같은 알림이 화면에 반복되는 것을 막는다.
- `hook session-start --json` DTO 필드는 그대로다. `compact` 값의 형식만 바뀌고, Omo 확장은 그 값을 그대로 모델에 넣는다.
- `meta.go`가 바뀌었으므로 이후 `project bootstrap --sync`를 실행하는 레포의 표준 문서 frontmatter가 새 문구로 바뀐다.
- 머지 전에 로컬 `self-verify`를 실행하면 native integration 단계 하나(`Codex thin context hooks missing issueops context hook surface`)가 실패한다. 사용자 설치에 아직 `SubagentStart`가 없어서다. 머지 뒤 source checkout에서 `io update`를 실행하면 해소된다. push마다 도는 CI의 임시 HOME self-verify가 권위 있는 증거다.

**롤백:** 순서를 지킨다. (1) `~/.claude/settings.json`과 `~/.codex/hooks.json`에서 `hook subagent-start`를 부르는 그룹을 먼저 지운다. (2) 이 변경을 되돌린 빌드로 `io update`를 실행한다. 순서를 바꾸면 이전 바이너리의 activation readback이 `installed hook event SubagentStart contains an unexpected issueops group`으로 실패해 `io update` 자체가 멈춘다.

**거절:**

- `PreToolUse`·`PostToolUse`·`Stop`·`PreCompact`·`UserPromptSubmit` hook 추가. 압축 뒤 catalog는 `SessionStart(compact)` 재실행이 이미 다시 주입하고, 압축 뒤 `issueops next` 228회 중 222회가 `--id`를 유지해 compact hook으로 보완할 손실이 작았다. tool 단계 차단은 2026-08-27에 지운 enforcement 표면을 되살리는 일이다. 같은 기간 사이클 안 `orca worktree create` 18건은 모두 정상 cherry-pick으로 끝났고, raw `gh pr create`는 2건이었다.
- Claude matcher로 제외 대상을 거르는 방식. Codex에는 같은 matcher가 없어 두 호스트 판정이 갈라진다.
- 호스트별로 모델 텍스트를 다르게 두는 방식. 모델은 같은 정보를 받아야 하고, 화면 표시는 `systemMessage` 한 줄로 따로 다룬다.
