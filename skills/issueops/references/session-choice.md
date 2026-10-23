# 준비된 워크트리에서 자동 세션 인계

[IssueOps](../SKILL.md)의 인계 지점에 도달했거나 인계받은 새 세션만 이 문서를
읽는다. 새 native 세션을 여는 것만으로 workspace나 lease가 바뀌지 않는다. 일반 흐름은
준비한 **같은 worktree**를 유지한다.

## 환경 판별과 승인 근거 기록

실측한 lifecycle ID, issue URL, branch, worktree 절대경로, 계획과 성공 기준을 확인한다.
이미 인계받은 세션은 기존 기록과 승인 범위를 대조한 뒤 이어간다. 환경 판별부터 다시
시작해 새 세션을 연쇄 생성하지 않는다.

브랜치·worktree 준비 뒤 Herdr를 확인한다. `HERDR_ENV=1`이고
`HERDR_SOCKET_PATH`가 있으면 Herdr 세션이다. env는 먼저 확인할 런처만 정한다. 사용 가능
여부는 아래 Herdr 확인 명령과 조건으로 실측해 판단하고, 대상은 그 세션의
`HERDR_SOCKET_PATH`가 가리키는 서버로 고정한다. 다른 서버를 찾거나 시작하지 않는다. 사용
가능하면 Herdr의 `new-session`을 선택한다. Herdr 세션이 아니거나 Herdr가 사용 불가임이
확인됐으면 Herdr를 다시 확인하지 않는다.
인계받은 새 세션도 같은 env를 갖지만 환경 판별을 다시 시작하지 않는다.

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

Herdr를 쓸 수 없음이 확인됐으면 현재 세션에서 이어가는 `current`다. 사용자가 직접 연 두 번째
터미널에서 이어가길 원하면 같은 release와 인수 체인으로 수동 인계한다. 바이너리 설치 여부만으로
ready라고 판단하지 않는다. 외부 도구를 자동 설치하거나
서버를 시작하지 않는다. 설치·업데이트·self-verify의 필수 조건으로도 추가하지 않는다.
status 오류로 사용 가능 여부를 판별할 수 없으면 오류를 보고한다.
이미 시작한 open·launch의 실패·모호함을 도구 부재로 해석해 다른 런처로 재실행하거나
현재 세션에서 구현하지 않는다.
사용자가 현재 세션·새 세션·보류를 명시했으면 그 지시가 우선한다.

### 사용자가 명시한 cmux 인계

cmux는 위 자동 선택에 참여하지 않는다. 사용자가 cmux 사용을 명시했을 때만
[session-choice-cmux.md](session-choice-cmux.md)를 읽고 그 절차를 따른다.

결정 결과와 원래 요청의 종료점을 짧게 알리고 아래 기록을 남긴다. 자동 결정은 세션
배치만 정하며 사용자 요청에 없던 commit·push·publication 권한을 만들지 않는다.

```bash
issueops decision add --id "$ISSUEOPS_ID" --kind implementation \
  --title "세션 인계 선택" --body "$CHOICE_RECORD" \
  --rationale "환경별 자동 세션 인계 규칙 또는 명시적 사용자 지시" \
  $RECORD_ACTOR_FLAGS --json
```

`CHOICE_RECORD`에는 `current|new-session|hold`, 자동 결정인지 명시적 지시인지,
선택한 런처(`herdr|none`, 명시적 지시일 때만 `cmux|manual`)와 확인한 status·host 실행 가능 관찰값(없으면 부재 근거),
원래 사용자 요청과 그 대화 위치,
선택 시점의 lifecycle ID·issue URL·branch·worktree·계획 경로, 승인 범위·종료점과
현재 generation을 적는다. `hold`의 승인 범위는 보류다. 이 기록의 created_at과 내용을
`status --id`로 읽어 확인한다. 사용자 답변을 지어내거나 예시 값을 그대로 저장하지 않는다.
같은 인계에 대한 기록이 이미 있으면 중복 기록하지 않는다. 취소·범위 수정 등 이후 지시가 우선한다.

## 1 현재 세션

기존 holder와 generation을 유지한다. 도구의 작업 경로를 canonical worktree로 맞추고
`next --id`가 가리키는 단계로 이어간다. release와 새 세션 생성은 하지 않는다.

## 2 새 세션

