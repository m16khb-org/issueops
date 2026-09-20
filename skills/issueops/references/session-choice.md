# 준비된 워크트리에서 자동 세션 인계

[IssueOps](../SKILL.md)의 인계 지점에 도달했거나 인계받은 새 세션만 이 문서를
읽는다. CLI의 `direct|orca`는 workspace/lease 운영 방식이다. 새 native 세션을 여는 것만으로
mode를 바꾸지 않는다. 일반 흐름은 direct로 만든 **같은 worktree**를 유지한다.

## 환경 판별과 승인 근거 기록

실측한 lifecycle ID, issue URL, branch, worktree 절대경로, 계획과 성공 기준을 확인한다.
이미 인계받은 세션은 기존 기록과 승인 범위를 대조한 뒤 이어간다. 환경 판별부터 다시
시작해 새 세션을 연쇄 생성하지 않는다.

준비 세션은 설치된 `orca-cli` 안내로 실행 파일을 정하고 `orca status --json`을 확인한다.
`runtime.state == "ready"`면 Orca의 `new-session`을 선택한다. 기존 Orca execution은
prepare·resume·reconcile에 연결된 production observer를 사용한다. direct execution의 raw
Orca 전송은 아래 `trace handoff-delivery` producer로 실제 호출 전후를 기록한다. Orca가 없거나
unready인 것이 확인됐을 때만 Herdr를 확인한다.

```bash
command -v herdr
herdr status
herdr worktree open --help
herdr pane run --help
```

Herdr는 실행 중인 서버가 있고 client/server endpoint가 호환되며, 기존 worktree를
열고 **현재 native host**를 실행할 수 있을 때 `new-session`을 선택한다. Herdr 전송도 아래
`trace handoff-delivery` producer로 호출 전후를 기록한다. Claude·Codex는
설치된 `herdr agent start --help`의 kind와 해당 실행 파일을 확인하고, Omo는
`omo --help`의 초기 프롬프트 지원과 `pane run`을 확인한다. `herdr status`는
사람용 출력이므로 존재하지 않는 JSON 필드나 `--json` 옵션을 가정하지 않는다.
별도 세션/socket 환경을 쓰면 모든 명령에 같은 대상을 유지한다.

둘 다 없거나 사용 불가임이 확인됐으면 `current`다. 외부 도구를 자동 설치하거나
서버를 시작하지 않는다. 설치·업데이트·self-verify의 필수 조건으로도 추가하지 않는다.
실행 방식을 묻지 않는다. status 오류로 사용 가능 여부를 판별할 수 없으면 오류를 보고한다.
이미 시작한 open·launch의 실패·모호함을 도구 부재로 해석해 다른 런처로 재실행하거나
현재 세션에서 구현하지 않는다.
사용자가 현재 세션·새 세션·보류를 명시했으면 그 지시가 우선한다.

### 사용자가 명시한 cmux 인계

cmux는 위 자동 선택에 참여하지 않는다. 사용자가 cmux 사용을 명시했고, 기존 direct
execution이 정확한 generation에서 released이며 canonical worktree가 이미 있을 때만 다음
명령을 실행한다. bare `cmux <path>`는 앱을 자동으로 시작할 수 있으므로 실행하지 않는다.
Omo도 generic terminal 안에서 native Omo 실행 파일을 직접 실행하며 `cmux omo`를 호출하지
않는다.

prompt 파일은 canonical worktree 안의 absolute regular file로 만들고 mode 0600, 최대 512 KiB,
SHA-256을 확인한다. material digest는 봉인된 인계 자료의 SHA-256이다. cmux와 native host
실행 파일, socket, window UUID, model을 실측한 exact 값으로 채우고, 해당 host가 지원하는
effort만 전달한다.

```bash
issueops execution handoff-cmux \
  --id "$ISSUEOPS_ID" --generation "$GENERATION" \
  --cmux-executable "$CMUX_EXECUTABLE" --cmux-version "$CMUX_VERSION" \
  --cmux-build-identity 'cmux 0.64.10 (90) [fafa50702]' \
  --socket "$CMUX_SOCKET" --window "$CMUX_WINDOW_UUID" --cwd "$ISSUEOPS_EXPECTED_WORKTREE" \
  --host "$HOST" --host-executable "$HOST_EXECUTABLE" \
  --model "$MODEL" --effort "$EFFORT" \
  --prompt-file "$PROMPT_FILE" --prompt-sha256 "$PROMPT_SHA256" \
  --material-sha256 "$MATERIAL_SHA256" --json
```

