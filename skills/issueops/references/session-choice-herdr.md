# Herdr 새 세션 인계

[세션 인계](session-choice.md)에서 Herdr `new-session`을 선택했을 때만 이 문서를 읽는다. release까지의 준비 절차와 인수 체인은 그 문서가 소유한다.

## raw launcher 전송 관측

Herdr를 호출할 때는 `issueops trace handoff-delivery`를 공통
producer로 사용한다. 이 명령은 user-state audit에 관측만 추가하며 retry, claim, status를
바꾸지 않는다. 입력 JSON에는 실제로 읽은 launcher path/version, runtime과 server 또는 target
identity, terminal/pane, generation, prompt와 material digest를 넣는다. `unknown`, `pending`,
runtime ID를 복사한 server ID 같은 대체값을 만들지 않는다.

외부 호출 직전에는 안전한 임시 파일에 다음 observation을 만들고 producer 성공을 확인한다.
`created_at`, `updated_at`, `call_staged.observed_at`에는 같은 RFC3339Nano 시각을 쓴다.

```json
{
  "schema_version": 1,
  "attempt_id": "manual-direct:<lifecycle>:<generation>:<launcher>:<one attempt>",
  "lineage_id": "manual-direct:generation:<generation>:prompt:<prompt sha256>:material:<material sha256>:call:prompt",
  "lifecycle_id": "<lifecycle>",
  "prompt_sha256": "<sha256>",
  "material_sha256": "<sha256>",
  "request": {"durable_id": ""},
  "launcher": {"name": "herdr", "version": "<observed>", "path": "<absolute observed path>", "runtime_id": "<observed>", "machine_id": "<observed>", "server_id": "<observed target identity>"},
  "target": {"terminal_id": "<observed>", "pane_id": "<observed>"},
  "expected_owner_host": "<codex|claude|omo|omp|opencode>",
  "source_generation": <generation>,
  "created_at": "<RFC3339Nano>",
  "updated_at": "<same RFC3339Nano>",
  "receipt": {},
  "call_staged": {"status": "observed", "observed_at": "<same RFC3339Nano>", "evidence": "external_call_staged"},
  "input_accepted": {"status": "not_observed"},
  "native_turn_observed": {"status": "not_observed"},
  "owner_claimed": {"status": "not_observed"},
  "ambiguous": {"status": "not_observed"}
}
```

```bash
issueops trace handoff-delivery --input "$OBSERVATION_JSON" --json
```

receipt를 받은 뒤 같은 attempt/lineage/`created_at`으로 두 번째 observation을 기록한다.
launcher가 반환한 실제 ID가 있으면 그 값만 `request.durable_id`에 넣고 실제 `processIncarnation`도
보존한다. launcher가 제공하는 process 조회로 수신 agent의 PID, 시작 시각, executable을 확인해
`target.process`에 기록한다. 이 PID reuse-safe receipt를 확인할 수 없으면 owner claim과 연결하지
않으며, 입력 수락 관측까지만 남긴다.

```json
{
  "target": {
    "terminal_id": "<observed>",
    "pane_id": "<observed>",
    "process_incarnation": "<observed>",
    "process": {
      "pid": <receiver pid>,
      "started_at": "<receiver process RFC3339Nano start time>",
      "executable": "<absolute observed receiver executable>"
    }
  },
  "source_generation": <generation>
}
```

이 조각은 두 번째 observation의 필수 receiver correlation 필드다. staged observation에는
launcher가 아직 반환하지 않은 process를 추측해 넣지 않는다. 완료 observation의 process는
claim holder의 `session_process`와 PID·시작 시각·executable이 모두 일치해야 owner-claim
근거로 연결된다.
첫 호출에는 retry ID를 넣지 않는다. 입력 수락 receipt에는 `input_accepted`의 evidence로
`launcher_receipt`를 사용한다. 실제 native host 기록에서 새 turn을 확인한 경우에만
`native_turn_observed`를 `native_receipt`로 기록한다. timeout이나 응답 유실은 `ambiguous`에
각각 `timeout` 또는 `accepted_response_lost`로 기록하며 자동 재전송하지 않는다.

Herdr의 `pane run`이나 `agent prompt` 성공은 `launcher_accepted` 입력 증거로만 기록한다.
`agent_prompt_stalled`와 wait 결과는 `ambiguous`의 `agent_prompt_stalled` 또는
`herdr_wait_state`이며 native turn 증거가 아니다. 수신자가 IssueOps claim에 성공하면 claim
handler가 기존 권한 저장소를 읽어 `owner_claimed`를 별도로 기록한다. 수동 JSON으로
`owner_claimed`를 만들지 않는다. claim handler는 current generation, expected host, 단 하나의
manual lineage, 입력 수락 상태, claim holder와 일치하는 PID·시작 시각·executable을 모두 확인한
경우에만 그 manual observation에 claim 증거를 추가한다.

