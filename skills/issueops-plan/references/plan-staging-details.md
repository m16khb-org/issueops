# 계획 작성과 스테이징 세부 규칙

## 인계받는 세션을 위한 기록

계획에 lifecycle ID, 사용자 요청 범위, 브랜치·worktree 준비 뒤의 환경별 자동 세션 인계,
승인된 종료점을 적어 인계받는 세션도 같은 경계를 알게 한다. 자동 인계는 사용자 요청 범위를
넓히지 않는다. 이미 읽은 plan-prep 조사와 문서는 변경이나
새 질문이 없으면 재사용한다. 작업 규모에 맞춰 필요한 검증 명령을 정하고, 같은 명령을
게이트와 별도 검증 목록에 중복 등록하지 않는다.

## artifact stage 인수

`artifact stage`는 actor 플래그를 받지 않는다. `--id`, `--name`, `--file`, `--json`뿐이다.
잘못 올렸으면 `issueops artifact unstage --id "$ISSUEOPS_ID" --name plan --json`으로
내린다.
