package issueopsapp

import (
	"context"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	orcaadapter "issueops/internal/adapter/orca"
	provideradapter "issueops/internal/adapter/provider"
	model "issueops/internal/contract/issueops"
)

func newIssueOpsCleanupRuntime(root string) feedbackcleanup.Deps {
	orphans := orphanCleaner(root)
	orca := orcaadapter.New()
	execution := orcaadapter.NewExecution()
	return feedbackcleanup.Deps{
		RemoveOrcaWorktree: func(ctx context.Context, id string) error {
			return orcaadapter.NormalizeRemoveWorktreeError(orca.RemoveWorktree(ctx, id, false))
		},
		OrcaIntent: execution, OrcaOwner: execution,
		VerifyMerged: func(a model.IssueOpsRemoteArtifactVerification) error {
			return newRemoteVerifier().Merged(context.Background(), a)
		},
		VerifyMergedHead: func(a model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
			return newRemoteVerifier().MergedHead(context.Background(), a)
		},
		ObserveArtifactMerged: newRemoteVerifier().ObserveMerged,
		Provider:              provideradapter.Resolve,
		OrphanPreview:         orphans.Preview, OrphanApply: orphans.Apply,
	}
}