이 명령은 앱을 시작하거나 설치하지 않고 socket을 검색하거나 권한을 바꾸지도 않는다.
absolute non-symlink Unix socket의 owner, mode, parent, device, inode, ctime을 읽어 **endpoint
incarnation**으로 봉인하고, exact executable/version, `ping`, `capabilities`, exact window의
`identify --no-caller`만 bounded read-only preflight로 실행한다. socket 부재·접근 거부,
불완전하거나 중복된 응답, 필요한 capability 부재, endpoint incarnation 변화는 mutation 전에
fail-closed된다.

요청의 `--cwd`, 실제 issueops 프로세스 cwd, durable canonical worktree가 모두 같은 실제
디렉터리인지 확인한 뒤에만 preflight를 시작한다. 같은 lifecycle·generation에 cmux
`call_staged`가 하나라도 있으면 window나 prompt가 달라도 새 시도를 거부한다. preflight가
끝나면 같은 handoff-delivery lineage에 exact window와 cwd만 `call_staged`로 먼저
기록한다. 아직 없는 workspace/surface identity를 만들지 않는다. 그 뒤 exact window에 빈
workspace 하나를 만들고, 반환된 workspace의 단일 pane/surface와 cwd를 다시 확인해 같은
관측을 보강한 다음, 그 exact window/workspace/surface로 private launcher command를 한 번만
보낸다. private artifact는 mode 0700 directory, mode 0600 prompt, mode 0700 launcher를 쓰며
receiver는 private expected 변수와 cmux가 제공한 실제 `CMUX_WORKSPACE_ID`,
`CMUX_SURFACE_ID`, `CMUX_SOCKET_PATH`를 대조하고 `CMUX_WINDOW_ID`가 있으면 window도
대조한다. cwd와 scope가 맞을 때만 receipt를 남긴 뒤 prompt와 launcher를 지운다.

cmux 0.64.10의 성공한 raw input은 `input_accepted`의 `raw_input` 증거일 뿐이다.
`native_turn_observed`나 `owner_claimed`를 설정하지 않는다. bootstrap PID에서 관측한 실행
파일이 기대한 native host executable과 같은 파일임을 입증한 경우에만 process receipt를
붙인다. launcher가 별도 자손을 시작해 정확한 agent descendant를 입증할 수 없으면 process
evidence를 비워 두고 raw input 접수까지만 보고한다. 수신자의 별도 IssueOps claim CAS만
owner claim을 만든다. 이 버전은 runtime, machine, server ID를 노출하지 않으므로 빈 값을
추측해 채우지 않는다. endpoint incarnation은 경로에서 관측한 socket 항목이 preflight와
mutation 사이에 같았다는 관측값일 뿐, 실제 peer identity나 socket race의 완전한 차단,
cmux runtime identity, live host 지원 인증을 뜻하지 않는다.

workspace create 또는 send가 timeout, 응답 유실, malformed receipt, target/cwd/runtime 변화로
끝나면 같은 attempt/lineage의 ambiguous evidence와 recovery artifact를 확인하고 명령을 다시
실행하지 않는다. durable request ID가 반환되지 않으므로 새 값을 만들거나 blind retry에 쓰지
않는다. exact 반환 ID와 read-only `identify`/`list-*`로 기존 workspace를 조사하며 자동으로
workspace를 닫지 않는다. 명시적인 복구 결정을 내리기 전에는 다른 런처나 current로 전환하지
않는다.

결정 결과와 원래 요청의 종료점을 짧게 알리고 아래 기록을 남긴다. 자동 결정은 세션
배치만 정하며 사용자 요청에 없던 commit·push·publication 권한을 만들지 않는다.

```bash
issueops decision add --id "$ISSUEOPS_ID" --kind implementation \
  --title "실행 방식 선택" --body "$CHOICE_RECORD" \
  --rationale "환경별 자동 세션 인계 규칙 또는 명시적 사용자 지시" \
  $RECORD_ACTOR_FLAGS --json
```

`CHOICE_RECORD`에는 `current|new-session|hold`, 자동 결정인지 명시적 지시인지,
선택한 런처(`orca|herdr|none`, 명시적 지시일 때만 `cmux`)와 확인한 status·host 실행 가능 관찰값(없으면 부재 근거),
원래 사용자 요청과 그 대화 위치,
선택 시점의 lifecycle ID·issue URL·branch·worktree·계획 경로, 승인 범위·종료점과
현재 generation을 적는다. `hold`의 승인 범위는 보류다. 이 기록의 created_at과 내용을
`status --id`로 읽어 확인한다. 사용자 답변을 지어내거나 예시 값을 그대로 저장하지 않는다.
같은 인계에 대한 기록이 이미 있으면 중복 기록하지 않는다. 취소·범위 수정 등 이후 지시가 우선한다.

