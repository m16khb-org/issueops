# 계획: 리뷰어 출력 계약과 PR 본문 규칙에 Claude Code 팀 리뷰 관행을 반영한다 (#566)

- lifecycle ID: `io-61996bf15223`
- branch: `566-review-output-contract-and-pr-risk-rules`, base `main` @ `6237393199944feb0d70d704e74e90dc9d2099a6`
- 사용자 요청 범위: 전체 IssueOps 사이클(구현 → 정리 → 문서 → 검증 → 커밋·푸시 → draft PR 발행 → execution complete). 머지와 정리는 별도 승인 단계다.
- 세션: 워크트리를 direct로 준비한 뒤 환경별 자동 세션 인계를 적용한다. 자동 인계는 위 범위를 넓히지 않는다.

## 목표와 비목표

목표는 이슈 #566의 완료 기준 여섯 가지다. 바꾸는 파일은 세 개다.

| 파일 | 변경 |
|---|---|
| `skills/issueops-review/SKILL.md` | (1) 결함 유형 전체 나열, (2) delta 리뷰의 직전 지적 상태 표시와 반복 금지, (3) 기존 결함 표시, (4) 버그 수정 사이클의 근본 원인 렌즈 |
| `skills/issueops-create-pr/SKILL.md` | (5) `review.tier`가 `schema-auth`·`contract`이면 `위험과 되돌리기` 절 필수 |
| `skills/issueops-sync-pr/SKILL.md` | (6) 낡은 본문 절 표를 PR 템플릿의 정규 제목으로 교체 |

비목표:

- CI 워크플로 경로의 change tier 분류(사용자가 제외).
- 구현 리뷰 이력 보존·diff 리뷰 라운드 상한의 Go 강제(record 형식 변경 필요, 별도 이슈).
- `artifacttemplate`의 `Required` 플래그와 `render-template` 동작 변경.
- 스킬 frontmatter `description` 변경. 바꾸면 response-contract golden이 바뀐다(`.issueops/cautions/issueops-lifecycle.md` 27절).
- `skills/issueops-sync-issue/SKILL.md:32`의 `## 위험과 트레이드오프`도 템플릿 제목(`위험`, `internal/domain/artifacttemplate/template.go:369,380`)과 어긋난다. 같은 유형의 결함이지만 이슈 범위 밖이라 PR 본문의 `남은 일`에 적고 고치지 않는다.
- `scripts/autoresearch` 정답 세트와 시나리오 추가. 측정은 저장소 밖 임시 spec으로 한다(아래 측정 계획).

## 변경 상세

### (1) 결함 유형 전체 나열 — `issueops-review` 출력 계약 4번

현재 4번은 필수 결함마다 "위치·발생 조건·위반한 계약·근거와 최소 수정 또는 확인 방법"을 요구한다(`skills/issueops-review/SKILL.md:86-88`). 같은 4번의 최소 수정 문단(`:98-101`) 뒤에 다음 규칙을 넣는다.

> 필수 결함마다 같은 유형의 다른 발생 위치를 대상 전체에서 찾아 같은 결함 안에 나열한다. delta 라운드에서도 대상 파일을 직접 열어 검색한다. 찾은 방법(검색한 패턴과 범위)을 함께 적고, 다른 위치가 없으면 "다른 위치 없음"과 그 근거를 적는다. 한 위치만 고친 수정이 다음 라운드에서 같은 결함으로 다시 걸리지 않게 하려는 것이다.

저자 측 점검(`:199-203`, 수정이 바꾼 옛 값을 대상 전체에서 검색)은 그대로 둔다. 리뷰어 규칙은 결함을 처음 낼 때, 저자 점검은 고친 뒤에 적용된다. delta 리뷰는 대상 전체를 다시 읽히지 않지만(`:194-196`) 이 검색은 대상 파일을 열어서 한다. delta만 보고 "다른 위치 없음"을 쓰지 않게 하려는 것이다.

### (2) delta 리뷰의 직전 지적 상태 — `issueops-review` 루프 규칙 첫 항목

현재 첫 항목은 delta 리뷰에 "직전 지적, 각 CHECK의 실행 결과, 대상의 delta, 영향받은 계약만" 넘긴다(`:194-198`). 같은 항목 끝에 다음을 더한다.

