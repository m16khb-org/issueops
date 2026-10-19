---
name: 2026-10-19-approve-the-architecture-before-the-plan-and-rename-the-desi
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Approve the architecture before the plan and rename the design stage to architecture

- Date: 2026-10-19
- Kind: `adr`
- Source: issueops-570
- Summary: plan 단계 안의 순서를 plan.architecture, plan.write, plan.review, plan.handoff로 바꿔 아키텍처 문서를 계획보다 먼저 승인하고, design이라는 단계·명령·기록 이름을 architecture로 바꾼다.
- Context: 계획을 다 쓴 뒤에 설계 검토를 기록하면 대안 비교가 결정보다 늦어서 이미 내린 결정을 정당화하는 데 그치고, 구조 결함은 plan.review의 적대 리뷰에서야 드러나 TODO와 게이트까지 쓴 계획 전체를 다시 쓰게 된다. 2026-06-18 결정(issueops-plan-prep-evidence-gate)은 design review를 plan phase 안의 활동으로 두었지만 계획 뒤에 오는 순서까지 정했다. 계획 필수 절 네 개에는 규모, 데이터와 API 계약, 장애 시나리오를 적을 자리가 없었다.
- Decision: 1) durable phase는 늘리지 않고 plan 안의 순서만 plan.architecture, plan.write, plan.review, plan.handoff로 바꾼다. 2026-06-18 결정 가운데 순서에 관한 부분을 이 결정이 대체하고, plan phase 안의 활동이라는 부분은 그대로 둔다. 2) 아키텍처는 새 artifact 이름 architecture로 스테이징한다. 문서에는 요구사항과 규모, 데이터와 API 계약, 구조와 흐름, 병목과 장애, 절충점과 기각한 대안 다섯 절이 있어야 하고 본문이 비면 architecture review 기록이 빠진 절 제목과 함께 거부한다. 3) 승인은 문서 sha256(reviewed_architecture_digest), reviewer_context(subagent 또는 inline), history에 묶는다. 승인 뒤 문서가 바뀌면 architecture_review_stale로 plan.architecture에 머문다. 부모 리뷰를 승계한 위임 자식은 reviewer_context delegated-parent-review로 이 게이트에서 빠진다. 4) stage 키, CLI 명령, record 키, 게이트 키, Go 타입, 공용 스킬의 design 이름을 architecture로 바꾸고 별칭과 옛 키 읽기 경로를 두지 않는다. 기각한 대안: 새 durable phase 추가(phase 열거형, ledger, regress 대상, 다섯 호스트 스킬을 모두 바꾸는데 결과는 plan 안의 순서 변경과 같다), 이름은 두고 순서만 변경(이름 변경도 요청 범위다), spec artifact 재사용(spec은 superpowers 스펙을 이슈에 붙인다는 다른 뜻이 있다), 옛 record 키 읽기 호환(2026-10-07 결정과 충돌하고 로컬에 남은 기록이 이 사이클뿐이었다).
- Consequences: 옛 바이너리가 쓴 design_review 키가 있는 record는 새 바이너리에서 invalid state이므로 그 사이클은 abandon 후 다시 시작한다. 이 사이클의 lifecycle 명령은 끝까지 옛 바이너리로 실행한다. 공용 스킬 design-review는 architecture-review로 바뀌고 사용자 홈의 옛 링크는 다음 issueops update가 정리한다. response contract golden, usage golden, DDD inventory, pioneer manifest가 바뀐다. 되돌리려면 PR을 revert하고, 새 바이너리가 쓴 architecture_review record는 옛 바이너리에서도 invalid state가 되므로 그 사이클을 새로 시작한다.
