# Parent와 child 분할 절차

`no split`이 아닌 경우에만 읽는다. split 조건은 SKILL.md의 `## Parent와 child`에 있다.

| 항목 | 규칙 |
|---|---|
| 실행 class | `[p]` 기본. `[s]`는 이름 있는 hard dependency가 있을 때만 |
| `[p]` | prerequisite `none`, 독립 검증, 보통 wave 1 |
| `[s]` | 선행 child URL/산출물과 순서를 반드시 명시 |
| 생성 | ordinary sibling이 아닌 `remote create-child` |
| parent 기록 | `## 하위 Task` 아래 URL·scope·wave·prerequisite를 본문에 기록 |

닫힌 parent를 재사용하지 않는다. parent body를 안전하게 갱신할 IssueOps 경계가 없으면
raw `gh`/`glab`로 우회하지 말고 중지한다. 댓글만 남기는 것은 완료가 아니다.
GitHub 관련 이슈는 body cross-reference, GitLab은 native linked item이다.
child는 양쪽 모두 native hierarchy를 확인하며 sibling/body 목록으로 대체하지 않는다.
`[p]`/`[s]`, prerequisite, wave는 child 제목·본문·parent 하위 Task 절에 모두 남긴다.

## child 생성

child는 parent URL이 record에 연결되고 umbrella branch gate가 통과한 뒤 만든다. 이 gate는
parent의 branch prepare(원격 branch 생성·연결 포함)다. 구현 없이 child만 만드는 요청도 이
단계를 먼저 거친다. create-issue와 달리 create-child는 record 명령이라 `$RECORD_ACTOR_FLAGS`를 붙인다.

```bash
issueops remote create-child --id "$ISSUEOPS_ID" --title "$CHILD_TITLE" \
  --template child_task --body-file "$CHILD_BODY" \
  --label "$LABEL" --assignee "$ASSIGNEE" $RECORD_ACTOR_FLAGS --json
```

같은 본문으로 의도적인 새 child를 만들려면 ID(32자 소문자 hex, 예: `openssl rand -hex 16`)를
한 번 생성해 저장하고 최초 요청과 재시도에 같은 값을 `--operation-id`로 넘긴다.

무ID 재시도는 완료 뒤에도 최초 implicit child를 반환한다. 같은 본문의 explicit 새
child가 생겨도 이 결합은 유지된다. 미해결 operation은 새 ID로도 생성하지 않으며,
오류 응답의 known URL·operation ID·recovery command를 보존한다.
`remote reconcile-child --id ID --operation-id OPERATION`의 preview에서 제목·본문
digest·type·계층·metadata를 확인한 뒤 같은 요청에 `--confirm`을 붙인다. 현재
execution holder만 confirm할 수 있고, generation이 바뀐 작업은 자동 replay 대신
명시적 reconcile로 복구한다. 복구는 기존 child를 조회·연결하며 생성하지 않는다.
호출 프로세스가 죽어 후보가 0개인 `pending` 작업은 2분이 지나고 생성 프로세스가 없으면
reconcile이 풀어 준다(`released: true`). 그 뒤 같은 요청을 다시 실행한다(explicit이면 같은 `--operation-id`).
2분 전에는 오류에 대기 조건이 나온다. 기다린 뒤 reconcile을 다시 실행한다.

child 본문의 요약에는 parent 링크와 이 child가 맡는 부분을, 선행 조건과 병합 조건
절에는 `[p]`/`[s]`, prerequisite, wave, 병합 조건을 쓴다. confirmed child 결과의
`hierarchy_verified`, type, URL, labels, assignee와 parent body readback을
확인한다.
