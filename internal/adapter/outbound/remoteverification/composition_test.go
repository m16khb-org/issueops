package remoteverification

import (
	"context"
	app "issueops/internal/application/remoteverification"
	issueopscontract "issueops/internal/contract/issueops"
	model "issueops/internal/contract/remoteverification"
	"net/url"
)

func testService() app.Service              { return app.Service{Reader: Reader{}} }
func VerifyChildIssueLive(url string) error { return testService().Child(url) }
func VerifyGitHubIssueLive(url string) error {
	return (Reader{}).GitHubChild(context.Background(), url)
}
func VerifyGitLabIssueLive(url *url.URL) error { return testService().Child(url.String()) }
func VerifyRemoteArtifactLive(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return testService().Verify(req)
}
func VerifyRemoteArtifactLiveContext(ctx context.Context, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return testService().VerifyContext(ctx, req)
}
func VerifyRemoteArtifactMergedLive(ctx context.Context, artifact issueopscontract.IssueOpsRemoteArtifactVerification) error {
	return testService().Merged(ctx, artifact)
}
func VerifyRemoteArtifactMergedHeadLive(ctx context.Context, artifact issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
	return testService().MergedHead(ctx, artifact)
}
func fetchGitLabIssueArtifact(url string) (model.Artifact, error) {
	return fetchGitLabIssueArtifactContext(context.Background(), url)
}
func fetchGitLabMergeRequestArtifact(url string) (model.Artifact, error) {
	return fetchGitLabMergeRequestArtifactContext(context.Background(), url)
}
