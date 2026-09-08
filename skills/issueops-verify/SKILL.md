---
name: issueops-verify
description: Run the IssueOps verify stage on the sealed diff without touching any file. Re-run the gate ledger and the repository's verification battery read-only, record the conditional schema evidence, run the adversarial implementation review through issueops-review, re-check compatibility against the real diff, and prove strict PR readiness leaves only commit and push. Use when "issueops next" reports verify, or when the user says "검증 단계", "검증해줘", "리뷰 돌리고 검증".
---

# IssueOps Verify

이 스킬의 일은 **봉인된 변경이 정말 통과하는지 확인하고 그 판정을 기록하는 것**이다.
파일은 하나도 바꾸지 않는다. 고칠 것이 나오면 앞 단계로 돌아간다.

- 전체 흐름과 단계 판별: [`issueops`](../issueops/SKILL.md)
- 게이트 원장: [`gates-ledger`](../gates-ledger/SKILL.md)
- 적대 리뷰: [`issueops-review`](../issueops-review/SKILL.md)
- 이전 단계: [`issueops-docs`](../issueops-docs/SKILL.md)
- 커밋: [`atomic-commit-push`](../atomic-commit-push/SKILL.md)

## 이 스킬이 맞는지 확인

```bash
issueops next --id "$ISSUEOPS_ID" --json
```

`stage.key`가 `verify`면 이 스킬이다. `clean`이나 `docs`면 봉인이나 문서 반영이
아직이므로 그 단계로 돌아간다.

## 파일을 만지지 않는다

change fingerprint는 `git diff <base>..HEAD`와 `git status`가 가리키는 **모든 경로의
내용 해시**다. untracked 파일도 들어간다. 그래서 이 단계에서 파일을 하나라도 바꾸면
정리 단계의 봉인과 문서 단계의 판정이 동시에 stale이 된다.

이 단계의 명령은 전부 읽기이거나 record 기록이다. record 기록은 durable state를 바꾸지만
워크트리 파일을 바꾸지 않으므로 fingerprint에 영향이 없다.

검증이 실패해 코드를 고쳐야 하면 4단계로 돌아가 구현·정리·재봉인·문서 반영을 다시
밟는다. `next`가 `clean`으로 되돌리는 것이 그 신호다.

## 0 동시에 띄운다

봉인된 fingerprint를 확인한 뒤 아래 셋을 **같은 fingerprint에 대해 동시에** 시작한다.
셋 다 읽기 전용이라 봉인을 바꾸지 않으므로 순서대로 기다릴 이유가 없다.

1. 게이트 원장과 저장소 검증 배터리(1절). `gates check`는 `--write` 없이 실행한다.
   `--write`는 4·5단계가 소유한다.
2. 스키마 실측(2절). 변경 집합에 스키마 파일이 있을 때만 활성화된다.
3. 구현 리뷰 서브에이전트(3절).
4. 화면 QA(`next`의 `review.frontend`가 true일 때만).
   [`aside-web-qa`](../aside-web-qa/SKILL.md)에 네 입력을 이렇게 매핑한다.
   TARGET=계획이나 이슈 본문의 로컬 실행 절차가 준 URL, SCOPE=변경 집합의 frontend 경로,
   REQUIREMENTS=이슈의 성공 기준과 `gates.md`, INTENT=4단계 report의 `## UI 판단` 절.
   넷 중 하나라도 없으면 지어내지 말고 QA를 `Not Run`으로 report에 사유와 함께 적는다.
   보고서 출력 경로는 ignored 영역 `.issueops/issues/<n>/review/`나 워크트리 밖으로
   고정한다 — 워크트리 안 미추적 파일은 fingerprint에 들어가 봉인을 깬다. 제품을 바꾸는
   시나리오는 그 스킬의 allowed mutations·cleanup 계약 안으로 한정하고 정리 영수증을
   report에 적는다. `aside-functional-qa`·`aside-visual-qa`를 직접 부르지 않는다.

