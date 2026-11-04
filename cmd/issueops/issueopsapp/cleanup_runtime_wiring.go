package issueopsapp

import (
	"context"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	provideradapter "issueops/internal/adapter/provider"
	model "issueops/internal/contract/issueops"
)

func newIssueOpsCleanupRuntime(root string) feedbackcleanup.Deps {
	orphans := orphanCleaner(root)
	return feedbackcleanup.Deps{
		VerifyMerged: func(a model.RemoteArtifactVerification) error {
			return newRemoteVerifier().Merged(context.Background(), a)
		},
		VerifyMergedHead: func(a model.RemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
			return newRemoteVerifier().MergedHead(context.Background(), a)
		},
		ObserveArtifactMerged: newRemoteVerifier().ObserveMerged,
		Provider:              provideradapter.Resolve,
		OrphanPreview:         orphans.Preview, OrphanApply: orphans.Apply,
	}
}
