# 준비된 워크트리에서 실행 방식 선택

[IssueOps](../SKILL.md)의 선택 지점에 도달했거나 그 선택으로 시작한 새 세션만 이 문서를
읽는다. CLI의 `direct|orca`는 workspace/lease 운영 방식이다. 새 native 세션을 여는 것만으로
mode를 바꾸지 않는다. 일반 흐름은 direct로 만든 **같은 worktree**를 유지한다.

## 선택과 승인 기록

실측한 lifecycle ID, issue URL, branch, worktree 절대경로, 계획과 성공 기준 요약을 보여
준다. "현재 세션(기본 추천) / 같은 worktree의 새 세션 / 보류"를 묻고, 1·2번은 draft
PR/MR 발행·execution complete까지 허용한다는 점을 함께 설명한다. 이전 요청이 더 좁으면
그 종료점을 사용한다. 사용자의 답을 받은 뒤에만 아래 결정 기록을 쓴다.

```bash
issueops decision add --id "$ISSUEOPS_ID" --kind implementation \
  --title "실행 방식 선택" --body "$CHOICE_RECORD" \
  --rationale "준비된 브랜치와 워크트리를 제시한 뒤 받은 사용자 선택" \
  $RECORD_ACTOR_FLAGS --json
```

`CHOICE_RECORD`에는 `current|new-session|hold`, 실제 사용자 답변과 그 대화 위치,
선택 시점의 lifecycle ID·issue URL·branch·worktree·계획 경로, 승인 범위·종료점과
현재 generation을 적는다. `hold`의 승인 범위는 보류다. 이 기록의 created_at과 내용을
`status --id`로 읽어 확인한다. 사용자 답변을 지어내거나 예시 값을 그대로 저장하지 않는다.
같은 답에 대한 기록이 이미 있으면 중복 기록하지 않는다. 취소·범위 수정 등 이후 지시가 우선한다.

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
3. 사용자가 직접 세션을 열기로 했으면 worktree 절대경로와 아래 인계문을 제공하고 종료한다.
   에이전트가 열기로 했고 현재 host가 이를 지원하면 **기존 worktree에** 새 세션 하나만 연다.
   Orca에서는 설치된 `orca-cli` 안내로 exact worktree 경로를 확인한 뒤 `terminal create`와
   일회성 prompt 전달을 사용한다. `worktree create`, `switch-mode`, coordinator task 생성은
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
선택: new-session. 사용자 답변: <actual reply and conversation reference>
승인 범위와 종료점: <scope>, <draft PR/MR publication + execution complete, or narrower endpoint>
기존 holder는 release를 마쳤습니다. 현재 status와 선택 기록을 읽고 인계 내용과 대조하세요.
같은 worktree에서 next --id가 제공하는 복구 명령 체인을 따라 자기 native actor로 인수하세요.
direct의 released 상태는 replace preview부터 시작하며 이후 반환된 exact next_command를 따릅니다.
execution resume은 Orca binding 전용이므로 direct에 쓰지 마세요. 현재 generation을 직접 관측하세요.
active(self)가 된 뒤 승인된 범위 안에서 재질문 없이 이어가세요. 기록보다 최신 사용자 지시가 우선합니다.
```

계획·범위·작업 경로가 선택 기록과 달라졌으면 원인을 조사한다. 다른 사이클의 승인이나
phase/claim 성공으로 승인 범위를 넓히지 않는다. 승인 근거를 확인할 수 없을 때만 필요한
결정을 묻는다. core의 actor·generation·fingerprint 검사는 그대로 통과해야 한다.

## 3 보류

`hold`를 기록한 뒤 새 세션과 같은 release/status 절차로 권한을 해제한다. branch·worktree·
계획·이슈를 그대로 두고 ID와 경로를 보고한다. 세션을 띄우거나 구현을 승인된 것으로 간주하지
않는다. 사용자가 재개하면 같은 ID를 복구하고 실행 방식을 선택받는다.
