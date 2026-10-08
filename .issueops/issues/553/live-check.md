# #553 실측 기록

- 관측 시각: 2026-10-08 02:20~02:33 UTC
- 환경: macOS 27.0, Claude Code 2.1.293, codex-cli 0.161.0
- 바이너리: canonical worktree에서 `go build -o bin/issueops ./cmd/issueops`로 만든 빌드
- 실행 위치: claude·codex 실측은 세션 scratch 디렉터리(git 저장소 아님)에서 했다. 저장소 파일은 바꾸지 않았다.

## Claude: 주입한 역할 에이전트가 설정 모델로 실행된다

1. `issueops model resolve --host claude --role research --agents --json`
   - 결과: `model=claude-sonnet-5-5`, `effort=medium`, 출처 `default`.
   - `agent_args`는 `--agents`와 JSON 하나다. JSON 키는 `issueops-plan-review`, `issueops-diff-review`,
     `issueops-review-escalate`, `issueops-research`, `issueops-reader-check` 다섯 개다.
2. 부모를 다른 모델로 띄웠다:
   `claude -p --model haiku --output-format json --allowedTools Agent --agents "<agent_args[1]>" -- "<issueops-research 에이전트를 한 번 호출해 자기 model ID를 답하게 하라>"`
   - 종료 코드 0, 응답 `claude-sonnet-5-5`.
   - `modelUsage` 키: `claude-haiku-5-5`, `claude-sonnet-5-5`.
   - 세션 transcript: 부모 메시지의 model은 `claude-haiku-5-5`뿐이다. 서브에이전트 transcript
     (`subagents/agent-*.jsonl`, meta의 `agentType=issueops-research`)의 model은 `claude-sonnet-5-5`뿐이다.
3. 함정: print 모드에서 `--agents <json>` 바로 뒤에 프롬프트를 위치 인자로 두면 claude가 프롬프트까지
   `--agents` 값으로 받아 `Input must be provided` 오류로 끝난다. `--` 뒤에 프롬프트를 두면 정상이다.
   cmux argv는 `--agents <json> -- <prompt>` 순서이고, Orca owner 명령은 위치 인자 프롬프트가 없다.

판정: claude pass

## Codex: 자식 스레드가 role 파일의 model·effort로 실행된다

1. 임시 `XDG_CONFIG_HOME`·`ISSUEOPS_STATE_DIR`을 scratch 아래로 두고
   `issueops model set --scope global --host codex --role research --model gpt-6-luna --effort low --json`.
   경고 없음.
2. `issueops model resolve --host codex --role research --agents --json`
   - 결과: `gpt-6-luna / low`, 출처 `global / global`.
   - `agent_args`는 역할마다 `-c agents.issueops-<role>.config_file="<state>/agent-roles/<sha256>.toml"` 다섯 쌍이다.
   - research role 파일 내용: `name = "issueops-research"`, `model = "gpt-6-luna"`, `model_reasoning_effort = "low"`, description, developer_instructions.
3. 부모 effort를 다르게 띄웠다(계정 플랜상 부모 모델도 luna):
   `codex exec --skip-git-repo-check -m gpt-6-luna -c model_reasoning_effort=medium <agent_args...> "<issueops-research 에이전트를 한 번 띄워 OK를 답하게 하라>"`
   - 종료 코드 0, 응답 `OK`.
4. `~/.codex/state_5.sqlite` 조회(cwd가 실측 디렉터리인 스레드):
   - 부모 `01a1195a-3902-…`: agent_role 없음, `gpt-6-luna`, `medium`
   - 자식 `01a1195a-4ff1-…`: agent_role `issueops-research`, `gpt-6-luna`, `low`
   - `thread_spawn_edges`: 부모 → 자식 한 줄

판정: codex pass

## local 설정: canonical worktree에서 메인 워크트리 파일을 쓴다

1. canonical worktree에서
   `./bin/issueops model set --scope local --host codex --role research --model gpt-6-luna --effort medium --json`.
   동작을 바꾸지 않도록 내장 기본값과 같은 값을 썼다.
   - `path`: `<source root>/.issueops/agent-models.local.json`(메인 워크트리), `excluded: true`.
2. 같은 위치에서 `./bin/issueops model resolve --host codex --role research --json`:
   `gpt-6-luna medium`, 출처 `local / local`.
3. `git -C <source root> status --short`는 빈 출력이고, canonical worktree의
   `git status --short`에도 `agent-models` 줄이 없다.
4. 관측 뒤 `model unset --scope local ...`(removed=true)으로 되돌리고, 남은 `{"version": 1}` 파일을 지웠다.
   메인 저장소 `.git/info/exclude`의 `/.issueops/agent-models.local.json` 줄은 무해하므로 남겼다.

판정: local pass