## 1 현재 세션

기존 holder와 generation을 유지한다. 도구의 작업 경로를 canonical worktree로 맞추고
`next --id`가 가리키는 단계로 이어간다. mode 전환, release, 새 세션 생성은 하지 않는다.

## 2 새 세션

이 절의 일반 경로는 direct execution이다. **기존 Orca execution**에서는 선택을 기록하고
release한 준비 세션이 `status`의 replace/reseed/resume 체인을 따른다. core의 `resume`이
봉인된 새 owner를 띄우므로 별도 terminal/session을 먼저 만들지 않는다. 새 owner는 자기
봉인된 claim을 사용하고 결정 기록을 읽는다. 수동으로 연 세션은 이 owner를 대신 claim하지
않고 coordinator 복구 경로만 사용한다. 아래 3번의 별도 세션 실행은 이 경우 생략한다.

1. 인계할 내용을 먼저 준비한다. 우선 **새 쓰기 작업과 하위 작업 dispatch를 중지**하고,
   이미 시작한 작업을 빠짐없이 열거한다. 각 항목에는 **소유자, 실행 핸들, 입력 리비전,
   쓰기 범위, 결과 위치**를 적고, **읽기 작업인지 쓰기 작업인지** 분류한다.
   build, golden, generator, formatter, fixture, 분류할 수 없는 작업은 명령, cwd, 출력 위치로
   공유 상태를 바꾸지 않는다는 점이 확인되기 전까지 writer로 다룬다. 공유 상태 밖의
   immutable 입력만 읽는 독립 reader만 계속 실행할 수 있으며, writer와 그 자손 프로세스는
   release 전에 종료하거나 취소해야 한다.
2. bounded observation으로 writer와 **자손 프로세스가 실제로 종료**됐는지 확인한다.
   signal 전송이나 lease release만으로는 프로세스 종료를 증명할 수 없다. canonical
   worktree와 공유 state를 다시 읽어 **대기 중인 writer가 0**임을 확인하고, 성공·실패 출력과
   결과 위치를 자료에 적는다. **늦게 도착한 결과는 격리**해 입력 리비전과 출처만 남긴다.
   이전 세션의 callback이나 결과는 release 뒤 source 변경이나 pass 기록의 근거가 될 수 없다.
   사용자의 최신 취소나 범위 변경이 저장된 인계문보다 우선하며, 그 지시가 오면 새 dispatch를
   중지한다.
3. 봉인된 인계 자료를 만든다. 필수 필드는 **목적, 비목표, 승인된 종료점**,
   **source root와 canonical worktree**, **base head, full head, diff**, **계획 경로와 계획
   digest**, 완료한 조사와 검증의 **입력, 명령, 시각, 환경, 실패**, **미완료 작업과 결과 위치**,
   **현재 lifecycle 상태**, exact **resume 명령과 읽기 전용 확인 명령**, 그리고 인계 자료
   digest다. 자료에는 secret, 인증 정보, 이전 lease token을 넣지 않는다.
4. 위 결정의 created_at과 내용, exact ID·issue·branch·worktree, 계획 경로, 현재 단계·성공 기준·
   승인된 종료점을 인계문에 실제 값으로 채운다. 인계문에 secret이나 claim token은 넣지 않는다.
   수신자는 **현재 HEAD, 계획 digest, 인계 자료 digest**를 대조하고, 일치하는 근거만
   재사용한다. stale하거나 누락된 근거는 필요한 범위만 다시 확인한다. 서로 다른 host의
   session ID는 이식 가능한 identity가 아니다. 새 host/session은 durable actor flags와 runtime
   receipt로 다시 식별한다. **독립 direct claim**은 이 자료와 status를 검증하지만,
   일반 direct claim에 Orca owner context packet을 요구하지 않는다.
5. 기존 holder가 canonical worktree에서 최신 `whoami`의 actor flags로 release한다.

   ```bash
   issueops execution release --id "$ISSUEOPS_ID" --generation "$GENERATION" $ACTOR_FLAGS --json
   issueops execution status --id "$ISSUEOPS_ID" --json
   ```

   released임을 확인하기 전에는 새 세션을 시작하지 않는다. release 실패를 revoke나
   worktree 삭제로 우회하지 않는다. release 후 기존 세션은 구현하거나 다시 claim하지 않는다.