- **배터리가 실패하면 리뷰와 QA 결과를 버린다.** 판정을 기록하지 않은 채 4단계로 돌아간다.
  실패한 diff에 대한 리뷰 판정은 fingerprint가 바뀌는 순간 무효다.
- 이 동시 실행의 전제는 정리 단계가 관련 검증을 이미 통과시켰다는 것이다. 배터리
  실패가 드물지 않으면 병렬화가 아니라 5단계를 먼저 고친다.
  `issueops review-metrics --repo "$WORKTREE" --json`의 revise 비율이 그 신호다.
- 이 fan-out은 `SUB_AGENT_PATTERNS.md`의 기대 이득 `parallel_speed`에 해당한다.
  그 slug와 실제로 절약한 벽시계 시간을 verified-execution report에 적는다.
- 정리(5단계)·문서 반영(6단계)과는 동시에 실행하지 않는다. 그 둘은 파일을 쓰므로
  봉인을 바꾼다.

## 1 검증 증거 확인과 필요한 재검증

현재 fingerprint에 대한 성공 기록이 있고 명령·입력·의존성·환경이 같으며 외부 상태의
유효기간도 지나지 않았으면 그 결과를 재사용한다. 단계가 바뀌었다는 이유만으로 같은
명령을 다시 실행하지 않는다. 하나라도 확인할 수 없거나 저장소가 새 실행을 요구하면
다시 실행한다. fingerprint 변경·실패·새 검증 항목은 재검증 사유다.

```bash
# 위 조건을 만족하면 기존 EVIDENCE를 읽는다. status는 CHECK를 실행하지 않는다.
issueops gates status --file "$LEDGER" --cwd "$WORKTREE" --workspace-root "$WORKTREE" --json

# 새 실행이 필요하면 CHECK를 실행하되 원장은 바꾸지 않는다.
issueops gates check --file "$LEDGER" --cwd "$WORKTREE" --workspace-root "$WORKTREE" --json

# 게이트 CHECK에서 이미 실행한 명령은 중복 실행하지 않는다.
# 저장소의 필수 검증 중 아직 유효한 성공 증거가 없는 명령만 실행한다.
issueops verify-work --json -- "$VERIFY_COMMAND"
```

위 명령은 모두 순서대로 실행하는 목록이 아니다. 재사용할 때는 원래 명령·결과·시점과
현재 입력이 같다는 근거를 보고한다. 실행하지 않은 명령을 새 PASS로 기록하거나 stale
판정을 덮어쓰지 않는다. 필수 리뷰·readiness·lease 검사는 그대로 수행한다.

- endpoint·DTO·OpenAPI가 바뀌었으면 `.issueops/OPEN_API_SPEC.md` 게이트를 적용하고
  `issueops api-doc check --json`을 실행한다. 대상 저장소에
  `npm run swagger:check` 같은 wrapper가 있으면 그것을 먼저 실행한다.
- `verify-work`는 실행한 명령과 결과를 evidence로 남긴다. 실행하지 않은 검증을 pass로
  적지 않는다.
- 미충족 게이트가 남으면 여기서 멈추고 4단계로 돌아간다. 원장을 고쳐 통과시키지 않는다.

## 2 스키마 실측과 기록

변경 집합에 마이그레이션·엔티티·`.sql`·`schema.prisma` 파일이 있을 때만 활성화된다.
없으면 이 게이트는 뜨지 않는다.

활성화되면 추정이 아니라 **실제 데이터베이스 관찰값**을 요구한다. 대상 테이블의 기존
인덱스 현황과 row 수, 그리고 그 값을 어디서 봤는지다. 조회는 [`database-design`](../database-design/SKILL.md)
또는 DB MCP 서버로 한다.

