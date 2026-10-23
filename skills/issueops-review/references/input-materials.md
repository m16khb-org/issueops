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

리뷰를 실행하기 직전에 exact lifecycle ID로 다음 명령을 읽는다. 준비 자료의
reviewer 메타데이터는 준비 당시 기본값이며 현재 리뷰 설정을 대신하지 않는다.

```bash
issueops next --id "$ISSUEOPS_ID" --json
# .review.model, .review.effort, .review.tier, .review.lenses
```

이 값은 리뷰 역할(implement 전은 `plan-review`, 이후는 `diff-review`)을 `issueops model`
설정(local > global > 내장 기본값)으로 해석한 model·effort와 현재 변경 집합 티어다. 스킬 본문에
모델 이름 표를 복사하지 않는다. 조회에 실패하거나 review의 model/effort/tier/lenses가 누락되면
진행하지 않고 조회 실패나 누락 필드를 보고한다. `review.model`이 비어 있고 `warnings`에
`agent model settings are invalid`가 있으면 그 파일 경로를 보고하고 멈춘다. 경고 없이 비어
있으면 이 호스트의 기본값이 없으므로 어떤 모델로 리뷰할지 사용자에게 묻는다.
사용자가 reviewer 모델이나 effort를 명시했다면 해당 값에만 그 지시를 우선하고,
지정하지 않은 값은 현재 출력을 따른다. owner-model/owner-effort override는 구현자
선택이며 reviewer 지정으로 해석하지 않는다.