6. 자동 `new-session`이면 추가 질문 없이 **기존 worktree에** 새 세션 하나만 연다.
   현재 native host를 유지하고 현재 모델·effort는 해당 launch가 지원하는 값만 전달한다.
   사용자가 직접 세션을 열겠다고 명시한 경우에만 경로와 인계문을 제공하고 종료한다.
   Orca에서는 설치된 `orca-cli` 안내로 exact worktree 경로를 확인한 뒤 `terminal create`와
   일회성 prompt 전달을 사용한다. Herdr는 아래 **Herdr 실행** 절을 따른다. raw launcher를
   호출하기 직전과 receipt를 읽은 직후에는 아래 producer를 호출한다. staged 관측 기록이
   실패하면 launcher를 호출하지 않는다. 호출 뒤 관측 기록이 실패하면 같은 입력을 다시
   보내지 않고 기존 terminal/session을 확인한다. `worktree create`, `switch-mode`, coordinator
   task 생성은 이 인계의 수단이 아니다.
7. 실행 결과가 모호하면 새 세션을 또 띄우지 않고 기존 terminal/session을 확인한다.
   인계 전달을 확인하면 원래 세션은 종료 보고한다. 구현 완료를 기다리는 감독 루프를 만들지 않는다.

Orca 1.4.200 이상에서 인계 전달을 관측할 때 IssueOps operation ID와 Orca mutation request ID를
혼동하지 않는다. `newExecutionOperationID()`는 IssueOps intent CAS lineage이고 Orca의
`--retry-request` 값이 아니다. 설치 번들의 `shared/orchestration-retry-request-id.js`는
`--retry-request`를 UUID로 검증하며, “Orca reported for the original request; omit to start a new request”를
계약으로 둔다. 따라서 첫 `orca orchestration dispatch`와 첫 `orca terminal send`에는
`--retry-request`를 붙이지 않는다. 성공 응답의 `mutation.requestId` 또는 ambiguous error의
`data.orchestrationRequestId`로 받은 UUID만 같은 외부 호출의 read-only recovery에서 재사용한다.

Omo 인계는 durable 외부 호출이 두 개다. `orchestration dispatch --return-preamble --inject=false`가
반환하는 dispatch mutation UUID와, 그 preamble을 전달하는 `terminal send` prompt UUID는 서로 다른
identity다. dispatch UUID를 terminal send의 `--retry-request`로 넘기지 않고, terminal prompt UUID를
dispatch recovery에 쓰지 않는다. dispatch 성공은 preamble 생성 증거일 뿐 Omo 입력 수락이 아니다.
Omo 입력 수락은 `terminal send`의 prompt receipt(`requestId`, `stages`, `provider`, `observation`,
`processIncarnation`, `generation`, `baselineWorkingSequence`)로만 기록한다. `turn_started` 같은
native turn 관측과 IssueOps owner claim은 여전히 별도 상태이며, claim/status는 IssueOps claim CAS만
바꾼다.

`orca orchestration request-show --request <uuid> --json`은 read-only evidence다. 설치 번들의
`handlers/orchestration/mutation-request-show-handler.js`와 `shared/orchestration-mutation-request.js`는
top-level `requestId`, `state`(`completed|pending|absent`), `method`, `interpretation`을 출력한다.
`completed`는 Orca가 같은 UUID replay를 idempotent하게 처리한다는 증거이고, `pending`·`absent`도
새 IssueOps owner나 자동 retry 권한이 아니다. IssueOps handoff delivery observation은 folded evidence로
현재 lineage/prompt/material/runtime/generation과 충돌하는 recovery를 fail-closed시키지만, blind retry,
새 owner claim, status promotion을 승인하지 않는다.

### raw launcher 전송 관측

direct execution에서 Orca나 Herdr를 호출할 때는 `issueops trace handoff-delivery`를 공통
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
  "launcher": {"name": "<orca|herdr>", "version": "<observed>", "path": "<absolute observed path>", "runtime_id": "<observed>", "machine_id": "<observed>", "server_id": "<observed target identity>"},
  "target": {"terminal_id": "<observed>", "pane_id": "<observed>"},
  "expected_owner_host": "<codex|claude|omo>",
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

