# #31 remote verify-artifact 결과에 실패 사유 남기기

## 문제
`issueops remote verify-artifact`가 label·assignee·target branch 불일치로 실패하면 record의
`remote_artifact` 검증 결과에는 성공 시각만 남고 무엇이 어긋났는지가 없다. 다음 세션은 같은 명령을 다시
실행해 봐야 원인을 안다.

## 제안 (이슈 작성자 메모)
- 검증 결과 DTO(`IssueOpsRemoteArtifactVerification`)에 `failureReason` 필드를 추가해 실패 사유를 남긴다.
- 프론트엔드 대시보드가 camelCase를 쓰므로 기존 `verified_at`도 `verifiedAt`으로 이름을 바꿔 맞춘다.
- 필드가 늘어나니 record의 `schema_version`을 2로 올린다.

## 성공 기준
- verify-artifact가 실패하면 `issueops status --json`에서 어떤 검사(label/assignee/target_branch)가 왜
  실패했는지 읽을 수 있다.
- 성공한 검증 결과는 지금과 같다.

## 범위 밖
- 대시보드 자체 변경.
