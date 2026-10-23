---
name: 2026-10-23-annotate-every-mcp-tool-budget-skill-bodies-retain-cycle-outcomes
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Annotate every MCP tool, budget skill bodies, retain cycle outcomes, and mark channel origin

- Date: 2026-10-23
- Kind: `adr`
- Source: [2026-10 AI 동향 조사](../research/2026-10-23-ai-updates-issueops-candidates.md)
- Summary: 51개 MCP 도구가 모두 행동 hint를 선언하고, `SKILL.md` 본문은 Agent Skills 명세 예산 안에 두며, cleanup은 record를 지우는 트랜잭션에서 지표 요약을 남기고, 채널 메시지는 harness가 기록한 `origin`을 가진다. MCP Skills·Tasks 확장은 SDK와 호스트 지원이 생길 때까지 도입하지 않는다.
- Context: 조사 시점에 도구 51개 중 2개만 annotation을 가졌다. Claude Code 2.1.302는 `readOnlyHint`가 없는 MCP 도구를 병렬 실행하지 않고 읽기 전용으로 보지 않으며, 2.1.307부터 끊긴 HTTP 호출은 read-only·idempotent 도구만 다시 보낸다. 스킬 7개는 본문이 명세 권고(5000 token 미만)를 넘었다. cleanup finish가 record를 지워 `review-metrics --repo`가 완료된 사이클을 하나도 보지 못했다. 채널의 `from`은 자기 신고 값인데 받는 쪽이 이를 구분할 표시가 없었다. go-sdk v1.8.0에는 Skills(SEP-2640)·Tasks 확장이 없고, Claude Code는 Skills 확장을 기본 꺼진 플래그 뒤에 둔다.
- Decision: 1) 모든 도구가 `readOnlyHint`를 명시하고, 쓰기 도구는 `LocalWriteAnnotations(additive, idempotent)` 또는 `UnclaimedWriteAnnotations`로 주장 범위를 정한다. 옵션으로라도 쓰는 도구는 read-only가 아니다. 2) `scripts/validate-skill.py`가 본문 500줄 미만, 추정 5000 token 이하, description 1024자 이하를 강제하고, 넘친 세부는 `references/`로 옮긴다. 3) cleanup finish·abandon은 `issueops_cycle_outcome_v1` bucket에 `IssueOpsCycleOutcome`(review-metrics 지표, repo, outcome, retained_at)을 record 삭제와 같은 CAS 트랜잭션으로 쓴다. review-metrics는 live record를 우선하고 나머지를 요약에서 읽는다. 4) 채널 `SendRequest.Origin`은 JSON으로 받지 않고 CLI는 `cli`, MCP는 `mcp`로 채운다. 그 밖의 값은 `origin_unknown`으로 거부한다. 5) Skills·Tasks 확장은 go-sdk가 확장을 구현하고 호스트 하나 이상이 기본으로 켤 때 다시 판단한다.
- Consequences: MCP 도구 설명과 annotation이 바뀌어 catalog SHA가 달라지므로 Omo·omp·agy 설정은 `issueops update`로 다시 써야 한다. 사이클 요약은 완료된 사이클마다 한 행씩 쌓이고 아직 정리하지 않는다. 채널 메시지의 `from`은 여전히 검증되지 않으며, `origin`도 같은 OS 사용자 신뢰 경계 안의 관측일 뿐이다.
- Evidence:
  - internal/contract/mcp/output_schemas.go annotation 생성자, internal/adapter/inbound/catalog/mcp/structured_output_test.go TestEveryToolDeclaresReadOnlyHint
  - scripts/validate-skill.py MAX_BODY_TOKENS·MAX_BODY_LINES·MAX_DESCRIPTION_LENGTH, scripts/validate_skill_test.py
  - internal/adapter/issueops/cycle_outcome_records.go, internal/adapter/issueops/cleanup_finish_records_test.go TestFinishRecordsDrainFailureRearmAndAtomicDelete
  - internal/application/issueopsreview/metrics.go, TestMetricsReaderAddsRetainedOutcomesWithoutDoubleCountingLiveRecords
  - internal/domain/channel/messages.go NormalizeSend origin_unknown, internal/adapter/channel/store_test.go TestSendRecvRoundTripAcrossChannelIsolation
- Alternatives / rejected options:
  - 구조화 출력 도구에만 annotation 유지: 나머지 49개가 호스트에서 쓰기 도구로 취급된다
  - record를 지우지 않고 완료 상태로 보존: cleanup 소유권·CAS 계약과 state 정리 결정(2026-10-08)을 되돌린다
  - 채널 발신자를 native actor 관측으로 인증: `execution whoami`가 세션 식별 모호로 실패하는 환경이 있어 send가 그 실패에 묶인다
  - Skills 확장을 go-sdk middleware로 직접 구현: SDK가 모르는 method를 받을 경로가 없고 기본으로 켠 클라이언트가 없어 검증할 수 없다