두 번째 observation JSON을 만든 뒤 같은 producer를 다시 호출하고, 응답의
`observation.receipt.location`과 digest를 읽어 실제 audit frame을 확인한다.

```bash
issueops trace handoff-delivery --input "$OBSERVATION_JSON" --json
```

인계문에는 다음 내용을 실제 값으로 채운다.

```text
같은 IssueOps 사이클을 이어서 수행하세요.
ID: <lifecycle ID>
worktree: <absolute path>
issue / branch / plan: <verified values>
인계 자료 digest: <sha256>
목적, 비목표, 승인된 종료점: <actual values>
source root와 canonical worktree: <actual values>
base head, full head, diff: <actual values>
계획 경로와 계획 digest: <actual values>
검증 입력, 명령, 시각, 환경, 실패: <actual values>
미완료 작업과 결과 위치: <actual values>
현재 lifecycle 상태: <actual state>
인수 명령과 읽기 전용 확인 명령: <exact commands>
사용자 선택 기록: status.decisions의 <created_at>, 제목 "세션 인계 선택"
선택: new-session. 런처: <herdr|cmux|manual>. 결정 근거: <사용 가능 관찰 또는 명시적 지시>
원래 사용자 요청: <actual request and conversation reference>
승인 범위와 종료점: <scope>, <draft PR/MR publication + execution complete, or narrower endpoint>
기존 holder는 release를 마쳤습니다. 현재 status와 선택 기록, 현재 HEAD, 계획 digest,
인계 자료 digest를 읽고 인계 내용과 대조하세요. stale하거나 누락된 근거는 필요한 범위만 다시 확인하세요.
같은 worktree에서 next --id가 제공하는 복구 명령 체인을 따라 자기 native actor로 인수하세요.
released 상태는 `execution replace --id ID --expected-generation N --preview`로 시작하고,
반환된 `--reseed` 명령을 실행한 뒤 `claim --claim-current-token`으로 인수합니다. 현재 generation을 직접 관측하세요.
서로 다른 host의 session ID는 이식 가능한 identity가 아니므로 새 host/session은 자기 native actor flags로 식별하세요.
active(self)가 된 뒤 승인된 범위 안에서 재질문 없이 이어가세요. 자동 인계를 다시 적용해 새 세션을 띄우지 마세요.
기록보다 최신 사용자 지시가 우선합니다.
```

계획·범위·작업 경로가 선택 기록과 달라졌으면 원인을 조사한다. 다른 사이클의 승인이나
phase/claim 성공으로 승인 범위를 넓히지 않는다. 승인 근거를 확인할 수 없을 때만 필요한
결정을 묻는다. core의 actor·generation 검사는 그대로 통과해야 한다.

## Herdr 실행

이 절은 [세션 인계](session-choice.md)의 release가 확인된 뒤에만 실행한다. Herdr는 세션 배치만 맡으며
IssueOps lease를 소유하지 않는다. 실제 Herdr 호출 전후에는 위
`trace handoff-delivery` producer를 사용한다.

1. `SOURCE_ROOT`와 `WORKTREE`는 record에서 확인한 절대경로다. 기존 checkout을 연다.

   ```bash
   herdr worktree open --cwd "$SOURCE_ROOT" --path "$WORKTREE" --no-focus
   ```

   응답의 `.result.workspace.workspace_id`와 `.result.root_pane.pane_id`를 사용한다.
   `.result.worktree.path`·branch가 canonical worktree·record와 일치하는지 확인한다.
   `already_open`이면 기존 workspace를 재사용한다. 앞선 인계가 이미 실행됐는지
   `pane list`와 native 세션 기록부터 확인하며, 기존 에이전트나 사용자 shell 작업에
   명령을 보내지 않는다. 이 인계의 세션이 없고 기존 pane이 사용 중이면 같은 workspace에
   `herdr tab create --workspace "$WORKSPACE_ID" --cwd "$WORKTREE" --no-focus`로
   빈 shell pane 하나를 만든다. 반환된 `.result.root_pane.pane_id`를 이후 대상으로 쓴다.
