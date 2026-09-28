package issueops

import (
	"context"
	"time"

	cleanupapp "issueops/internal/application/issueopscleanup"
	completionapp "issueops/internal/application/issueopsremote"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

type CleanupRemoteBranchDeps struct {
	Git                  func(ctx context.Context, dir string, args ...string) (int, string)
	VerifyMergedArtifact func(artifact issueops.IssueOpsRemoteArtifactVerification) (issueops.CleanupRemoteBranchArtifactHead, error)
	// ReflectAudit는 삭제 성공 사실을 이슈 본문 completion 섹션에 멱등 병합한다
	// (finish ④'의 CleanupAudit 병합 선례). best-effort이며 실패해도 이미 끝난
	// 원격 삭제를 되돌리지 않는다.
	ReflectAudit func(record issueops.IssueOpsRecord, completion issueops.RemoteCompletionSection, audit string) error
	// ObserveArtifact는 replacement 증거를 provider에서 읽는다. 주입되지 않으면
	// 그 경로는 열리지 않는다 — 관측 없이 증거를 인정하지 않는다(#323).
	ObserveArtifact func(url string) (issueopsdomain.ArtifactObservation, error)
}

func remoteBranchPreviewer(deps CleanupRemoteBranchDeps) cleanupapp.RemoteBranchPreviewer {
	return cleanupapp.RemoteBranchPreviewer{Environment: CleanupRemoteBranchEnvironment{RunGit: deps.Git}, VerifyMergedArtifact: deps.VerifyMergedArtifact, ObserveArtifact: deps.ObserveArtifact}
}
func CleanupRemoteBranch(ctx context.Context, root string, req CleanupRemoteBranchRequest, deps CleanupRemoteBranchDeps) (CleanupRemoteBranchResult, error) {
	s := cleanupapp.RemoteBranchCleaner{Records: CleanupRecordStore{StateRoot: root}, Acquire: (CleanupLifetimeLock{StateRoot: root}).Acquire, NewAttempt: NewCleanupAttempt, Preview: remoteBranchPreviewer(deps), Completion: completionapp.NewCompletionCollector(CompletionArtifacts{}).Collect, Now: time.Now}
	if deps.ReflectAudit != nil {
		s.ReflectAudit = func(_ context.Context, rec issueops.IssueOpsRecord, completion issueops.RemoteCompletionSection, audit string) error {
			return deps.ReflectAudit(rec, completion, audit)
		}
	}
	return s.Run(ctx, req)
}