> 호출자는 다음 규칙을 delta 리뷰 프롬프트에 넣는다. delta 리뷰어는 직전 지적마다 해결됨 또는 남음만 판정하고, 남음이면 남은 위치와 이유를 적는다. 반증 자료로 철회된 지적은 해결됨으로 적고 반증 근거를 붙인다. 새 필수 결함은 delta와 영향받은 계약에서만 찾는다. 직전 라운드에 나온 지적을 다른 말로 다시 내지 않는다.

### (3) 기존 결함 표시 — `issueops-review` 출력 계약 4번

비차단 메모 규칙(`:89-93`) 뒤에 다음을 넣는다.

> 대상이 만들지 않았고 성공 기준과도 무관한 결함(`diff`면 base에 이미 있던 코드·문서, `plan`이면 계획이 건드리지 않는 기존 동작)은 기존 결함으로 표시해 비차단 메모에 둔다. 다만 이슈·intent의 성공 기준이 고치라고 한 결함, 결함 수정 사이클에서 그 결함의 근본 원인, 대상이 성공 기준을 지키려고 기대는 기존 동작은 기존 결함으로 내리지 않는다. 대상이 기존 결함을 새 경로로 노출하거나 악화시켰으면 그렇게 만든 대상의 줄을 근거로 필수 결함에 남긴다.

근거 원칙: `skills/design-review/SKILL.md:26` "Naming preferences, optional refactors, minor duplication, speculative future scale, and unrelated pre-existing debt do not block delivery." 차단에서 빠지는 것은 "unrelated" 부채뿐이고, 관련 결함에는 `:22`의 차단 조건이 그대로 적용된다. 이 규칙은 그 범위를 finding 분류로 옮길 뿐 차단 기준을 바꾸지 않는다. (4)와 같은 입력(원인을 둔 채 기대값만 완화한 diff)에서 두 규칙이 부딪히지 않도록 결함 수정 사이클의 근본 원인을 제외 조건에 넣었다.

### (4) 근본 원인 렌즈 — `issueops-review` 출력 계약 5번과 `## 입력`

5번(`:113-118`)의 끝, `frontend 추가 렌즈` 문장 뒤에 별도 문장으로 넣는다. `:116`의 "선택된 렌즈만 적용한다"와 붙여 놓으면 두 문장이 부딪히므로 티어와 상관없다는 점을 문장 안에 적는다.

> intent가 결함·회귀 수정이면 `diff`에 근본 원인 렌즈를 더한다. 이 렌즈는 `tier`와 `lenses` 목록과 상관없이 더한다. diff가 원인을 제거하는지, 증상만 덮는지(출력 문구 보정, 테스트 기대값 완화, 문서만 변경, 우회 분기) 판정한다. 증상만 덮었는데 대상·계획에 근본 원인이 이미 제거됐거나 현재 범위에서 물리적으로 제거할 수 없다는 근거가 없으면 필수 결함이고, 위반한 계약 칸에는 `.issueops/CONSTITUTION.md:70` 원문을 쓴다.

diff 리뷰 입력(`:25`)에는 이슈 본문이 없고 봉인 intent 문서만 있으므로 신호는 intent 하나로 한다. 판정 조건은 `CONSTITUTION.md:70`의 문구("근본 원인이 이미 제거됐거나 현재 범위에서 물리적으로 제거할 수 없다는 근거")를 그대로 쓴다. 범위를 골라 미룬 것은 근거가 아니다.

`## 입력`의 원칙 발췌 문장(`:35-36`)에 결함·회귀 수정을 추가해, 그 경우 `.issueops/CONSTITUTION.md:69-70`의 원칙 문장을 리뷰어 프롬프트에 넣게 한다. 렌즈의 판정 조건은 CONSTITUTION 원문이 소유하고, 스킬 문장은 그 원문을 옮겨 적용 시점만 정한다.

### (5) 위험과 되돌리기 필수 — `issueops-create-pr` 읽는 순서

선택 절 문단(`skills/issueops-create-pr/SKILL.md:37-39`)을 다음으로 바꾼다.

> 대안과 선택 이유, 위험과 되돌리기, 호환성과 마이그레이션, 남은 일은 쓸 내용이 있을 때만 둔다. 다만 `issueops next --id "$ISSUEOPS_ID" --json`의 `review.tier`가 `schema-auth`나 `contract`면 `위험과 되돌리기`를 반드시 두고, 깨질 수 있는 호출자·데이터와 되돌리는 방법(되돌릴 커밋, 되돌린 뒤 남는 상태)을 쓴다. 같은 내용을 여러 절에 복사하지 않는다. 작은 변경에 큰 다이어그램을 넣지 않는다. PR과 MR은 provider만 다르고 읽는 순서는 같다.

