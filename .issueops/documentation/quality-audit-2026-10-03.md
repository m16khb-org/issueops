# 스킬·프로젝트 문서 구조 및 품질 감사

**최종 상태: 완료.** 정량 비교와 독립 정성 검토를 마쳤고, 최종 단일
self-verify run은 28/28 통과, 최소 목표 점수 100이다. 실행 조건과 한계는 아래에 기록했다.

## 범위와 판정 방법

사용자 요청에 따라 저장소가 소유한 모든 스킬과 project docs를 대상으로
구조, 중복, 내용 품질을 점검한다. 현재 운영 지침과 과거 계획·증거,
machine-consumed template/fixture, 생성된 OpenWiki를 구분한다.
역사적 판정이나 생성 결과를 현재 규칙인 것처럼 다시 쓰지 않는다.

기준 revision은 `345ae6187cd75b786985e65a6074e77f237544b1`이다.
전수 목록과 원문 SHA-256은
[quality-baseline-2026-10-03.json](quality-baseline-2026-10-03.json)에 보존했다.
분류는 감사용 cohort이며, 파일이 현재 규칙인지 여부는 작성 목적과 실제
consumer를 추가로 확인한다. 줄 수만 보고 규칙이나 예시를 삭제하지 않는다.

## 정량 기준선

| 항목 | 변경 전 |
|---|---:|
| 추적 Markdown 전체 | 889 |
| repo 소유 SKILL.md | 52 |
| 스킬 Markdown, fixture 제외 | 93 |
| SKILL.md 전체 줄 수 | 11,420 |
| SKILL.md가 250줄을 넘는 진입점 | 16 |
| 스킬의 실제 local Markdown 링크 | 186 |
| 스킬의 끊어진 local Markdown 링크 | 0 |
| 기존 project-doc checker 검사 문서 | 611 |
| 기존 checker가 검사한 family | 6 |
| 기존 checker 위반 | 0 |
| 첫 분류의 운영 문서·스킬·root 진입점 cohort | 174 |
| 해당 cohort의 동일한 긴 문단 중복 | 1 |

줄 수는 최종 newline을 빈 줄로 중복 계산하지 않는다. 중복 문단은 공백을
정규화한 160자 이상의 동일 문단이 서로 다른 파일에 있는 경우다. code fence와
표로 시작하는 block은 이 지표에서 제외했다. 이 수치는 의미 중복을 측정하지
않으므로 아래의 책임 검토를 별도로 수행한다.

스킬의 250줄은 이번 감사의 진입점 설계 목표이지 기존 validator의 hard limit은
아니다. 한도를 낮추거나 범위를 제외해 기존 검사를 통과시키지 않는다.
기존 project-doc manifest의 budget은 그대로 적용한다.

## 정성 평가 기준

| 축 | 통과에 필요한 근거 |
|---|---|
| 규칙 보존 | 이동한 절의 출처·도착지와 필수 gate, 실패·중단 조건, 입출력 계약을 대조한다 |
| 정확성 | 발견한 command/version/inventory 주장을 현재 코드와 실제 read-only CLI 출력으로 확인한다 |
| 탐색성 | root는 역할·진입 조건·workflow·critical rules를 유지하고 필요한 reference를 직접 안내한다 |
| 중복 책임 | 한 주제의 normative owner를 정한다. 독립 활성화에 필요한 짧은 요약과 실제 중복을 구분한다 |
| 검증 가능성 | 위 판정에 파일·명령·관측 결과를 붙인다. 설명만으로 높은 점수를 부여하지 않는다 |

스킬 자체의 task 수행 품질과 문서 품질은 구분한다. 문서를 줄였다는 이유로
실제 에이전트의 성공률·latency·점수가 개선됐다고 주장하지 않는다.
근거가 부족한 항목은 통과로 처리하지 않는다.

## 확인한 문제

