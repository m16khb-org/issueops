# 발행 세부 규칙

`SKILL.md`의 `## 기록 순서`와 `## Canonical publication`이 가리키는 세부 규칙이다. 본문은 기록 순서와
발행 단계 번호, 중복 생성 금지, readback 조건을 소유하고, 이 문서는 라벨 판단과 기존 이슈 처리를 소유한다.

## 라벨 판단 (발행 1·2단계)

현재 provider 이슈·label 후보를 조사한다. 일치하는 이슈가 있으면 중복 생성 대신
연결/갱신한다. score의 기본 threshold는 0.70(더 강한 repo/user 값 우선)이며 선택된
label·관련 이슈만 적용한다. 없는 label도 선택된 것만 먼저 만든다. 미선택 label을
적용하려면 근거 있는 override를 decision/feedback에 기록한다. 무라벨 발행은 하지 않는다.
모든 후보가 threshold 미만이거나 `abstained`이면, 사용자가 말한 유형("버그로", "기능 추가")이나
요청 내용에 맞는 label 하나를 고르고 그 근거를 같은 decision에 override로 적는다.

deterministic score 뒤 semantic 판단이 필요하면 `remote score --judge prompt`를
fresh 독립 read-only agent에게 주고 결과 JSON을 `--judge file --judge-file`로 검증한다.
독립 agent가 없거나 비활성화했으면 deterministic 결과를 쓰고 그 선택을 기록한다.

## link-issue와 기존 이슈

`remote create-issue` 뒤 `status --json`에 `issue_url`이 없을 때만
`issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" $RECORD_ACTOR_FLAGS --json`을 실행한다.
사용자가 기존 이슈(번호나 URL)를 처리하라고 하면 새로 만들지 않는다. 그 이슈를 읽어 intent를 기록하고
같은 `link-issue`로 붙인다. 기존 이슈의 라벨은 바꾸지 않는다. score가 고른 라벨은 PR의 `--label`에 쓰고 보고한다. 본문에 완료 기준이 없거나 범위가 바뀌면 [`issueops-sync-issue`](../../issueops-sync-issue/SKILL.md)로 갱신한다.

## 관련 이슈·라벨 점수 입력

입력 JSON 형식은 `issueops remote score --example`이 출력한다. 후보는 provider의 열린 이슈·라벨 목록이다.
issueops에는 목록 조회 명령이 없다. gh issue list / gh label list(GitLab은 glab)로 읽기만 한다.
선택하거나 거절한 라벨과 threshold는 본문이 아니라 record에 남긴다.
