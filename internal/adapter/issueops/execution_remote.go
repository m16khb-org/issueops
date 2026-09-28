package issueops

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/adapter/issueops/artifactverify"
	"issueops/internal/contract/issueops"
	publicationcontract "issueops/internal/contract/issueopspublication"
	publicationdomain "issueops/internal/domain/issueopspublication"
	"issueops/internal/domain/issueopsremote"
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

func validateRemotePullRequestCandidate(record issueops.IssueOpsRecord, payload publicationcontract.IntentPayload, candidate port.IssueProviderReconcilePullRequestCandidate) error {
	request := publicationcontract.ProviderCreateRequest{
		ProjectKey: payload.Request.ProjectKey, Title: payload.Request.Title, Body: payload.Request.Body,
		HeadBranch: payload.Request.HeadBranch, BaseBranch: payload.Request.BaseBranch,
		ExpectedHeadSHA: payload.Request.ExpectedHeadSHA, Labels: payload.Request.Labels,
		Assignees: payload.Request.Assignees, Draft: payload.Request.Draft,
	}
	observed := publicationcontract.Candidate{
		URL: candidate.URL, ProjectKey: candidate.ProjectKey, SourceProjectKey: candidate.SourceProjectKey,
		HeadBranch: candidate.HeadBranch, BaseBranch: candidate.BaseBranch, HeadSHA: candidate.HeadSHA,
		Title: candidate.Title, BodySHA256: candidate.BodySHA256, Labels: candidate.Labels,
		Assignees: candidate.Assignees, Draft: candidate.Draft, State: candidate.State,
	}
	if err := publicationdomain.ValidateCandidate(request, observed, payload.KnownURL); err != nil {
		return err
	}
	if err := remote.ValidateArtifactURL(candidate.URL, payload.Provider, payload.Kind); err != nil {
		return err
	}
	codeProjectKey := ""
	if record.BranchPrepare != nil {
		codeProjectKey = record.BranchPrepare.CodeProjectKey
	}
	return remote.ValidateArtifactMatchesProject(
		remote.EffectiveProjectKey(codeProjectKey, record.IssueURL, payload.Provider),
		candidate.URL, payload.Provider, payload.Kind)
}

func verifyRemotePullRequestResult(record issueops.IssueOpsRecord, payload publicationcontract.IntentPayload, url string, verify RemoteArtifactVerifyFunc) error {
	req := issueops.IssueOpsRemoteArtifactVerificationRequest{
		Provider: payload.Provider, Kind: payload.Kind, URL: strings.TrimSpace(url),
		Labels: payload.Request.Labels, Assignees: payload.Request.Assignees, TargetBranch: payload.Request.BaseBranch,
	}
	if _, err := artifactverify.Projection(record, req); err != nil {
		return err
	}
	if verify != nil {
		return verify(req)
	}
	return nil
}
