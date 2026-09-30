package issueops

import (
	"context"
	"time"

	cleanupapp "issueops/internal/application/issueopscleanup"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

type CleanupRemoteBranchDeps struct {
	Git                  func(ctx context.Context, dir string, args ...string) (int, string)
	VerifyMergedArtifact func(artifact issueops.IssueOpsRemoteArtifactVerification) (issueops.CleanupRemoteBranchArtifactHead, error)
	// ObserveArtifact는 replacement 증거를 provider에서 읽는다. 주입되지 않으면
	// 그 경로는 열리지 않는다 — 관측 없이 증거를 인정하지 않는다(#323).
	ObserveArtifact func(url string) (issueopsdomain.ArtifactObservation, error)
}

func remoteBranchPreviewer(deps CleanupRemoteBranchDeps) cleanupapp.RemoteBranchPreviewer {
	return cleanupapp.RemoteBranchPreviewer{Environment: CleanupRemoteBranchEnvironment{RunGit: deps.Git}, VerifyMergedArtifact: deps.VerifyMergedArtifact, ObserveArtifact: deps.ObserveArtifact}
}
func CleanupRemoteBranch(ctx context.Context, root string, req issueops.CleanupRemoteBranchRequest, deps CleanupRemoteBranchDeps) (issueops.CleanupRemoteBranchResult, error) {
	s := cleanupapp.RemoteBranchCleaner{Records: CleanupRecordStore{StateRoot: root}, Acquire: (CleanupLifetimeLock{StateRoot: root}).Acquire, NewAttempt: NewCleanupAttempt, Preview: remoteBranchPreviewer(deps), Now: time.Now}
	return s.Run(ctx, req)
}