2. `herdr pane get "$PANE_ID"`와 `herdr pane process-info --pane "$PANE_ID"`로
   canonical cwd와 foreground가 빈 interactive shell임을 확인한다.
   현재 native host별로 **한 경로만** 실행한다.

   - Claude Code: `herdr agent start "$AGENT_NAME" --kind claude --pane "$PANE_ID"
     --timeout 60000 -- --dangerously-skip-permissions`를 사용한다.
   - Codex: `herdr agent start "$AGENT_NAME" --kind codex --pane "$PANE_ID"
     --timeout 60000 -- --dangerously-bypass-approvals-and-sandbox`를 사용한다.
     이름은 `agent list`와 대조한 고유한 이름이고, 모델·effort 인자는 해당
     호스트의 설치된 help에서 확인한 경우에만 같은 `--` 뒤에 추가한다.
     성공 후에도 `herdr agent read "$AGENT_NAME" --source visible`로 입력창을 확인한 뒤
     `herdr agent prompt "$AGENT_NAME" "$HANDOFF"`로 한 번 전달한다.
   - Claude Code와 Codex의 모델·effort는 `issueops model resolve --host "$HOST" --role implement
     --agents --json`의 `model`·`effort`를 쓴다(child cycle은 `--role child-implement`). 역할
     서브에이전트는 같은 응답의 `agent_args`를 같은 `--` 뒤에 원소 그대로 붙인다. cmux 인계와
     같은 인자이며, 다시 인용하거나 합치지 않는다.
   - Omo: Herdr 0.9.0의 kind 목록에는 `omo`가 없다. `pi`나 `omp`, Claude로 대체하지
     않는다. `omo --model … --thinking … --permission-preset full-access -- "$HANDOFF"`처럼 설치된 CLI가 지원하는
     **새 interactive 세션과 초기 프롬프트**를 하나의 shell 명령으로 구성하여
     `herdr pane run "$PANE_ID" "$LAUNCH_COMMAND"`으로 한 번 실행한다.
     실행 파일·모델·프롬프트를 각각 shell-quote하며 프롬프트의 따옴표·개행·`$`·backtick을
     shell이 평가하지 못하게 한다. `--continue`, `--resume`, `--print`는 사용하지 않는다.
     `pane run` 성공은 입력 제출 영수증일 뿐이다. `pane read --source visible`,
     foreground process와 Omo native 세션 기록으로 새 세션·cwd·인계 수신을 확인한다.
     Herdr의 agent 감지가 없어도 native 기록으로 확인하며 감지를 성공으로 꾸미지 않는다.
   - omp: `herdr agent start "$AGENT_NAME" --kind omp --pane "$PANE_ID" --timeout 60000 --
     --model "$MODEL" --thinking "$EFFORT" --auto-approve`를 사용한다. 모델·effort는
     `issueops model resolve --host omp --role implement --json`의 값이다. 성공 후
     `herdr agent read "$AGENT_NAME" --source visible`로 입력창을 확인하고
     `herdr agent prompt "$AGENT_NAME" "$HANDOFF"`로 한 번 전달한다.
   - opencode: 모델은 TUI 플래그가 아니라 `OPENCODE_CONFIG_CONTENT`로만 전달되므로
     `herdr pane run "$PANE_ID" "$LAUNCH_COMMAND"`로 한 번 실행한다. `LAUNCH_COMMAND`는
     `OPENCODE_CONFIG_CONTENT='{"model":"<implement model>"}' opencode --standalone --auto --prompt '<handoff>'`이며
     모델과 인계문을 각각 shell-quote한다. 모델은 `issueops model resolve --host opencode --role implement
     --json`의 `model`이고 effort는 넣지 않는다(TUI는 사용자 `model.json`의 variant를 쓴다).
     수신은 Herdr가 opencode TUI plugin으로 보고하는 `ses_…` 세션 id와 새 세션 셸의
     `issueops execution whoami --json`(host `opencode`, `session_id_source` `OPENCODE_SESSION_ID`)으로 확인한다.
3. Herdr의 `idle`/`interactive_ready`만으로 준비 완료라고 판단하지 않는다.
   0.9.0에서 Claude의 첫 MCP 선택 화면도 `idle`로 관측됐다. 실제 화면의 질문을 읽고
   기존 승인 범위 안에서 처리한다. 필요 없는 선택 기능은 화면이 제공하는 건너뛰기를
   사용한다.
4. 입력 제출 직전에 host의 이벤트/상태 구독을 등록하고 유한한 timeout을 둔다.
   전달 완료는 native 세션 기록에 exact 인계가 수신됐거나 새 세션의 수신 응답으로
   확인한다. 프롬프트가 입력창에만 남아 있거나 Herdr가 `done`이라고 한 것만으로는
   충분하지 않다. `agent_prompt_stalled`·timeout은 미전달 증거가 아니므로 재전송 전에
   같은 pane과 native 기록을 읽는다. 미전달을 확인하고 장애를 해소한 경우에만
   같은 세션에 한 번 재전송한다. 여전히 모호하면 ID·경로·관측 오류를 보고하고 멈춘다.
   구현 완료를 기다리지 않으며, 세션을 자동 삭제하거나 다른 세션을 추가 생성하지 않는다.

명령 계약은 설치된 `--help`와 [Herdr CLI 문서](https://herdr.dev/docs/cli-reference/)를
함께 확인한다. Herdr workspace/pane ID는 실행 위치 식별자이지 IssueOps actor가 아니다.
인수하는 세션은 공통 인계문의 `whoami`·`next` 복구 절차를 그대로 따른다.