`review.tier`는 implement 이후 phase에서만 변경 집합으로 계산되고, `pr.create` 단계는 그 뒤이므로 값이 있다(`internal/application/issueopsnext/service.go:131-142`). 관측에 실패하면 `default`와 경고가 나오고 이 규칙은 적용되지 않는다.

### (6) sync-pr 절 표 — `issueops-sync-pr` 언제 쓰는가

`skills/issueops-sync-pr/SKILL.md:29-33`의 오른쪽 열을 템플릿 제목(`template.go:339-346`)으로 바꾼다.

| 신호 | 본문에서 낡은 부분 |
|---|---|
| 리뷰 피드백을 반영해 구현이 바뀌었다 | `## 변경 내용`, `## 리뷰 포인트` |
| 검증 명령·결과가 달라졌다 | `## 확인한 것` |
| target branch를 retarget했다 | `## 위험과 되돌리기`, base 설명 |
| breaking change 판단이 바뀌었다 | `## 요약`, `## 호환성과 마이그레이션` |
| 문서·마이그레이션을 추가했다 | `## 변경 내용`, `## 호환성과 마이그레이션` |

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제2장: "임시 우회, 출력 문구 보정, 테스트 기대값 완화, 문서만 변경하는 대응은 근본 원인이 이미 제거됐거나 현재 범위에서 물리적으로 제거할 수 없다는 근거가 있을 때만 허용한다." → (4) 렌즈의 판정 기준은 이 문장을 그대로 따른다.
- `.issueops/ARCHITECTURE.md` Host integration 요약: "skills는 `skills/` 원본 하나로 host-neutral이며 hub-and-spoke cross-reference를 따른다." → 변경은 공용 `skills/` 원본에만 하고 host별 사본을 만들지 않는다.
- `.issueops/CONVENTIONS.md` 핵심 불변식: "티어 계산은 implement 이후 phase에서만 일어난다." → (5) 규칙은 `pr.create` 단계에서 `review.tier`를 읽으므로 성립한다. plan 리뷰의 tier는 `default`로 고정이라 (5)를 plan 단계에 적용하지 않는다.
- `.issueops/conventions/go-and-packages.md` shared skill packaging: "검증은 repo-owned `python3 scripts/validate-skill.py skills/<skill-name>`로 수행한다." → 게이트 G9가 세 스킬에 이 명령을 쓴다. 성공 기준 7의 `verify-skill-shell.py`는 G10이 맡는다.
- `.issueops/CAUTIONS.md` → `.issueops/cautions/2026-10-07-check-expect-escape-re2.md`: "CHECK 안의 줄바꿈은 chr(10)으로 만들고 '|'는 쓰지 않는다." → 게이트 CHECK에 `|`와 정규식 alternation을 쓰지 않는다.
- `.issueops/ADR.md` → `.issueops/adr/2026-09-08-adversarial-review-throughput-executable-findings-change-tie.md` 기각한 대안: "parallel lens review by default, because coordination cost exceeds the saving outside schema-auth" → 병렬 버그 탐색·투표를 넣지 않는다.
- `.issueops/ADR.md` → `.issueops/adr/2026-10-07-review-revise-and-regress-rounds-are-capped-at-five.md` 제목: "Review revise and regress rounds are capped at five" → 상한 5회를 유지하고 3회로 낮추지 않는다.
- `.issueops/ADR.md` → `.issueops/adr/2026-10-14-gates-lint-and-pre-delta-author-sweep.md`: "`issueops-review` 루프: delta 리뷰를 띄우기 전에 호출자가 세 가지를 점검하고 그 결과를 delta 입력에 넣는다." → (1)과 (2)는 리뷰어 측 규칙으로 더하고, 저자 측 세 점검은 바꾸지 않는다.
- `.issueops/TESTING.md` 최소 완료 기준: "문서만 변경한 경우에도 문서 링크·구조를 대조하고, 단일 self-verify의 완전한 성공 결과로 test/build/golden/docs/inspect 기본 게이트를 확인한다." → 7단계에서 self-verify 한 번을 완전 성공으로 남긴다.
- `AGENTS.md` §3 Surgical Changes: "Touch only what you must. Clean up only your own mess." → sync-issue의 같은 유형 결함은 고치지 않고 남은 일로 남긴다.
- `.issueops/VCS.md`: 존재하지 않는다(route가 선택 문서로 돌려줬으나 파일 없음). PR 발행은 `issueops remote create-pr`만 쓴다.
- `.issueops/COMMIT_POLICY.md`: "- Risk: <남은 위험/롤백 주의. 없으면 Low>" → 커밋 body에 Risk 줄을 쓴다.
- `.issueops/AGENT_WORKFLOW.md` Work 절: "구조 선택이나 대안 기각 사유가 생기면 MCP `project_docs_append(kind=adr)`로 `.issueops/ADR.md`에 남긴다." → CI 경로 분류 제외 근거는 이슈 본문과 이 계획에 있고 구조 선택이 아니므로 6단계에서 ADR 필요 여부를 다시 판정한다.
- `.issueops/cautions/integrations.md` 9절 Shared skill drift: "`skills/<name>`을 원본으로 둔다." → 원본만 고친다.
- `.issueops/cautions/issueops-lifecycle.md` 27절: "스킬 description이나 필수 문서 목록을 바꾸면 `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -update`로 골든을 재생성하고" → frontmatter를 바꾸지 않으므로 golden 재생성이 필요 없다. G11로 확인한다.
- `skills/gates-ledger/SKILL.md` EXPECT 규칙: "`grep -n`처럼 줄 앞에 번호가 붙는 출력은 정규식 형식으로 쓴다." → 문구 게이트는 `grep -n` 대신 공백 정규화 검사로 `FOUND`를 출력하게 한다.
- `.issueops/cautions/audit-and-process.md`: 대조했으나 없음(리뷰 finding의 production fix 순서 규칙은 Go 변경이 없는 이 계획에 걸리지 않는다).
- `.issueops/cautions/2026-08-28-devils-advocate-record-was-unbound.md`: 대조했으나 없음(판정 기록 결속 문제이며 이 계획은 기록 형식을 바꾸지 않는다).
- `.issueops/cautions/2026-08-28-gitlab-workitem-labels-assignees-selection-create-child-glab.md`: 열지 않음: GitLab create-child 라벨 선택 문제로 GitHub 저장소의 스킬 문서 변경과 무관하다.
- `.issueops/adr/2026-10-02-sync-base-skill-chooses-merge-or-rebase-by-evidence.md`: 열지 않음: `sync-base` 스킬의 merge·rebase 선택이며 `sync-pr`과 이름만 겹친다.
- `.issueops/cautions/2026-08-27-skill-without-openai-yaml-self-verify-qa-gate.md`: 열지 않음: 새 스킬 추가 때의 `agents/openai.yaml` 누락 문제이며 이 계획은 스킬을 추가하지 않는다.
- `.issueops/cautions/2026-08-27-cleanup-stops-worktree-processes.md`: 열지 않음: cleanup 단계의 Orca 터미널 문제로 이 변경과 무관하다.

