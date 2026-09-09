# 준비된 워크트리에서 자동 세션 인계

[IssueOps](../SKILL.md)의 인계 지점에 도달했거나 인계받은 새 세션만 이 문서를
읽는다. CLI의 `direct|orca`는 workspace/lease 운영 방식이다. 새 native 세션을 여는 것만으로
mode를 바꾸지 않는다. 일반 흐름은 direct로 만든 **같은 worktree**를 유지한다.

## 환경 판별과 승인 근거 기록

실측한 lifecycle ID, issue URL, branch, worktree 절대경로, 계획과 성공 기준을 확인한다.
이미 인계받은 세션은 기존 기록과 승인 범위를 대조한 뒤 이어간다. 환경 판별부터 다시
시작해 새 세션을 연쇄 생성하지 않는다.

준비 세션은 설치된 `orca-cli` 안내로 실행 파일을 정하고 `orca status --json`을 확인한다.
`runtime.state == "ready"`면 Orca의 `new-session`을 선택한다. Orca가 없거나
unready인 것이 확인됐을 때만 Herdr를 확인한다.

```bash
command -v herdr
herdr status
herdr worktree open --help
herdr pane run --help
```

Herdr는 실행 중인 서버가 있고 client/server endpoint가 호환되며, 기존 worktree를
열고 **현재 native host**를 실행할 수 있을 때 `new-session`을 선택한다. Claude·Codex는
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

결정 결과와 원래 요청의 종료점을 짧게 알리고 아래 기록을 남긴다. 자동 결정은 세션
배치만 정하며 사용자 요청에 없던 commit·push·publication 권한을 만들지 않는다.

```bash
issueops decision add --id "$ISSUEOPS_ID" --kind implementation \
  --title "실행 방식 선택" --body "$CHOICE_RECORD" \
  --rationale "환경별 자동 세션 인계 규칙 또는 명시적 사용자 지시" \
  $RECORD_ACTOR_FLAGS --json
```

`CHOICE_RECORD`에는 `current|new-session|hold`, 자동 결정인지 명시적 지시인지,
선택한 런처(`orca|herdr|none`)와 확인한 status·host 실행 가능 관찰값(없으면 부재 근거),
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

1. 인계할 내용을 먼저 준비한다: 위 결정의 created_at과 내용, exact ID·issue·branch·worktree,
   계획 경로, 현재 단계·성공 기준·승인된 종료점. 인계문에 secret이나 claim token은 넣지 않는다.
2. 기존 holder가 canonical worktree에서 최신 `whoami`의 actor flags로 release한다.

   ```bash
   issueops execution release --id "$ISSUEOPS_ID" --generation "$GENERATION" $ACTOR_FLAGS --json
   issueops execution status --id "$ISSUEOPS_ID" --json
   ```

   released임을 확인하기 전에는 새 세션을 시작하지 않는다. release 실패를 revoke나
   worktree 삭제로 우회하지 않는다. release 후 기존 세션은 구현하거나 다시 claim하지 않는다.
3. 자동 `new-session`이면 추가 질문 없이 **기존 worktree에** 새 세션 하나만 연다.
   현재 native host를 유지하고 현재 모델·effort는 해당 launch가 지원하는 값만 전달한다.
   사용자가 직접 세션을 열겠다고 명시한 경우에만 경로와 인계문을 제공하고 종료한다.
   Orca에서는 설치된 `orca-cli` 안내로 exact worktree 경로를 확인한 뒤 `terminal create`와
   일회성 prompt 전달을 사용한다. Herdr는 아래 **Herdr 실행** 절을 따른다.
   `worktree create`, `switch-mode`, coordinator task 생성은
   이 인계의 수단이 아니다. 직접 실행 가능한 launch 기능이 없으면 경로와 인계문을 제공하고
   수동 시작이 남았다고 알린다. 현재 세션에서 몰래 구현하거나 실행됐다고 보고하지 않는다.
4. 실행 결과가 모호하면 새 세션을 또 띄우지 않고 기존 terminal/session을 확인한다.
   인계 전달을 확인하면 원래 세션은 종료 보고한다. 구현 완료를 기다리는 감독 루프를 만들지 않는다.

인계문에는 다음 내용을 실제 값으로 채운다.

```text
같은 IssueOps 사이클을 이어서 수행하세요.
ID: <lifecycle ID>
worktree: <absolute path>
issue / branch / plan: <verified values>
사용자 선택 기록: status.decisions의 <created_at>, 제목 "실행 방식 선택"
선택: new-session. 런처: <orca|herdr>. 결정 근거: <사용 가능 관찰 또는 명시적 지시>
원래 사용자 요청: <actual request and conversation reference>
승인 범위와 종료점: <scope>, <draft PR/MR publication + execution complete, or narrower endpoint>
기존 holder는 release를 마쳤습니다. 현재 status와 선택 기록을 읽고 인계 내용과 대조하세요.
같은 worktree에서 next --id가 제공하는 복구 명령 체인을 따라 자기 native actor로 인수하세요.
direct의 released 상태는 replace preview부터 시작하며 이후 반환된 exact next_command를 따릅니다.
execution resume은 Orca binding 전용이므로 direct에 쓰지 마세요. 현재 generation을 직접 관측하세요.
active(self)가 된 뒤 승인된 범위 안에서 재질문 없이 이어가세요. 자동 인계를 다시 적용해 새 세션을 띄우지 마세요.
기록보다 최신 사용자 지시가 우선합니다.
```

계획·범위·작업 경로가 선택 기록과 달라졌으면 원인을 조사한다. 다른 사이클의 승인이나
phase/claim 성공으로 승인 범위를 넓히지 않는다. 승인 근거를 확인할 수 없을 때만 필요한
결정을 묻는다. core의 actor·generation·fingerprint 검사는 그대로 통과해야 한다.

### Herdr 실행

이 절은 위 direct release가 확인된 뒤에만 실행한다. Herdr는 세션 배치만 맡으며
IssueOps lease나 `direct|orca` mode를 소유하지 않는다.

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
