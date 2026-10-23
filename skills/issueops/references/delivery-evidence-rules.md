# 구현·검증 증거 규칙

domain 검토에는 invariant,
exact mechanism, equivalent behavior, source 근거를 분리해 적는다. 구현 전 환경 표에는
Environment / Repo-config evidence / Runtime evidence / Failure path / Remediation order를
채운다. feedback은 classification, verification, thread reply와 resolution을 추적한다.
완료 기록에는 diff, verification, labels, children, draft URL, thread status, cleanup,
follow-ups를 남겨 사람이 검사할 수 있게 한다.

domain 계약은 구현 전에 issue 또는 plan의 evidence 절에 기록한다. exact mechanism이
없다는 사실과 다른 경로가 같은 invariant를 보장하는지는 별개로 판정한다. end-to-end
동작까지 미확인·반박된 것이 아니면 단순한 mechanism 부재를 기능 부재로 단정하지 않는다.
endpoint/controller/DTO/schema/OpenAPI 또는 public error 동작이 바뀌면 계획과 PR/MR
초안에 변경 endpoint(또는 계약 변경 없음), usecase/error mapping의 도달 가능한 public
error, 대상 repo의 static/API-doc review 결과와 관련 검증 명령을 기록한다. 해당 명령이
없으면 부재와 가장 가까운 static/review 검사를 명시하고 무관한 legacy 부채와 구분한다.

live matrix에서는 source diff와 실제 DB/config/env/pod/log 증거를 분리하고 비슷해 보이는
failure path를 한 원인으로 합치지 않는다. local-only 관측과 same-network/in-workload
관측을 구분하며, 실제 runtime 확인 가능 여부와 여러 수정의 순서를 기록한다.
feedback classification은 contract_change/defect/question/noise/valid_review/stale_review/
rollout_evidence_missing/environment_debt, resolution은 unresolved/fixed/resolved/obsolete/
split to follow-up으로 남긴다. 검증된 feedback만 적용하고 수용 기준·non-goals·검증·labels·
링크·구현 범위가 바뀌면 계속 진행하기 전에 remote issue body를 갱신한다.

ready/done을 보고하기 전에 actual diff, source/target branch, remote issue/PR/MR body의
최신성, issue label의 복사 또는 명시적 교체, single-commit 정책 또는 다중 commit 사유,
branch divergence와 clean worktree, cleanup 상태 또는 번호 붙인 선택지를 확인한다.
최종 보고와 remote write 전에 위 증거를 담은 inspectable draft completion record를 만든다.
테스트 성공만으로 remote 갱신·thread reply·merge readiness·cleanup 완료를 대신하지 않는다.

## publication 전 문서 대조와 스키마 증거

- publication 전에 구현 diff를 project docs와 양방향으로 대조한다. CONSTITUTION·
  CONVENTIONS·ARCHITECTURE를 어겼으면 구현을 고치고, CAUTIONS에 남길 재발 함정이나
  ADR에 남길 결정이 생겼으면 문서를 먼저 고친 뒤 기록한다. 남길 것이 없으면 실제로
  읽은 문서 경로(`--reviewed-doc`)와 확인 근거를 함께 `no-change`로 기록한다.
- 변경 집합에 마이그레이션·엔티티·SQL 스키마 파일이 있으면 실제 데이터베이스에서
  인덱스 현황과 카탈로그 추정 row 수를 관찰해 관찰값과 출처를 기록한다. 전수 스캔하지 않는다.
  관찰 불가면 근거 있는 waive로 남긴다. 문서·스키마 증거 뒤 구현 리뷰를 기록하고,
  변경 집합이 바뀌면 봉인을 재검증한다.