## 재사용하는 기존 구현

- 차단 기준은 `skills/design-review/SKILL.md`의 Blocking threshold를 그대로 쓴다. (3)은 그 원칙의 분류 표시만 더한다.
- 근본 원인 기준은 `CONSTITUTION.md:70` 문장을 발췌해 쓴다. 새 기준 문장을 만들지 않는다.
- `위험과 되돌리기`는 `artifacttemplate.sectionsFor`의 기존 선택 절(`risk_rollback`)이다. 새 절을 만들지 않는다.
- tier 값은 `issueops next`의 기존 `review.tier`를 읽는다. 새 CLI 필드가 없다.
- sync-pr 표의 제목은 `template.go:339-346`의 문자열을 그대로 옮긴다.
- 측정은 기존 `scripts/autoresearch/procedure_review_eval.py`를 `--spec`으로 재사용한다.

## 성능 영향

Go hot path 변경이 없다. 스킬 본문이 커져 리뷰어 프롬프트와 세션 컨텍스트가 늘어난다. 변경 전후 세 파일의 바이트 수를 `wc -c`로 기록하고, 세 파일 합계 증가가 3,000바이트를 넘지 않게 한다. (1)은 리뷰어가 대상 전체를 한 번 더 검색하게 해 1라운드 리뷰 시간이 늘 수 있다. 측정 계획에서 세션 시간을 함께 기록한다.