커넥션을 소모하는 대형 스캔을 던지지 않는다. `COUNT(*)` 전수 대신 카탈로그의 추정 row
수(`pg_class.reltuples`, `information_schema`, `SHOW INDEX`)를 쓰고 필요하면 `LIMIT`을
건다. 운영 DB에서 무거운 쿼리 하나가 커넥션 풀을 마르게 한다.

```bash
issueops schema-evidence record --id "$ISSUEOPS_ID" \
  --measurement "orders: 8.4M rows(reltuples), idx_orders_user_id 없음" \
  --source "mcp db-bc-prod execute_sql_bc_prod_market" \
  $RECORD_ACTOR_FLAGS --json
```

- measurement와 source는 짝이다. 출처 없는 수치는 추정과 구분되지 않는다.
- 관찰이 불가능하면 `--waive --waiver-rationale "<근거>"`로 남긴다. rationale 없는
  waive는 게이트를 열지 않는다.
- 실측 결과가 구현을 바꿔야 한다면 그것은 이 단계가 아니라 4단계의 일이다. row 수가
  크면 인덱스 생성 전략, 마이그레이션 잠금 시간, 백필 배치 크기가 달라진다.

## 3 구현 리뷰

[`issueops-review`](../issueops-review/SKILL.md)를 `--target diff`로 호출한다. 루프
절차는 그 스킬이 소유한다. 이 단계가 아는 것은 여섯이다.

- 리뷰어에게 diff와 **계획을 함께** 준다. 무엇을 하기로 했는지 모르는 리뷰어는 구현이
  계획에서 벗어났는지 판정할 수 없다.
- `pass`만 통과한다. `revise`면 지적을 고쳐야 하므로 4단계로 돌아간다. 이 단계에서
  고치면 fingerprint가 바뀌어 앞 판정이 전부 stale이 된다.
- 모드에 따른 면제는 없다. execution이 있는 사이클은 전부 이 게이트의 대상이다.
- **계획 리뷰가 남긴 주장을 함께 준다.** `issueops status --id "$ISSUEOPS_ID" --json`의
  `devils_advocate_review.findings`와 `history`의 finding을 "검증할 주장 목록"으로
  프롬프트에 넣는다. 리뷰어는 diff 전체를 탐색하기 전에 그 주장이 실제로 지켜졌는지
  확인한다. 계획 리뷰에서 살아남은 위험이 구현에서 되살아났는지가 가장 싸게 잡히는 결함이다.
- **렌즈는 `next.review.lenses`만 적용한다.** `issueops next --id "$ISSUEOPS_ID" --json`이
  돌려주는 티어와 렌즈를 프롬프트에 넣고 그 목록만 검토하게 한다. `docs-only` 티어는
  side effect 렌즈 하나다.
- **`review.frontend`가 true면 UI 렌즈를 더한다.** diff 리뷰 프롬프트의 "검증할 주장
  목록"에 4단계 report의 `## UI 판단` 절을 넣고, 렌즈 목록에 접근성·반응형 상태·모션
  감소 세 항목을 덧붙인다. 이 셋이 [`ui-ux-craft`](../ui-ux-craft/SKILL.md)가 소유한
  판단이며, frontend 사이클의 리뷰 finding에 이 렌즈 언급이 하나도 없으면 프롬프트가
  실패한 것이다.

`next.review.tier`가 `schema-auth`이고 `git -C "$WORKTREE" diff --stat "$BASE_SHA"`의
변경 줄 수가 500을 넘을 때만 렌즈 네 개를 서브에이전트 넷으로 나눈다. 합치는 규칙은
"필수 결함이 하나라도 있으면 `revise`"다. 그 밖의 티어에서는 나누지 않는다 — 조정
비용이 절약한 시간을 넘는다. 이 분할 조건은 이 문장이 단독으로 소유하며 코드에는 없다.

## 4 호환성 재확인과 readiness