| ID | 문제 | 근거 | 조치 방향 |
|---|---|---|---|
| D1 | 스킬 수와 명칭 분류가 오래됨 | TECH_STACK은 34개라 설명하지만 실제 inspect와 SKILL.md는 52개 | 손으로 유지하는 목록·합계 대신 실제 discovery와 metadata를 source로 안내 |
| D2 | self-verify 계약 버전이 문서마다 다름 | skill은 v5, testing 문서는 v6, ContractValue 구현은 7 | 버전·hash의 native source와 baseline 호환 계약을 일치시킴 |
| D3 | Python 검사 설명이 현재 runner와 다름 | skill의 unittest-discover 설명과 python_suite_runner.py 실제 실행이 다름 | 실제 runner 경로와 검사 책임을 맞춤 |
| D4 | HTTP/stdio 혼동 후보를 재검토 | OPERATIONS, operations/install, architecture/runtime은 이미 HTTP 기본값과 stdio 호환 경로를 구분함 | 초기 가설을 기각하고 올바른 설명을 보존 |
| D5 | 지원되지 않는 command 예시와 actor 없는 예시가 섞임 | skill-bench는 exit 2; feedback/status alias는 실제 지원됨을 독립 검토로 재확인 | 가짜 runner는 제거하고 정상 feedback 기록은 호출 stage의 인증 계약으로 연결 |
| D6 | 상세 방법·예시가 진입 파일을 과대하게 만듦 | 16개 SKILL.md가 250줄 초과; 평가는 본문만 받음 | 필수 실행 규칙은 root에 보존하고 선택적 예시·배경만 reference로 분리 |
| D7 | API 문서 gate 설명의 owner가 중복됨 | AGENT_WORKFLOW와 testing/api-documentation에 같은 긴 문단 | 실행 workflow에는 canonical owner로 가는 안내만 유지 |
| D8 | 평가 격리 규칙과 결과 template가 rubric v2와 충돌 | rubric은 fresh session과 6개 축을 요구하지만 rerun fixture는 기존 main session과 5개 축을 허용 | 평가자 규칙의 owner를 rubric으로 통일하고 현재 template를 맞춤 |

## 실행 단위와 보존 경계

1. **운영 문서 정합성:** TECH_STACK, 설치·검증 guide, self-verify 관련 skill의
   사실 관계와 canonical owner를 맞춘다.
2. **분석·계획 스킬:** algorithm-optimization, code-quality-metrics,
   implementation-planning, issueops-debugging, prompt-engineering,
   requirements-analysis, web-research의 필수 방법은 본문에서 간결하게 유지하고
   선택적 예시·배경을 분리한다.
3. **Git·review·execution 스킬:** git-operations, sync-base, pr-review,
   verified-execution을 같은 기준으로 정리한다.
4. **IssueOps lifecycle:** router, implement, create-issue, cleanup의 중복
   예시와 상세 설명을 정리하되 actor/generation/권한/중단 계약을 유지한다.
5. **나머지 스킬과 project docs:** metadata·구조·링크·공통 지침 책임을 전수
   점검한다. 문제가 없는 문서는 단순한 문체 통일을 위해 다시 쓰지 않는다.
6. **정량·정성 검증:** 새 reference를 포함한 전후 지표, 이동 보존 대조,
   명령 사실 확인, 독립 검토, strict 검사와 실제 discovery 결과를 기록한다.

meeting-notes처럼 root 자체를 읽는 기존 consumer가 있는 경우에는 분리 가능
범위를 먼저 확인한다. machine-consumed template, schema, field와 named marker를
임의로 이동하거나 삭제하지 않는다. 코드·테스트 변경을 검증 회피 수단으로 쓰지 않는다.
body-only benchmark, no-input/no-change/refusal 예외, 필수 artifact label은 유지한다.
250줄 목표가 이 보존 계약과 충돌하면 보존을 우선하고 예외를 결과에 기록한다.

## DAG 및 독립 검토

CLI의 인증 probe는 invalid_state를 반환했지만 네이티브 DAG의 Astra 노드는
실제 source inspection을 수행했다. 에이전트 가용성은 CLI probe만으로 단정하지 않았다.
공통 평가·integration 지침의 owner 결정과 보존 검토를 의존 관계로 실행한다.
에이전트의 완료 보고는 파일과 코드 근거를 대조하기 전에는 완료 증거로 사용하지 않는다.

이 문서는 감사의 범위·기준·발견을 기록한다. 최종 판정은 실제 수정과 검증 결과를
추가한 뒤에만 내린다.

## 정량 결과

최종 보존 보완 후 같은 기준으로 다시 측정했다.
[전체 per-skill 측정값](quality-after-2026-10-03.json)은 줄·단어·UTF-8 byte를
따로 기록한다. 단어 수는 공백 기준 단위이며 모델 token 수가 아니다.

| 항목 | 변경 전 | 변경 후 |
|---|---:|---:|
| SKILL.md 진입점 | 52 | 52 |
| 진입점 줄 수 | 11,420 | 9,746 |
| 진입점 단어 단위 | 85,328 | 72,015 |
| 진입점 UTF-8 byte | 654,662 | 578,008 |
| 250줄 초과 진입점 | 16 | 11 |
| source cohort 파일 수 | 174 | 187 |
| source cohort 전체 줄 수 | 28,889 | 27,840 |
| 스킬 local Markdown 파일 링크 | 186 | 179 |
| 끊어진 스킬 파일 링크 | 0 | 0 |
| frontmatter 변경 | — | 0 |