## 하위 호환성과 side effect

- CLI JSON, MCP schema, record schema, golden, provider body 계약은 바뀌지 않는다. frontmatter `description`을 바꾸지 않으므로 `TestResponseContractsGolden`도 그대로다.
- (5)는 `schema-auth`·`contract` PR 본문에 절 하나를 더 요구한다. 가독성 검사의 필수 절 판정은 바뀌지 않으므로 기존 PR 본문을 거부하지 않는다. 지키는 주체는 스킬을 읽는 에이전트다.
- 이미 게시된 PR 본문은 바뀌지 않는다. (6) 이후 `sync-pr`은 정규 제목을 가리킨다.
- 롤백: 세 스킬 파일만 바뀌므로 해당 커밋을 `git revert`하면 이전 동작으로 돌아간다. 설치된 host skill은 `skills/` 원본을 심볼릭 링크로 보므로 별도 재설치가 필요 없다.

### 프롬프트 출력 기준(prompt-engineering)

(1)~(4)는 리뷰어 서브에이전트 프롬프트에 들어가는 계약이다. 실패 기준과 확인 산출물은 다음과 같다.

| 규칙 | 실패로 보는 출력 | 확인 산출물 |
|---|---|---|
| (1) | 같은 유형 결함이 대상에 여러 곳 있는데 한 위치만 적고, 검색 근거나 "다른 위치 없음"도 없다 | 측정 시나리오 `review-diff-bugfix`의 judge 결과에서 같은 유형 결함 세 곳(E1~E3)의 found |
| (2) | delta 리뷰어가 직전 지적을 해결됨·남음으로 판정하지 않거나, 직전 지적을 새 필수 결함으로 다시 낸다 | 기존 `review-delta` 시나리오의 리뷰어 출력(직전 지적 상태 표시) |
| (3) | base에 있던 결함을 변경과 무관한데도 필수 결함으로 낸다 | `review-diff-bugfix`의 blocking_findings에 기존 결함(P1) 위치가 없어야 한다 |
| (4) | 버그 수정 diff가 테스트 기대값만 완화했는데 필수 결함으로 내지 않는다 | `review-diff-bugfix` judge 결과의 E4 found와 blocking 여부 |
| 회귀 | 기존 시나리오 점수가 떨어진다 | `review-diff`, `review-plan-clean`, `review-delta` 점수와 invalid blocking 수 |

측정 계획(4단계에서 실행한다. 결과 JSON은 ignored 영역인 `.issueops/issues/566/review/`(`.gitignore:40`)이나 워크트리 밖에 둔다):

1. 저장소 밖 임시 디렉터리에 `review-diff-bugfix` fixture를 만든다. `change.diff` 하나에 같은 유형 결함 세 곳(E1~E3, 예: `--json` 필드 camelCase 태그 세 개), 원인 함수는 그대로 둔 채 테스트 기대값만 완화한 우회(E4), diff 문맥에 보이지만 변경하지 않은 기존 결함(P1)을 넣는다. 결함 수정 사이클임을 알리는 `intent.md`(결함 수정 intent와 성공 기준)를 같은 fixture에 둔다.
2. 임시 spec에 기존 `review-diff`, `review-plan-clean`, `review-delta` 항목을 그대로 복사하고 새 시나리오를 절대 경로 fixture로 추가한다(`HERE / sc["fixture"]`는 절대 경로를 그대로 쓴다, `procedure_review_eval.py:245`). 새 시나리오의 `task`에도 결함 수정 리뷰임을 적는다.
3. 같은 `--bin`(워크트리에서 빌드한 `bin/issueops`)으로 `--repo`만 source checkout(변경 전 스킬)과 워크트리(변경 후 스킬)로 바꿔 `--reps 2 --out <json>`으로 돌린다. 스킬은 `--repo`에서 읽힌다(`:253`).
4. 판정은 `--out` JSON의 judge `expected`·`blocking_findings`와 `review_text`로 한다. stdout 점수만 보지 않는다.
5. 통과 조건: 새 시나리오에서 변경 후 E1~E4 재현율이 변경 전보다 낮지 않고, 변경 후 두 실행 모두 E4를 필수 결함으로, P1을 비차단 메모(기존 결함)로 낸다. `review-delta`의 변경 후 2라운드 리뷰어 출력이 직전 지적마다 해결됨·남음을 적는다. 기존 세 시나리오는 변경 후 점수가 변경 전보다 15점 넘게 떨어지지 않고 `review-plan-clean`의 invalid blocking 수가 늘지 않는다. LLM 실행이라 결정적이지 않으므로 게이트가 아니라 측정 증거로 남기고, 조건을 못 맞추면 원인을 진단해 문구를 고친 뒤 다시 잰다(최대 3회, 그 뒤에는 결과와 함께 보고한다).

