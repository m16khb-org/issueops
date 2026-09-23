---
name: 2026-09-23-the-lease-contract-decodes-persisted-records-through-the-pro
description: Accepted decision record with rationale, alternatives, and consequences.
---

# The lease contract decodes persisted records through the production record contract

- Date: 2026-09-23
- Kind: `adr`
- Source: architecture review 2026-09-23
- Summary: internal/contract/issueopslease는 IssueOpsRecord 전체를 복제한 stable v1 사본 대신 internal/contract/issueops의 production record DTO로 persisted JSON을 엄격하게 decode한다. 2026-07-28 결정을 대체한다.
- Context: 2026-07-28 ADR은 test-only differential prototype이 internal/core/issueops/model을 import하지 않도록 stable v1 JSON shape를 복제하게 했다. 그 뒤 prototype이 production release vertical이 됐고(2026-07-29), internal/core는 사라졌으며 record DTO는 internal/contract/issueops로 옮겨졌다. contract 간 참조는 2026-08-08 결정으로 허용됐다. 사본(431줄)을 지키던 leasevertical 규칙은 존재하지 않는 internal/core/issueops만 검사해 발동할 수 없었고, 생성 이후 feat·fix 커밋 14건이 schema를 바꿀 때마다 사본을 함께 고쳤다. differential 테스트는 2026-08-03(49606394)에 이미 제거됐다.
- Decision: Decode는 production IssueOpsRecord로 DisallowUnknownFields decode한 뒤 canonical JSON을 lease Record로 옮긴다. lease Record는 execution만 typed로 다루고 나머지 sidecar는 json.RawMessage로 원문 그대로 보존한다. execution 하위 typed 타입 7개는 execution_sidecars.go의 lease contract 자체 타입으로 남긴다. stable_v1.go와 죽은 leasevertical_contract_must_not_import_production_issueops 규칙을 지운다.
- Consequences: 모르는 field 거부는 그대로다(사본에만 있는 field가 없음을 확인했다). production execution의 모든 field가 lease Execution 타입에 있어야 re-encode가 field를 버리지 않으므로 TestLeaseExecutionShapeCoversEveryPersistedExecutionField가 이를 대조한다. TestDecodeRejectsFieldsThePersistedRecordDoesNotDefine과 TestDecodeEncodePreservesEverySidecar가 엄격성과 sidecar 보존을 고정한다. canonical JSON은 production DTO가 쓰는 형식과 같아진다. 전환 중 lease Record에 linked_branch_cleanup field가 없어 release·claim·complete·reseed·prepare가 그 감사 기록을 버리던 기존 결함을 찾았고, field를 추가하고 TestLeaseRecordCarriesEveryPersistedTopLevelField로 모든 최상위 field를 대조한다.
- Evidence:
  - internal/contract/issueopslease/record.go Decode
  - internal/contract/issueopslease/execution_sidecars.go
  - internal/contract/issueopslease/record_preservation_test.go
  - internal/architecture/dependency_test.go
- Alternatives / rejected options:
  - 사본을 유지한다: schema 변경마다 같은 필드를 두 곳에 추가하는 비용이 계속된다
  - raw JSON patch로 execution.lease만 교체한다: 모르는 field 거부와 null 정규화 규칙이 바뀌어 새 fixture와 검증 경로가 필요하다