새 세션은 Herdr가 열거나 사용자가 직접 연 두 번째 터미널에서 시작한다. 어느 경로든 기존 세션이
release로 lease를 해제하고, 새 세션이 자기 native actor로 `execution replace --id ID
--expected-generation N --preview`를 실행한 뒤, 반환된 `--reseed` 명령과
`claim --claim-current-token`으로 인수한다. 아래 번호 순서는 이 체인 앞의 준비 절차다.

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
   **현재 lifecycle 상태**, exact **인수 명령과 읽기 전용 확인 명령**, 그리고 인계 자료
   digest다. 자료에는 secret, 인증 정보, 이전 lease token을 넣지 않는다.
4. 위 결정의 created_at과 내용, exact ID·issue·branch·worktree, 계획 경로, 현재 단계·성공 기준·
   승인된 종료점을 인계문에 실제 값으로 채운다. 인계문에 secret이나 claim token은 넣지 않는다.
   수신자는 **현재 HEAD, 계획 digest, 인계 자료 digest**를 대조하고, 일치하는 근거만
   재사용한다. stale하거나 누락된 근거는 필요한 범위만 다시 확인한다. 서로 다른 host의
   session ID는 이식 가능한 identity가 아니다. 새 host/session은 durable actor flags와 runtime
   receipt로 다시 식별한다. **독립 claim**은 이 자료와 status를 검증한다.
5. 기존 holder가 canonical worktree에서 최신 `whoami`의 actor flags로 release한다.

   ```bash
   issueops execution release --id "$ISSUEOPS_ID" --generation "$GENERATION" $ACTOR_FLAGS --json
   issueops execution status --id "$ISSUEOPS_ID" --json
   ```

   released임을 확인하기 전에는 새 세션을 시작하지 않는다. release 실패를 revoke나
   worktree 삭제로 우회하지 않는다. release 후 기존 세션은 구현하거나 다시 claim하지 않는다.
6. 자동 `new-session`이면 추가 질문 없이 **기존 worktree에** Herdr 세션 하나만 연다.
   현재 native host를 유지하고 현재 모델·effort는 해당 launch가 지원하는 값만 전달한다.
   Claude Code를 여는 명령에는 `--dangerously-skip-permissions`, Codex를 여는
   명령에는 `--dangerously-bypass-approvals-and-sandbox`, Omo를 여는 명령에는
   `--permission-preset full-access`, omp를 여는 명령에는 `--auto-approve`, opencode를 여는 명령에는
   `opencode --standalone --auto`를 쓴다. 설치된 host의
   `--help`에서 플래그 지원을 확인한 뒤 호출한다. opencode는 공유 서비스 세션이 lease를 쥘 수
   없으므로 `--standalone`을 빼지 않는다.
   사용자가 직접 연 두 번째 터미널로 인계하겠다고 명시한 경우에만 경로와 인계문을 제공하고 종료한다.
   Herdr는 [session-choice-herdr.md](session-choice-herdr.md)의 **Herdr 실행** 절을 따른다. raw launcher를
   호출하기 직전과 receipt를 읽은 직후에는 같은 문서의 producer를 호출한다. staged 관측 기록이
   실패하면 launcher를 호출하지 않는다. 호출 뒤 관측 기록이 실패하면 같은 입력을 다시
   보내지 않고 기존 terminal/session을 확인한다. worktree를 다시 만드는 것은 이 인계의
   수단이 아니다.
7. 실행 결과가 모호하면 새 세션을 또 띄우지 않고 기존 terminal/session을 확인한다.
   인계 전달을 확인하면 원래 세션은 종료 보고한다. 구현 완료를 기다리는 감독 루프를 만들지 않는다.

### 인수 체인

인수 세션은 자기 native actor로 `next --id`를 실행하고 반환된 `next_command`를 따른다.
claimable이면 `claim --claim-current-token`, released이면 `execution replace --id ID
--expected-generation N --preview`, revoking이면 `execution replace --finalize-preview`다.
preview가 반환한 `--reseed` 명령을 실행한 뒤 `claim --claim-current-token`으로 마친다.
이 체인이 유일한 인계 경로다. Herdr 세션이든 사용자가 직접 연 두 번째 터미널이든 같다.

### Herdr 전송과 실행

Herdr `new-session`을 선택했으면 [session-choice-herdr.md](session-choice-herdr.md)의
raw launcher 전송 관측과 Herdr 실행 절을 따른다. 현재 세션이나 보류를 고른 세션은 그
문서를 읽지 않는다.

## 3 보류

`hold`를 기록한 뒤 새 세션과 같은 release/status 절차로 권한을 해제한다. branch·worktree·
계획·이슈를 그대로 두고 ID와 경로를 보고한다. 세션을 띄우거나 구현을 승인된 것으로 간주하지
않는다. 사용자가 재개하면 같은 ID를 복구하고 최신 지시와 환경별 자동 세션 인계 규칙을 따른다.
