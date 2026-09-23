---
name: 2026-09-23-issueopsrecord-field-lease-record
description: Caution record for a solved false case or recurring risk.
---

# IssueOpsRecord에 최상위 field를 추가하면 lease Record에도 추가한다

- Date: 2026-09-23
- Kind: `caution`
- Source: architecture review 2026-09-23
- Summary: lease vertical의 Record에 linked_branch_cleanup이 없어 release·claim·complete·reseed·prepare가 lease 전이마다 그 감사 기록을 조용히 버렸다.
- Context: lease Record는 execution만 typed로 다루고 나머지 최상위 field는 json.RawMessage로 받아 원문 그대로 다시 쓴다. Decode는 production record를 엄격하게 읽지만 lease Record로 옮길 때는 json.Unmarshal이 모르는 field를 버린다. 그래서 Record에 없는 최상위 field는 Encode에서 사라진다. 2026-09-23 독립 리뷰가 찾았다.
- Resolution: Record에 LinkedBranchCleanup json.RawMessage field를 추가했다. internal/contract/issueopslease/record_preservation_test.go의 TestLeaseRecordCarriesEveryPersistedTopLevelField가 production IssueOpsRecord의 모든 최상위 json tag가 lease Record에 있는지 대조하고, TestDecodeEncodePreservesEverySidecar가 왕복 보존을 확인한다. 새 최상위 field를 추가하면 이 테스트가 먼저 실패한다.
- Evidence:
  - internal/contract/issueopslease/record.go
  - internal/contract/issueopslease/record_preservation_test.go
