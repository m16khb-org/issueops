---
name: 2026-09-29-cleanup-finish-ownership-is-an-optional-record-field-with-gu
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Cleanup finish ownership is an optional record field with guarded writers

- Date: 2026-09-29
- Kind: `adr`
- Source: DDD responsibility refactor T08
- Summary: Represent cleanup finish attempts in the authoritative schema1 record and reject ordinary writes while an attempt is present.
- Context: The DDD cleanup migration needs to bind destructive finish effects and final deletion to the same record revision. A raw CAS alone accepts an already armed expected snapshot, while an older typed snapshot can omit the ownership field.
- Decision: Add optional cleanup_finish_attempt with one shared contract DTO containing a 64-character lowercase hexadecimal token and an RFC3339Nano started_at. Record and lease codecs validate and preserve the field. Pure domain policy rejects any non-nil attempt for ordinary mutations; adapters enforce it on current stored records and candidate snapshots, including raw CAS intent paths. Preparation clones copy the pointed value. Retention never selects an armed record, regardless of age. A future finish-only persistence boundary will own arming and conditional clearing; the current writer-fence prerequisite does not yet activate finish attempts in production.
- Consequences: This is an explicit additive persisted-state extension to the DDD plan's original unchanged-schema intent. The schema version remains 1, and records without the optional field remain readable. Older strict readers reject records carrying the new field; new readers reject malformed attempts. No legacy alias, migration command, or automatic schema promotion is introduced. Finish recovery, attempt-bound receipts and deletion still require the separate executor migration; codec and writer tests do not prove that migration complete.
- Evidence:
  - internal/contract/issueops/cleanup_finish_attempt.go
  - internal/adapter/outbound/issueopsrecord/finish_attempt_codec_test.go
  - internal/adapter/outbound/issueopsrecord/finish_fence.go
  - internal/adapter/outbound/issueopslease/finish_intent_fence_test.go
  - internal/adapter/issueops/cleanup_finish_writer_fence_test.go
  - internal/application/issueopslease/claim_transaction_test.go
  - internal/domain/issueopsretention/retention_test.go
- Alternatives / rejected options:
  - Do not overload cleanup_finish_failure with an applying state: failure and ownership have different lifetimes.
  - Do not use time-based expiry or a second durable ownership journal.
  - Do not treat raw CAS equality as permission to overwrite an armed record.