receipt를 받은 뒤 같은 attempt/lineage/`created_at`으로 두 번째 observation을 기록한다. Orca는
반환된 terminal prompt UUID만 `request.durable_id`에 넣고 실제 `processIncarnation`도 보존한다.
launcher가 제공하는 process 조회로 수신 agent의 PID, 시작 시각, executable을 확인해
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
resume 명령과 읽기 전용 확인 명령: <exact commands>
사용자 선택 기록: status.decisions의 <created_at>, 제목 "실행 방식 선택"
선택: new-session. 런처: <orca|herdr>. 결정 근거: <사용 가능 관찰 또는 명시적 지시>
원래 사용자 요청: <actual request and conversation reference>
승인 범위와 종료점: <scope>, <draft PR/MR publication + execution complete, or narrower endpoint>
기존 holder는 release를 마쳤습니다. 현재 status와 선택 기록, 현재 HEAD, 계획 digest,
인계 자료 digest를 읽고 인계 내용과 대조하세요. stale하거나 누락된 근거는 필요한 범위만 다시 확인하세요.
같은 worktree에서 next --id가 제공하는 복구 명령 체인을 따라 자기 native actor로 인수하세요.
direct의 released 상태는 replace preview부터 시작하며 이후 반환된 exact next_command를 따릅니다.
execution resume은 Orca binding 전용이므로 direct에 쓰지 마세요. 현재 generation을 직접 관측하세요.
독립 direct claim은 자료와 status 검증을 요구하지만 Orca owner packet을 요구하지 않습니다.
서로 다른 host의 session ID는 이식 가능한 identity가 아니므로 새 host/session은 자기 native actor flags로 식별하세요.
active(self)가 된 뒤 승인된 범위 안에서 재질문 없이 이어가세요. 자동 인계를 다시 적용해 새 세션을 띄우지 마세요.
기록보다 최신 사용자 지시가 우선합니다.
```

계획·범위·작업 경로가 선택 기록과 달라졌으면 원인을 조사한다. 다른 사이클의 승인이나
phase/claim 성공으로 승인 범위를 넓히지 않는다. 승인 근거를 확인할 수 없을 때만 필요한
결정을 묻는다. core의 actor·generation·fingerprint 검사는 그대로 통과해야 한다.

### Herdr 실행

이 절은 위 direct release가 확인된 뒤에만 실행한다. Herdr는 세션 배치만 맡으며
IssueOps lease나 `direct|orca` mode를 소유하지 않는다. 실제 Herdr 호출 전후에는 위
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

   - Claude·Codex: `herdr agent start "$AGENT_NAME" --kind "$HOST" --pane "$PANE_ID"
     --timeout 60000`을 사용한다. 이름은 `agent list`와 대조한 고유한 이름이고,
     모델·effort 인자는 해당 호스트의 설치된 help에서 확인한 경우에만 `--` 뒤에 붙인다.
     성공 후에도 `herdr agent read "$AGENT_NAME" --source visible`로 입력창을 확인한 뒤
     `herdr agent prompt "$AGENT_NAME" "$HANDOFF"`로 한 번 전달한다.
   - Omo: Herdr 0.9.0의 kind 목록에는 `omo`가 없다. `pi`나 `omp`, Claude로 대체하지
     않는다. `omo --model … --thinking … -- "$HANDOFF"`처럼 설치된 CLI가 지원하는
     **새 interactive 세션과 초기 프롬프트**를 하나의 shell 명령으로 구성하여
     `herdr pane run "$PANE_ID" "$LAUNCH_COMMAND"`으로 한 번 실행한다.
     실행 파일·모델·프롬프트를 각각 shell-quote하며 프롬프트의 따옴표·개행·`$`·backtick을
     shell이 평가하지 못하게 한다. `--continue`, `--resume`, `--print`는 사용하지 않는다.
     `pane run` 성공은 입력 제출 영수증일 뿐이다. `pane read --source visible`,
     foreground process와 Omo native 세션 기록으로 새 세션·cwd·인계 수신을 확인한다.
     Herdr의 agent 감지가 없어도 native 기록으로 확인하며 감지를 성공으로 꾸미지 않는다.
3. Herdr의 `idle`/`interactive_ready`만으로 준비 완료라고 판단하지 않는다.
   0.9.0에서 Claude의 첫 MCP 선택 화면도 `idle`로 관측됐다. 실제 화면의 질문을 읽고
   기존 승인 범위 안에서 처리한다. 필요 없는 선택 기능은 화면이 제공하는 건너뛰기를
   사용할 수 있지만 인증·신뢰·권한을 일괄 승인하지 않는다.
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

## 3 보류

`hold`를 기록한 뒤 새 세션과 같은 release/status 절차로 권한을 해제한다. branch·worktree·
계획·이슈를 그대로 두고 ID와 경로를 보고한다. 세션을 띄우거나 구현을 승인된 것으로 간주하지
않는다. 사용자가 재개하면 같은 ID를 복구하고 최신 지시와 환경별 자동 세션 인계 규칙을 따른다.
