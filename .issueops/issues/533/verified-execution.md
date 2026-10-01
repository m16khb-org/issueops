# #533 구현 검증 보고서

Cycle: `io-339cd3178172`. Branch: `533-child-create-recovery`.
범위는 child 생성·복구 계약이며 main 병합·cleanup·전역 설치는 root가 수행한다.

## 의도 대조

| 기준 | 관찰한 결과 |
|---|---|
| 모호한 create 오류 뒤 두 번째 create 차단 | fake gh URL+exit1 및 URL 없는 exit1에서 create 1회. help 오류는 create 0회 |
| 생성 후 기록 실패·재시작 복구 | receipt 실패 뒤 같은 URL을 채택하고 link+completed를 한 CAS로 기록 |
| 무ID 요청의 영속 identity | 실제 GitHub·GitLab CLI에서 completed 응답 유실 → 재시작 → 같은 ID/URL. explicit B 뒤에도 implicit A 유지 |
| 미해결 작업 차단 | pending/unknown/verification/receipt 실패는 새로운 ID로도 생성하지 않음 |
| 검증·preview 보존 | preview durable state·attach 0회. candidate zero/many/truncated 및 metadata/project/parent drift 거부 |
| 현재 권한 유지 | stale operation의 자동 replay 거부, 현재 holder의 explicit reconcile만 허용. 관찰 generation과 CAS generation 비교 |

## TDD와 검증

초기 RED는 preferred create 오류에서 create 2회와 receipt 실패 뒤 재생성으로 확인했다.
GitHub·GitLab provider, application, strict lease codec, domain, CLI focused 검사를 실행했다.
`gates.md`가 G1–G3의 명령과 마지막 결과를 보존한다.

독립 리뷰 round1은 세 필수 결함을 찾았다. 아래 최소 재현은 모두 수정 전 exit 1,
수정 뒤 exit 0이었다. 실제 record codec을 거치는 CLI 테스트로 URL 영속 보존도 확인했다.

- CAS가 not_invoked A를 선택한 동시 재시도: B marker가 전송되는 RED를 확인했다.
  선택된 A marker와 봉인 digest를 대조한 뒤 전달하여 create 1회와 같은 URL reconcile을 확인했다.
- GitLab 생성 뒤 follow-up process start failure: 원래 URL 없이 pending으로 남는 RED를 확인했다.
  post-create 오류는 생성 전체의 Invoked=true로 분류하고, 알려진 URL을 not_invoked로 분류하지 않는다.
  새 production reader가 URL을 읽고 재생성을 막으며 같은 URL로 복구한다.
- GitLab 부모 issues/work_items 별칭: 같은 부모의 preview가 거부되는 RED를 확인했다.
  기존 identity 비교 함수를 재사용하며 host/project/IID 변경은 계속 거부한다.
  이미 연결된 자식의 preview와 confirm에는 추가 create/attach가 없다.

CHECK: `go test ./internal/application/issueopsremote ./cmd/issueops/issueopscli/remotecmd -run
'TestChildConcurrentNotInvokedRetryUsesSelectedOperationBody|TestChildCLIFollowUpStartFailurePersistsKnownURL|TestChildCLIGitLabParentAliasesAndDrift' -count=1`.
GREEN: 최초 수정 application 0.320초, CLI 13.170초. 마지막 lint 정리 뒤 같은 회귀도 통과했다. RED/GREEN 원문은 lifecycle evidence 경로에 보존한다.
DDD 정규 inventory generator도 실행해 focused architecture 검사를 통과했다.
수정 전 전체 배터리는 510.59초에 종료했으나 최종 완료 증거에서 제외한다.
최종 배터리는 delta 리뷰 후 처음부터 다시 실행한다.

최종 전체 검증 명령은 root가 승인한 serialized wrapper 아래에서 gofmt, vet,
전체 race, build, docs, inspect와 self-verify를 실행한다. 전체 Go/Python/golden 검사는
self-verify의 실제 step 결과를 사용한다. 최종 결과는 completion의 verification 영수증과
`/tmp/issueops-five-20261001/533/final-battery.log`에 기록한다.
독립 리뷰와 PR 최신 HEAD의 CI 결과는 lifecycle의 implementation_review와 remote artifact
영수증에 기록한다. 이 보고서만으로 게시·CI 완료를 주장하지 않는다.

## Side effect와 rollback

- confirm은 provider 호출 전에 child_create_operations에 sealed identity를 저장한다.
- 작업당 create는 한 번이며, 복구는 기존 child의 조회와 필요한 hierarchy attach만 수행한다.
- completed receipt와 child link가 같은 CAS에 들어가며 부모 IssueURL은 유지한다.
- 본문 원문은 state에 저장하지 않고 digest와 marker를 저장한다.
- 기존 v1 record의 필드 부재는 허용한다. 새 필드를 모르는 예전 binary로 state를 읽는
  rollback은 strict decoder가 거부하므로 runtime을 예전 binary로 되돌리지 않는다.
- 원격 자식을 rollback 명목으로 삭제하지 않는다.

## 성능·정리 측정

변경 production Go 파일 전체(추적·미추적 포함)의 shell 방식 근사 측정은
초기 정리 전후 SNR 0.953020 → 0.953031, boilerplate 비율 0.031767 → 0.031760이었다.
리뷰 수정 후 같은 production 파일 범위의 4,477개 nonblank 줄 중 주석 210개로
SNR 근사는 0.953094이고 package/import/struct 선언 줄 비율은 0.031718이다.
함수 signature를 제외한 본문이 같은 쌍은 1개다. GitHub·GitLab의 post-create 오류
wrapper이며, 각 provider 경계에서 생성 전체의 호출 의미를 보존하므로 그대로 둔다.
주석 수를 noise 근사로 계산했으며 의미적 dead code 판정이나 AST 복잡도 점수가 아니다.
zero input은 insufficient-input으로 처리한다. 초기 정리와 리뷰 수정 후 측정은 방법 차이를 구분하며, 수치 변화만으로 품질 개선을 주장하지 않는다.

첫 GitHub create에는 bounded help 조회 1회가 추가된다. 완료 replay의 provider create는
0회이고, 동시 implicit 요청의 실제 create는 1회다. application focused 0.351초,
architecture focused 6.691초는 병렬 작업 중 관찰값이며 성능 개선으로 해석하지 않는다.
정리는 오류 후 fallback으로 오해할 수 있는 preview 문구를 capability 선행 선택으로
바꿨다. CAS에서 선택된 marker로 본문을 재구성하므로 CAS 전 Seal의 본문 반환값은 사용하지 않도록 바꿨다.
CI와 같은 golangci-lint v1.64.8의 focused lint로 불필요한 대입 제거를 확인했다.
추상화 확대나 범위 밖 파일 정리는 수행하지 않았다.

## UI 판단

CLI 계약 변경이다. 브라우저 UI 변경과 화면 QA 대상은 없다.

## 리뷰 실행

round1 전체 diff 리뷰는 독립 세션에서 수행했다. 필수 결함 세 개에 대해서만 수정했고
구조와 범위를 바꾸지 않았다. round2는 직전 지적, 실제 RED/GREEN, delta와 영향받은 계약을
독립 세션에 전달한다. root 지시에 따라 최종 heavy battery와 delta 리뷰는 순서대로 실행한다.
`parallel_speed` 패턴은 round1의 focused 검사와 리뷰에 적용했지만 벽시계 절약량은
별도로 측정하지 않아 수치로 주장하지 않는다.
