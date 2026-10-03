# 이슈 작성 예시와 배경

[본문](../SKILL.md)의 계약을 설명하는 선택적 자료다. 실행에 필요한 규칙은 본문에 있다.

## 템플릿 근거

- [GitHub issue forms](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/syntax-for-issue-forms)는 field type과 required validation으로 입력 오류를 줄인다.
- [GitHub PR templates](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/creating-a-pull-request-template-for-your-repository)는 반복되는 리뷰 정보를 제공한다.
- [GitLab description templates](https://docs.gitlab.com/user/project/description_templates/)는 Issue와 MR에 Markdown을 사용한다.

GitHub Issue form과 GitLab Issue/PR/MR Markdown은 같은 읽기 순서를 유지한다.
필수 항목은 완료·검증에 필요한 것만 남긴다.

## 나쁜 입력

| 입력 | 개선 |
|---|---|
| `버그 고쳐주세요` | 재현 절차와 기대/실제 동작, 완료 기준을 적는다 |
| 파일 20개 목록 | 문제와 연결하고 범위·비목표를 나눈다 |
| 본문에 라벨 점수나 해시 | 라벨 판단은 decision, 해시는 record에 둔다 |
| `나중에 테스트` | 명령과 기대 결과를 적는다 |
| `task: 작업` | parent, `[p]`/`[s]`, prerequisite, wave를 적는다 |
| closed #18에 child 부착 | 활성 parent를 확인하거나 새 parent를 준비한다 |
| token이 든 로그 | secret을 지우고 최소 재현 출력만 남긴다 |
| worktree 안에서 `start` | source checkout에서 시작한다 |
| blocking 아닌 질문 반복 | 먼저 조사한다 |
| plan-prep를 waive로 채움 | 조사 결과나 불필요한 이유를 evidence로 남긴다 |

## 의도적인 새 child ID 예시

새 작업의 ID를 한 번 생성해 보관하고 최초 요청과 재시도에서 같은 값을 쓴다.

```bash
CHILD_OPERATION_ID="$(python3 -c 'import secrets; print(secrets.token_hex(16))')"
# remote create-child 요청의 --operation-id에 저장한 값을 사용한다.
```
