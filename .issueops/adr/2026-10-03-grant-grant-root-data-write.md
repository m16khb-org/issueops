---
name: 2026-10-03-grant-grant-root-data-write
description: Accepted decision record with rationale, alternatives, and consequences.
---

# 요청 grant 재검사는 grant root의 data write 트랜잭션에서 한다

- Date: 2026-10-03
- Kind: `adr`
- Source: agent improvements final review F1-F3
- Summary: HTTP capability 요청의 grant 재검사를 첫 span이 아니라 grant root DB의 span과 모든 data write(Apply, CompareAndApply, CompareAndApplyFunc)에 묶는다.
- Context: 처음 구현은 요청 guard를 어느 root든 처음 열린 span에서 한 번 소비했다. 그래서 resume·reseed는 grant가 없는 reseed fence DB가 guard를 소비해 유효한 capability가 revoked로 거부됐고(F1), reconcile은 canonicalization span 뒤의 raw CompareAndApply가 재검사 없이 커밋돼 철회된 grant로도 쓰기가 남았다(F3). 근거: .issueops/plans/agent-improvements-2026-10-02/final-review.md, final-fixes-verification.md.
- Decision: sqlstore.WithRecordGuard(ctx, root, bind)는 grant root(issueOpsStateRoot())의 span에서만 bind한다. 다른 root의 span은 guard를 통과시킨다. grant root의 data write는 mutation을 적용한 뒤 commit 직전에 같은 data transaction을 reader로 grant를 다시 검사하고, 실패하면 rollback한다. capability가 없는 native stdio와 CLI 요청은 영향이 없다.
- Consequences: grant 회전·삭제와 lifecycle mutation이 SQLite write lock으로 직렬화되고, 새 쓰기 경로도 grant root에 쓰면 자동으로 재검사된다. grant root가 아닌 store(loop, worker)에 쓰는 경로는 계속 grant root span을 먼저 여는 fence가 필요하다. 테스트: TestReconcileReceiptRechecksRequestGuardAfterGrantChange, TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions, sqlstore RecordGuard 5건.
