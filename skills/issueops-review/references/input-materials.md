# 리뷰 입력 자료 준비

`SKILL.md`의 `## 입력`에서 호출한다. 리뷰어에게 줄 자료를 모으는 절차 전문이다.

| target | 리뷰 대상 | 함께 주는 자료 |
|---|---|---|
| `architecture` | staged `architecture` artifact 전문(`artifact stage --name architecture`로 올린 것) | 이슈 본문, intent contract의 성공 기준, 관련 문서 발췌 |
| `plan` | `status --json`의 `plan_path` 파일 전체(링크 전이면 staged plan artifact) | 이슈 본문, intent contract의 성공 기준, 승인된 아키텍처 문서 전문 |
| `diff` | `git -C "$WORKTREE" diff "$BASE_SHA"` 전체 | plan 파일 전체, 봉인 intent 문서(`<artifact_dir>/intent.md`, 없으면 `status --json`의 `.intent`), 검증 명령과 결과, 변경한 프로젝트 문서 |

리뷰가 "무시된 결정"을 잡으려면 계획이 인용한 항목만으로는 부족하다. plan과 diff 모두 리뷰를
띄우기 전에 `.issueops/ADR.md`, `.issueops/CAUTIONS.md`, `.issueops/ARCHITECTURE.md` 세 색인을 직접 열어
변경 모듈과 관련된 항목을 고르고, 그 항목이 가리키는 모듈·기록 파일을 연다. `project_docs_route`에
대상 요약을 넣어 나온 dated `cautions/`·`adr/` 기록은 여기에 **더한다**. route 결과가 세 색인을
대신하지 않는다. diff 리뷰에는 `.issueops/CONVENTIONS.md`와 `.issueops/TESTING.md`를 더하고, 두 색인의
모듈 표가 변경 영역(예: 변경 언어의 기본 검증, CLI·JSON 출력 계약)을 넘기는 모듈 파일도 연다.
grep으로 색인의 한 줄만 본 것은 연 것이 아니다.

모든 변경에 걸리는 원칙(`.issueops/CONSTITUTION.md`)도 연다. 대상이 원칙에 걸리는 동작(반복·재시도·감시
루프, secret이 지나는 경로, 권한·실행 경계, 결함·회귀 수정)을 담으면, 그 원칙 문장을 리뷰어 프롬프트의
문서 발췌에 넣는다. 결함·회귀 수정이면 문제 해결 원칙의 근본 원인 두 문장
(`.issueops/CONSTITUTION.md:69-70`)을 넣는다.

리뷰 모델·effort·tier·lenses는 `SKILL.md`의 `## 입력`대로 리뷰 직전 `issueops next --id`에서 읽는다.