진입점은 줄 수 약 14.7%, 단어 단위 약 15.6%, byte 약 11.7% 줄었다.
선택적 reference를 추가했으므로 source 파일 수는 늘었지만 전체 source 줄 수는 줄었다.
감사 보고서·측정 JSON은 source cohort의 절감 수치에 넣지 않았다.

긴 동일 문단 cluster 수는 1→1이다. 기존 API gate 중복은 canonical owner 안내로
대체했다. 남은 cluster는 router와 implement가 각각 독립 실행될 때 필요한 handoff
안전 계약이다. 이를 없애면 C1/C2 보존 결함이 재발하므로 의도적으로 유지했다.
중복 수가 무조건 0이어야 한다는 목표는 세우지 않았다.

## 정성 결과와 예외

- [독립 스킬 보존 검토](skill-preservation-verdict.md): 최초 NEEDS-FIX 후
  C1-C4/E1-E2를 보완하고 R1의 미검증 변경을 원복해 최종 PASS.
- [독립 project-doc 검토](project-quality-verdict.md): 검토한 사실·owner·탐색·
  문서 계약에 material finding 없음, PASS.
- [보완 내역](preservation-corrections.md): 회귀 테스트를 약화하지 않고
  실제 안전·state·review 계약을 본문에 복원했다.
- 과거 기록·fixture 542개는 SHA-256이 동일하다. 협업 예시 네 묶음은 본문 내용을
  보존하고 청중별로 분리했으며 기존 인덱스와 heading 진입점도 유지했다.
- requirements-analysis는 수정 전 OCR RED 증거를 갖추지 못했으므로 원문으로 복원했다.
  meeting-notes와 긴 실행 계약 등은 독립 실행·machine-consumed 입력을 보존하기 위해
  250줄 목표보다 완전성을 우선했다. 개별 예외는 lane report와 측정 JSON에 남겼다.

이 판정은 문서의 정합성과 계약 보존이다. 새 actor benchmark, OCR 평가,
실제 사용자 task 성공률 또는 runtime latency 개선을 측정했다고 주장하지 않는다.

## 최종 검증과 한계

- 전체 52개 스킬 validator 통과. 이름·description을 포함한 frontmatter 변경 0,
  현재 discovery 52개, local Markdown 파일 링크 오류 0.
- strict project-doc checker: `ok=true`, `violations=[]`. 실제 `inspect`와
  `project route-docs`도 성공했고 모든 반환 문서가 존재했다.
- 관련 Python 전체 suite, architecture/projectdoc/projectdocs, 기존 skillcontract
  검사 통과. 테스트를 삭제·skip·완화하지 않았다.
- 최종 self-verify 원본: `/tmp/issueops-docs-final-pass.json`.
  Go 1.26.3, Python 3.13, `GOFLAGS=-p=2`, seed 100, target 95,
  `--llm-eval=false --progress=jsonl --json`로 실행했다.
  exit 0, 28/28 성공, 최소 목표 점수 100, `termination_eligible=true`,
  소요 350,556ms다. 검증 도중 source cohort와 manifest hash 변화는 없었다.
- Go test는 `go test ./... -count=1` 전체 범위였고 package 동시성만 2로 제한했다.
  문서-only 변경의 risk tier는 standard였다. 이 run을 race/vet 실행 증거로
  확대하지 않는다.
- 기본 package 병렬 실행 두 번에서 수정하지 않은 provider 취소 테스트가 기존
  3초 관측 한도를 넘겼다. 해당 테스트 단독과 package 전체 단독은 각각 통과했다.
  최종 검증은 동시성을 명시해 통과했지만 이 기존 부하 민감성의 근본 원인을
  수정했다고 주장하지 않는다. provider 코드와 테스트는 그대로다.
- 한 일반 문장의 token budget 표기를 redaction 규칙이 대입문으로 오인했다.
  의미가 같은 자연어 표현으로 수정했으며 redaction 규칙은 완화하지 않았다.
- Git commit, push, 사용자 홈 설치·설정 갱신은 수행하지 않았다.

최초 worker report는 초기 snapshot이다. 최종 수치는 측정 JSON을, 보존 지적의
현재 상태는 각 독립 verdict의 마지막 closure를 따른다.
