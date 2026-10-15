package issueopscli

import (
	"context"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	provideradapter "issueops/internal/adapter/provider"
	model "issueops/internal/contract/issueops"
)

func testCleanupRuntime() feedbackcleanup.Deps {
	orphans := orphanCleaner()
	return feedbackcleanup.Deps{
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