## 구현 순서

1. 워크트리에서 세 파일의 변경 전 바이트 수를 기록한다.
2. (6) sync-pr 표 → (5) create-pr 문단 → (1)(3)(4) 출력 계약·입력 → (2) 루프 규칙 순서로 고친다.
3. 게이트 원장을 만들고 G1~G11을 실행한다.
4. 측정 계획을 실행한다.
5. 정리·문서·검증 단계로 넘긴다.

## 게이트

G1~G7은 규칙 문장이 줄바꿈돼도 찾도록, 백틱을 지우고 공백을 하나로 접은 본문에서 문구를 찾는다. `grep -n` 출력은 줄 번호로 시작해 리터럴 EXPECT와 맞지 않는다(`skills/gates-ledger/SKILL.md:46-50`). 문구는 위 `## 변경 상세`의 규칙 문장에서 백틱을 뺀 그대로다.

- G1: 결함 유형 전체 나열 규칙이 있다 | CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "같은 유형의 다른 발생 위치를 대상 전체에서 찾아 같은 결함 안에 나열한다" | EXPECT: FOUND
- G2: delta 리뷰어가 직전 지적 상태를 판정한다 | CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "직전 지적마다 해결됨 또는 남음만 판정하고" | EXPECT: FOUND
- G3: 기존 결함 표시 규칙이 있다 | CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "기존 결함으로 표시해 비차단 메모에 둔다" | EXPECT: FOUND
- G4: 성공 기준과 관련된 결함은 기존 결함에서 빠진다 | CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "성공 기준이 고치라고 한 결함, 결함 수정 사이클에서 그 결함의 근본 원인" | EXPECT: FOUND
- G5: 근본 원인 렌즈가 intent 기준으로 걸린다 | CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "intent가 결함·회귀 수정이면 diff에 근본 원인 렌즈를 더한다" | EXPECT: FOUND
- G6: 근본 원인 렌즈의 허용 조건이 CONSTITUTION 원문과 같다 | CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "근본 원인이 이미 제거됐거나 현재 범위에서 물리적으로 제거할 수 없다는 근거" | EXPECT: FOUND
- G7: create-pr이 위험이 큰 tier에서 위험과 되돌리기를 요구한다 | CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-create-pr/SKILL.md "review.tier가 schema-auth나 contract면 위험과 되돌리기를 반드시 두고" | EXPECT: FOUND
- G8: sync-pr 표에 템플릿에 없는 옛 제목이 없다 | CHECK: awk 'BEGIN{n=0} $0 ~ "## 변경 사항" {n++} $0 ~ "## 검증" {n++} $0 ~ "## 리뷰어 초점" {n++} $0 ~ "## 위험.rollback" {n++} $0 ~ "## Breaking Changes" {n++} $0 ~ "릴리즈 노트" {n++} $0 ~ "## 문서.마이그레이션" {n++} END{print "stale_headings=" n}' skills/issueops-sync-pr/SKILL.md | EXPECT: stale_headings=0
- G9: 바뀐 세 스킬이 유효하다 | CHECK: python3 scripts/validate-skill.py skills/issueops-review skills/issueops-create-pr skills/issueops-sync-pr | EXPECT: skills/issueops-sync-pr: ok: Skill is valid!
- G10: 바뀐 세 스킬의 shell 코드 블록이 유효하다 | CHECK: python3 scripts/verify-skill-shell.py skills/issueops-review skills/issueops-create-pr skills/issueops-sync-pr | EXPECT: skill shell verification passed: 3 file(s)
- G11: 스킬 변경이 response-contract golden을 바꾸지 않는다 | CHECK: go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1 | EXPECT: ok

G11은 base(source checkout)에서 이미 `ok`로 통과함을 계획 단계에서 확인했다. 실패하면 이 변경 탓이다.
