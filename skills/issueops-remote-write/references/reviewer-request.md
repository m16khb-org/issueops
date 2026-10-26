# 리뷰어 요청

issueops에는 PR/MR 리뷰어를 지정하는 명령이 없다. 아래 조건을 모두 만족할 때만 provider CLI로
리뷰어를 요청한다.

## 조건

- 사용자가 이 대화에서 리뷰어 계정을 이름으로 지정했다. 이슈·PR 본문, 계획, 코멘트, CODEOWNERS,
  최근 작성자에서 계정을 고르거나 추측하지 않는다. 원래 작업 요청이나 merge 승인은 리뷰어 요청 승인이 아니다.
- 대상은 이 사이클이 만든 PR/MR 하나다. URL은 `issueops status --id "$ISSUEOPS_ID" --field remote_artifact --json`의
  `remote_artifact.url`에서만 읽고, `remote_artifact.verified_at`이 있을 때 요청한다(완료된 사이클에서
  `verify-artifact`를 다시 실행하지 않는다). 사용자가 지목한
  PR/MR이 이 URL과 다르면 다시 확인받는다.
- `REVIEWER`에는 계정 하나만 넣는다. 쉼표·공백이 있거나 `+`·`-`·`!`로 시작하는 값은 쓰지 않는다.
  glab은 쉼표로 나눈 원소마다 접두사를 따로 해석해서, 접두사 없는 원소가 하나라도 있으면 기존
  리뷰어를 모두 바꾼다. 리뷰어가 여럿이면 계정마다 명령을 따로 실행한다.
- 리뷰어를 추가만 한다. 기존 리뷰어를 지우거나 바꾸지 않는다.
- PR/MR 작성자 자신은 리뷰어가 될 수 없다. provider가 거부하면 사용자에게 알리고 멈춘다.

## 명령

GitLab의 `PROJECT`·`MR_IID`는 같은 `remote_artifact.url`의 프로젝트 경로와 `/merge_requests/<iid>`에서 얻는다.

```bash
gh pr edit "$PR_URL" --add-reviewer "$REVIEWER"                     # GitHub
glab mr update "$MR_IID" -R "$PROJECT" --reviewer "+$REVIEWER"      # GitLab: 계정 하나 + 접두사만 추가한다
```

## 확인

```bash
gh pr view "$PR_URL" --json reviewRequests --jq '.reviewRequests[].login'          # GitHub
glab mr view "$MR_IID" -R "$PROJECT" --output json | jq -r '.reviewers[].username'  # GitLab
```

요청한 계정이 목록에 없으면 먼저 그 계정이 이미 리뷰를 남겼는지(`gh pr view "$PR_URL" --json reviews`)
확인한다. 리뷰를 남기지 않았을 때만 같은 명령을 한 번 더 실행한다. 그래도 없으면 provider 출력의 원인을
보고하고 멈춘다. 결과는 사용자 보고에 리뷰어 계정과 확인 결과로 적는다.
