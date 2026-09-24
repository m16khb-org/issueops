package issueops

import (
	"context"
	"testing"

	"issueops/internal/contract/issueops"
)

// 정리 명령은 이슈 본문을 쓰지 않는다(#513). 그래서 reflect-completion이 남긴
// 반영 상태를 cleanup remote-branch가 바꾸지도, 지우지도 않는다. issueops list는
// 이 캐시로 반영 여부를 보고한다(#128).
func TestCleanupPathsKeepAuditOutOfIssueBody(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	const reflectedAt = "2026-09-24T00:00:00Z"
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.RemoteCompletion = &issueops.IssueOpsRemoteCompletion{ReflectedAt: reflectedAt}
	})
	deps := remoteBranchDeps(remoteBranchGit())
	preview, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil || !result.Deleted || result.Audit == "" {
		t.Fatalf("remote-branch apply must delete and report the audit: err=%v %+v", err, result)
	}
	got, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteCompletion == nil || got.RemoteCompletion.ReflectedAt != reflectedAt {
		t.Fatalf("cleanup must leave the reflected state as reflect-completion set it: %+v", got.RemoteCompletion)
	}

	finishRoot, finishRecord, _ := finishTestRecord(t, true)
	finishDepsValue := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	finishPreview, err := CleanupFinish(context.Background(), finishRoot, finishRequest(finishRecord.ID, false, ""), finishDepsValue)
	if err != nil {
		t.Fatal(err)
	}
	finished, err := CleanupFinish(context.Background(), finishRoot, finishRequest(finishRecord.ID, true, finishPreview.Fingerprint), finishDepsValue)
	if err != nil || finished.Audit == "" {
		t.Fatalf("finish apply must report the audit: err=%v %+v", err, finished)
	}
}