strict readiness `warnings`의 `base_advanced`는 차단이 아니다. 이 단계에서는
`issueops execution sync-base --id "$ISSUEOPS_ID" --preview $ACTOR_FLAGS --json`으로 충돌
유무만 확인해 report에 적고 **apply하지 않는다.** apply는 봉인된 변경 집합에 base의
파일을 끌어들여 fingerprint를 바꾸고, 정리·구현 리뷰·문서 반영 판정이 한꺼번에 stale이
되어 5단계부터 다시 밟게 만든다. 머지는 PR 병합 시점에 provider가 한다. 충돌이 있으면 그
사실을 PR 본문의 위험 절에 적는다.

구현된 diff가 계획 시점의 compatibility review와 다르면 durable 판정을 최신으로 맞춘다.

```bash
issueops compatibility review --id "$ISSUEOPS_ID" \
  --backward-compatibility "<실제 diff 기준>" --side-effect "<실제 diff 기준>" \
  --rollback-plan "<실제 되돌리는 방법>" --verification "<이 단계에서 실행한 검증>" \
  --approved $RECORD_ACTOR_FLAGS --json

issueops pr-readiness --id "$ISSUEOPS_ID" --strict --json
```

strict readiness의 `missing`이 `worktree_clean`, `upstream`, `upstream_fetch`,
`upstream_synced`의 부분집합이면 남은 일은 커밋과 푸시뿐이다. 그 밖의 키가 남으면
`next`가 가리키는 단계로 돌아간다. 특히 `gates_incomplete:*`와 `*_stale`은 앞 단계로
되돌아가라는 뜻이다.

## 출구

다음은 8단계 커밋·푸시다. [`atomic-commit-push`](../atomic-commit-push/SKILL.md)로
plan.md, gates.md, verified-execution report, 문서, 구현을 커밋·푸시하고, `next`가 렌더한
`phase --to pr`를 실행한다.

## 나쁜 예

- 배터리가 실패했는데 동시에 돌던 리뷰의 `pass`를 기록한다. 그 판정은 무효인 diff에
  대한 것이다.
- 검증 단계에서 `gates check --write`를 실행한다. 원장이 바뀌어 봉인이 stale이 된다.

| 나쁜 행동 | 문제 |
|---|---|
| 이 단계에서 파일을 고친다 | 앞 두 단계의 봉인과 판정이 동시에 stale이 된다 |
| 실행하지 않은 검증을 pass로 기록한다 | 기록은 남고 검증은 없다 |
| 리뷰 없이 구현 리뷰를 pass로 기록한다 | 게이트 연극이다 |
| 운영 DB에 `SELECT COUNT(*)` 전수 스캔을 던진다 | 커넥션 풀을 마르게 한다. 카탈로그 추정치를 쓴다 |
| direct 모드라서 구현 리뷰를 생략한다 | 모드 면제는 없다. 리뷰 없이 게시된 변경은 검토되지 않은 변경이다 |
| revise 지적을 이 단계에서 고치고 재리뷰를 생략한다 | fingerprint가 바뀌어 판정이 stale이 되거나, 검토되지 않은 수정이 게시된다 |
| 미충족 게이트를 원장 수정으로 통과시킨다 | 원장이 통과하고 결과는 검증되지 않는다 |
| 이 단계에서 커밋한다 | 커밋은 8단계다. 여기서 커밋하면 무엇을 검증한 상태인지 흐려진다 |

## 검증

- `issueops pr-readiness --id "$ISSUEOPS_ID" --strict --json`의 `missing`이
  커밋·푸시로 해소되는 키만 남았다.
- `issueops status --id "$ISSUEOPS_ID" --json`의 `implementation_review`와
  `schema_evidence`(활성화된 경우)가 현재 fingerprint에 묶여 있다.
- `git -C "$WORKTREE" status --porcelain`이 이 단계 시작 때와 같다. 달라졌으면 이 단계가
  파일을 바꾼 것이다.
