package issueops

import (
	"context"
	"fmt"

	"issueops/internal/contract/issueops"
	publicationcontract "issueops/internal/contract/issueopspublication"
	"issueops/internal/port"
)

var externalIntentBucket = fmt.Sprintf("external_intent_v%d", issueops.IssueOpsSchemaVersion)

const (
	externalIntentRemotePR = publicationcontract.RemoteIntentKind
)

type RemoteArtifactVerifyFunc func(issueops.IssueOpsRemoteArtifactVerificationRequest) error

type RemotePullRequestDependencies struct {
	Handler RemotePullRequestCreateHandler
}

// CreateRemotePullRequest는 IssueOps v1의 유일한 PR/MR 생성 경로다. provider를
// 호출하기 전에 정확한 operation intent 하나를 영속화하며, 모호한 호출은 절대
// 재시도하지 않는다.
func CreateRemotePullRequest(ctx context.Context, stateRoot string, req RemotePullRequestRequest, deps RemotePullRequestDependencies) (port.IssueProviderCreatePullRequestResult, error) {
	if deps.Handler == nil {
		return port.IssueProviderCreatePullRequestResult{}, ErrRemotePullRequestCreateHandlerUnavailable
	}
	if req.Confirm {
		actor, err := normalizeNativeActor(req.Actor)
		if err != nil {
			return port.IssueProviderCreatePullRequestResult{}, err
		}
		req.Actor = actor
	}
	return deps.Handler(ctx, stateRoot, req)
}
