# #515 검증 보고서

## 의도 대조

공개 본문 10건의 지표를 재현하고 독립 reader 10개를 원래 의도와 대조해 독자 검토의 필수화 여부를 결정했다. G1–G5가 각각 입력·지표·reader·ADR·원격 관측을 덮고 G6가 최종 문서 배터리를 소유한다. 원래 산식과 공개 PR/MR 8건 목록 부재는 재구성 측정·N/A로 처리했다. 구현 게이트, 표본 수정, 비공개 자료는 범위에 넣지 않았다.

## 결과와 증거

| 수용 기준 | 이분 판정 방법 | 실제 자료 |
|---|---|---|
| T1 고정 원문과 사전 의도 | 10개 번호·원문 해시·선정 순서가 일치하고 rubric 시각이 reader보다 앞선다 | [snapshot](../../../docs/research/2026-09-30-readability/frozen-sample.json), [rubric](../../../docs/research/2026-09-30-readability/pre-reader-rubric.json), G1 |
| T2 재현 지표 | 별도 프로세스의 계산이 저장 결과와 같다 | [metrics](../../../docs/research/2026-09-30-readability/metrics.json), G2 |
| T3 독립 reader | 서로 다른 context 10개·도구 0건·정확한 본문 하나·네 답·질문별 판정이 있다 | [readers](../../../docs/research/2026-09-30-readability/readers/), [비교](../../../docs/research/2026-09-30-readability/reader-comparison.json), G3 |
| T4 결정 연결 | 권장 유지 ADR과 색인이 연결된다 | [ADR](../../adr/2026-09-30-reader-review-remains-recommended-after-ten-public-bodies.md), G4 |
| 원격 drift | 실제 API 10건의 최신 body hash를 snapshot과 비교한다 | [remote comparison](../../../docs/research/2026-09-30-readability/remote-comparison.json), G5 |
| 문서 배터리 | 필수 연구 검사·Python discovery·완전 self-verify 단일 run이 모두 성공해야 한다 | [원장](gates.md), G6 |

[연구 보고서](../../../docs/research/2026-09-30-readability/README.md)에 본문별 표와 공개 기준선의 한계가 있다. H2 중앙값 4.5개, 문자 수 중앙값 1,525.5자이며 독자 검토를 권장 절차로 유지한다. Q1–Q3 원래 의도 확인 가능 7/10건 중 불일치 0/7, unknown 3/10이다. #520 Q2는 정보가 없어 답하지 못했다. Q4 지적은 10/10이다. #527 원격 drift는 원래 snapshot 결과와 구분했다.

## 검증 책임

G6는 `battery.py`가 같은 작업 공간·환경에서 처음부터 실행하는 단일 배터리다. Homebrew Python 3.14를 PATH 앞에 두고 프로세스 범위 `GOFLAGS=-p=2`, `GOMAXPROCS=4`를 사용한다. Python discovery와 self-verify의 26개 step 결과를 확인한다. 전체 test·build·golden·docs·inspect는 이 self-verify run이 소유하며 별도로 중복 최종 실행하지 않는다. Go 변경이 없어 추가 vet·race 책임은 없다. 실패 시 전체 로그를 private 임시 증거에 보존하고 처음부터 다시 실행한다. 알려진 host probe의 100ms flaky를 수정하거나 skip하지 않는다.

첫 원장 실행은 1,800초 timeout 요청이 정책의 15분 상한을 넘어서 6개 CHECK 모두 실행 전에 거부됐다. 실패 JSON을 보존했고 900초 상한으로 처음부터 다시 실행한다. 코드·정책·EXPECT는 바꾸지 않았다.

작은 실패 fixture에서는 변조 snapshot을 거부하고 코드 안 H2·64hex·inline code를 제외하며 실제 H2와 코드 밖 hex를 센다. 문서 검사에서는 417개 문서·6개 family에 위반이 0개였다. 이 결과가 전체 배터리를 대신하지 않는다.

## 정리와 변경 범위

unsupported-claim 정리에서 #520 reader의 Q2 정보 부재를 반영해 초안의 모든 공개 범위 질문 일치 주장을 29/30 match·1/30 unanswerable로 바로잡았다. 원래 의도 unknown과 정보 부재를 분리했다. 코드 품질 SNR·entropy·중복·boilerplate는 문서 중심 연구에 적용하지 않아 N/A다. 연구 원문·답변 중복은 각각 입력과 관측 증거이므로 의도적으로 보존한다. 연구용 Python 두 파일은 250줄 이하며 production 추상화를 추가하지 않았다.

## Side effect와 성능

추적 파일은 연구 결과·선택 snapshot·reader 원문·재현 도구, ADR와 색인, #515 계획·의도·리뷰 사본과 원장·보고서다. 원격 변경은 이 브랜치 push와 Draft PR만 포함한다. source, #527/#528 작업 공간, 표본 원문과 공용 스킬·설치·전역 설정을 변경하지 않는다. rollback은 이 문서 커밋을 되돌리는 것이다.

독립성 패턴은 fresh-context reader, 리뷰는 devil's-advocate다. reader 10회 합산 716.82초, 최대 5개 동시 실행의 관측 벽시계 시간은 358.29초였다. `parallel_speed` 관측 차이는 358.53초다. 이는 순차 합산과의 관측 차이이며 반복 실행의 보장은 아니다. 비용은 usage만 보존하고 금액으로 추정하지 않았다. 사용자 실행 경로의 성능은 바뀌지 않는다.

## 발행과 종료 경계

최신 HEAD의 push CI와 PR CI가 모두 성공한 뒤 `remote verify-artifact`와 `execution complete`로 증거를 durable record에 연결한다. 최종 HEAD·PR URL·completion receipt는 `io-fdb9f8c080b3`의 실제 완료 record가 소유한다. 보고서에 자기 커밋 SHA나 생성 전 PR 번호를 만들어 넣지 않는다. 이 사이클은 verified Draft PR, done/released에서 끝내며 머지·cleanup·설치 갱신은 수행하지 않는다.

Success criteria: T1–T4와 G1–G6.
Evidence artifact: 이 보고서의 표에 연결된 공개 자료, 실제 원격 API 관측과 reader 응답.
Cleanup receipt: reader child 명령 10개 모두 종료; 배터리 임시 로그는 실패 조사용으로 보존.
Verification mode: 문서 연구에 필요한 focused reproduction과 프로젝트 필수 단일 배터리.
Skipped checks: Go 변경 전용 vet/race와 API/DB/browser gate는 범위에 해당하지 않는다.
