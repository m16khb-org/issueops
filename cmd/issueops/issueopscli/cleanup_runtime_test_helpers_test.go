package issueopscli

import (
	"context"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	orcaadapter "issueops/internal/adapter/orca"
	provideradapter "issueops/internal/adapter/provider"
	model "issueops/internal/contract/issueops"
)

func testCleanupRuntime() feedbackcleanup.Deps {
	orphans := orphanCleaner()
	orca := orcaadapter.New()
	execution := orcaadapter.NewExecution()
	return feedbackcleanup.Deps{
		RemoveOrcaWorktree: func(ctx context.Context, id string) error {
			return orcaadapter.NormalizeRemoveWorktreeError(orca.RemoveWorktree(ctx, id, false))
		},
		OrcaIntent: execution, OrcaOwner: execution,
		VerifyMerged: func(a model.IssueOpsRemoteArtifactVerification) error {
			return testRemoteVerifier().Merged(context.Background(), a)
		},
		VerifyMergedHead: func(a model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
			return testRemoteVerifier().MergedHead(context.Background(), a)
		},
		ObserveArtifactMerged: testRemoteVerifier().ObserveMerged,
		Provider:              provideradapter.Resolve,
		OrphanPreview:         orphans.Preview, OrphanApply: orphans.Apply,
	}
}
