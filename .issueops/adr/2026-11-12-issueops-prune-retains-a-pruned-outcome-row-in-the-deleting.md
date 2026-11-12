---
name: 2026-11-12-issueops-prune-retains-a-pruned-outcome-row-in-the-deleting
description: Accepted decision record with rationale, alternatives, and consequences.
---

# issueops prune retains a pruned outcome row in the deleting transaction

- Date: 2026-11-12
- Kind: `adr`
- Source: claude-code main session
- Summary: issueops prune --confirm은 지우는 record마다 outcome이 pruned인 CycleOutcome 보존 행을 record 삭제와 같은 CAS commit에 쓴다.
- Context: ADR 2026-10-23 결정 3은 cleanup finish·abandon이 record를 지우는 같은 트랜잭션에서 지표 요약을 issueops_cycle_outcome_v1 bucket에 남기게 했다. issueops prune은 record만 지웠다. 그래서 cleanup 없이 30일이 지나 prune된 done 사이클은 review-metrics와 view --view timeline에서 사라졌다. 2026-11-12에 저장소 소유자가 prune도 보존 행을 남기는 안을 골랐다.
- Decision: 1) outcome 값 pruned를 더한다. CycleOutcomeSchemaVersion은 1로 둔다. 2) application의 retention 서비스가 --confirm일 때만 record마다 CycleOutcomeForRecord로 outcome을 만든다. 미리보기는 아무것도 만들지도 쓰지도 않는다. 3) outbound issueopsrecord store의 DeleteIfUnchanged가 쓰기 목록을 받아 record 삭제와 같은 CompareAndApply에 넣는다. CAS가 실패하면 record도 보존 행도 반영되지 않는다. 인코딩이 실패하면 트랜잭션을 열기 전에 끝난다. 4) 보존 행 인코더는 contract의 codec 함수 EncodeCycleOutcome 하나다. cleanup과 prune이 함께 쓴다. bucket 이름은 port.CycleOutcomeBucket 한 곳에 둔다. 5) review-metrics --repo의 retention 객체에 pruned 수를 더하고 0이어도 낸다. pruned 행도 last_retained_at 후보다. record가 없을 때의 안내는 pruned 행이면 "prune removed it"으로 쓴다. 6) 보존 행은 ID당 하나다. 같은 ID의 사이클이 다시 지워지면 최신 outcome으로 덮어쓴다. 이 동작은 cleanup에서 이미 그랬고 이번에 문서와 테스트로 고정했다.
- Consequences: prune은 이제 지운 record마다 최대 64KiB 보존 행 하나를 남긴다. 보존 행은 prune 대상이 아니며 줄이는 경로가 없다. 옛 binary는 pruned 행을 review-metrics의 cycles에는 넣고 retention 수에서는 뺀다. 안내 문구는 옛 binary에서 "cleanup removed it (outcome pruned ...)"로 나온다. prune된 ID의 not-found 안내는 state 경로 안내 대신 보존 행 안내가 된다. response-contract golden의 issueops_review_metrics에 pruned가 생겼다. prune의 나이 판정은 여전히 UpdatedAt만 본다(범위 밖). ADR 2026-10-23 결정 3과 2026-11-12 cycle views 결정을 넓힌다.
- Evidence:
  - internal/application/issueopsretention/service.go, internal/adapter/outbound/issueopsrecord/store.go (DeleteIfUnchanged), internal/contract/issueops/cycle_outcome_codec.go, internal/port/record_buckets.go
  - TestDeleteIfUnchangedRollsBackWhenAWriteFailsInCommit, TestRepositoryDeleteIfUnchangedKeepsRecordWhenOutcomeWriteFailsInCommit (a write that fails inside the commit leaves the record)
  - TestIssueOpsPruneRetainsOutcomeThroughCLI (production wiring: preview writes nothing, prune --confirm raises retention.pruned 0 -> 1, view reads the retained timeline)
  - nine mutations each fail a named test, including delete-then-separate-write in the adapter and in the store
- Alternatives / rejected options:
  - 지금 동작 유지(문서에 한계만 적음): 소유자가 지표와 타임라인을 남기는 쪽을 골랐다
  - prune 결과에 지표 요약을 돌려주고 저장하지 않음: 호출한 쪽이 보관해야 하고 review-metrics와 view가 읽을 수 없다
  - composition root가 인코더를 retention outbound에 주입: 주입 누락이라는 실패 경로가 생기고 배선이 늘어난다. contract codec이 더 단순하다
  - 삭제 뒤 별도 트랜잭션으로 보존 행 쓰기: record는 지워졌는데 보존 행이 없는 상태가 생긴다
  - CycleOutcome schema 버전 올리기: bucket 이름이 바뀌어 옛 행을 읽지 못한다
  - 보존 행 키를 ID+시각으로 바꾸어 덮어쓰기를 없앰: 읽는 경로 전부가 바뀌는 별도 설계다
- Note (2026-11-12, 후속 변경): Consequences의 "prune의 나이 판정은 여전히 UpdatedAt만 본다"는 더 이상 맞지 않는다. domain의 IsPrunable은 이제 `issueops list`가 `last_record_at`으로 보여 주는 마지막 기록 시각(issueopsinventory.LastRecordAt, record에 저장된 시각 중 가장 늦은 값)이 cutoff보다 앞설 때만 참이다. UpdatedAt이 파싱되지 않거나 0인 record는 전처럼 지우지 않는다. 마지막 기록 시각은 UpdatedAt보다 앞서지 않으므로, 예전 규칙이 남기던 record를 새 규칙이 지우는 일은 없다. 같은 변경에서 보존 행을 읽는 세 곳(review-metrics·view의 행 읽기, record not-found 안내 두 곳)이 EncodeCycleOutcome 옆의 DecodeCycleOutcome 하나를 쓴다. not-found 안내는 schema나 ID가 다른 행에서 안내를 붙이지 않는다. 증거: TestIsPrunableDecidesAgeFromTheLastRecordInstant, TestIsPrunableNeverPrunesARecordTheUpdatedAtRuleKept, TestIssueOpsPruneKeepsRecordWithRecentLastRecordInstant, TestDecodeCycleOutcomeIsStrictAboutSchemaAndCycle, TestReadIssueOpsMissingRecordNotesOnlyAStrictRetainedOutcome, TestReadNotFoundIgnoresARetainedRowThatDoesNotDecodeStrictly.
