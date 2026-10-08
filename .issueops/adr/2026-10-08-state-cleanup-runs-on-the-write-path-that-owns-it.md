---
name: 2026-10-08-state-cleanup-runs-on-the-write-path-that-owns-it
description: Accepted decision record with rationale, alternatives, and consequences.
---

# State cleanup runs on the write path that owns it

- Date: 2026-10-08
- Kind: `adr`
- Source: issueops-docs io-7dff0c24f1e9 (#558)
- Summary: 채널 메시지 보존 정리는 channel send가, 은퇴한 subsystem의 state 잔재 정리는 install/update의 commit 뒤 단계가 맡는다. state maintain과 state prune은 지금 계약대로 둔다.
- Context: #558 이전에는 channel_v1 메시지와 MCP HTTP server.log, 제거된 daemon·hook log·maintain stamp·migration receipt가 user state에 무한히 남았다. state prune은 state bucket의 key 목록을 kept/pruned로 보고하는 계약이고 채널은 별도 root(<state>/channel)에 있다. state maintain은 비파괴(checkpoint·chmod)로 문서화돼 있고 MCP 도구 설명이 Omo·omp·agy catalog SHA에 묶여 있다.
- Decision: 1) channel send가 쓰기 직후 7일이 지난 메시지를 sqlstore.DB.DeleteBefore 한 번으로 지운다. 메시지 ID가 생성 시각 nanosecond hex로 시작하므로 'msg-<cutoff hex>' 미만이 곧 오래된 메시지다. prune 실패는 send 결과를 바꾸지 않는다. 2) 은퇴 경로 allowlist는 statedomain.RetiredStateEntries가 소유한다. state doctor는 retired_path로 보고하고, install/update는 activation seal과 Finalize 뒤에만 지운다. 정확한 이름·종류만 Lstat로 확인하고 symlink는 건드리지 않으며, legacy daemon이 socket이나 PID로 살아 있으면 daemon/을 남긴다. 3) MCP HTTP 서비스 로그는 unit의 리다이렉트를 유지한 채 프로세스가 8MiB copytruncate로 제한한다.
- Consequences: 7일이 지난 채널 메시지는 다음 send에서 사라지며, 지워진 since 커서는 ADR 2026-08-22 결정 2대로 처음부터 읽는다. install/update는 되돌릴 수 없는 파일 삭제를 하므로 dry-run이 'would remove retired state path ...'로 미리 보여 준다. 새 subsystem을 제거할 때는 그 state 잔재를 RetiredStateEntries에 추가한다.
- Evidence:
  - internal/application/channel/service.go Send → Writer.PruneBefore
  - internal/adapter/outbound/sqlstore/sqlstore.go DeleteBefore
  - internal/domain/state/doctor.go RetiredStateEntries
  - internal/application/install/transaction.go RemoveRetiredState after Finalize
  - internal/adapter/install/retired_state.go
  - internal/adapter/mcpservice/server_log.go
  - TestSendPrunesMessagesPastRetention, TestRunTransactionRemovesRetiredStateOnlyAfterFinalize, TestRemoveRetiredStateKeepsDaemonWhileLegacyProcessIsAlive, TestCappedLogWriterCopyTruncatesPastLimit
- Alternatives / rejected options:
  - state prune에 채널 포함: prune 출력 계약에 모든 메시지가 섞이고 사람이 실행하지 않으면 정리되지 않는다
  - state maintain에서 은퇴 경로 삭제: 비파괴 계약과 MCP 도구 설명(catalog SHA)이 바뀐다
  - unit에서 로그 리다이렉트 제거 후 프로세스 자체 파일 로깅: panic 출력이 사라지고 unit 형식이 바뀐다
