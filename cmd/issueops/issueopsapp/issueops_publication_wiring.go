package issueopsapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"issueops/cmd/issueops/issueopscli"
	publicationinbound "issueops/internal/adapter/inbound/issueopspublication"
	"issueops/internal/adapter/issueops"
	publicationoutbound "issueops/internal/adapter/outbound/issueopspublication"
	"issueops/internal/adapter/provider"
	publicationapp "issueops/internal/application/issueopspublication"
	remoteapp "issueops/internal/application/issueopsremote"
	publicationcontract "issueops/internal/contract/issueopspublication"
	"issueops/internal/port"
)

type issueOpsPublicationCompositionDeps struct {
	Resolve        func(string) (port.IssueProvider, error)
	VerifyLive     issueops.RemoteArtifactVerifyFunc
	Now            func() time.Time
	NewOperationID func() (string, error)
}

func productionIssueOpsPublicationDeps() issueOpsPublicationCompositionDeps {
	return issueOpsPublicationCompositionDeps{
		Resolve: provider.Resolve, VerifyLive: issueopscli.VerifyRemoteArtifactLive, Now: time.Now,
	}
}

func issueOpsPublicationCreateHandler(ctx context.Context, stateRoot string, request issueops.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return newIssueOpsPublicationHandlers(productionIssueOpsPublicationDeps()).Create(ctx, stateRoot, request)
}

func issueOpsPublicationReconcileHandler(ctx context.Context, stateRoot string, request issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
	return newIssueOpsPublicationHandlers(productionIssueOpsPublicationDeps()).Reconcile(ctx, stateRoot, request)
}

func newIssueOpsPublicationHandlers(deps issueOpsPublicationCompositionDeps) issueops.RemotePublicationHandlers {
	return issueops.RemotePublicationHandlers{
		Create: func(ctx context.Context, stateRoot string, request issueops.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			create, _ := newIssueOpsPublicationServices(stateRoot, deps)
			return publicationinbound.NewCreateHandler(create)(ctx, stateRoot, request)
		},
		Reconcile: func(ctx context.Context, stateRoot string, request issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
			_, reconcile := newIssueOpsPublicationServices(stateRoot, deps)
			return publicationinbound.NewReconcileHandler(reconcile)(ctx, stateRoot, request)
		},
	}
}

func newIssueOpsPublicationServices(stateRoot string, deps issueOpsPublicationCompositionDeps) (*publicationapp.CreateService, *publicationapp.ReconcileService) {
	observer := issueops.RemotePublicationObserver{StateRoot: stateRoot, Clock: deps.Now, OperationIDFactory: deps.NewOperationID}
	repository := remoteapp.NewPublicationJournal(issueops.RemotePublicationStore{StateRoot: stateRoot}, observer)
	providerAdapter := &publicationProviderAdapter{deps: deps}
	gateway := publicationoutbound.NewProviderGateway(providerAdapter.create, providerAdapter.inspect)
	verifier := remoteapp.NewPublicationVerifier(issueops.RemotePublicationStore{StateRoot: stateRoot}, deps.VerifyLive)
	preparer := remoteapp.NewCreatePreparation(observer)
	return publicationapp.NewCreateService(preparer, repository, gateway, verifier), publicationapp.NewReconcileService(repository, gateway, verifier)
}

type publicationProviderAdapter struct {
	deps issueOpsPublicationCompositionDeps
}

func (e *publicationProviderAdapter) create(ctx context.Context, providerName string, request publicationcontract.ProviderCreateRequest) (publicationcontract.ProviderCreateResult, error) {
	if e.deps.Resolve == nil {
		return publicationcontract.ProviderCreateResult{}, fmt.Errorf("publication provider resolver is required")
	}
	resolved, err := e.deps.Resolve(providerName)
	if err != nil {
		return publicationcontract.ProviderCreateResult{}, err
	}
	result, err := issueops.CreateRemotePullRequestViaProviderContext(ctx, portPublicationRequest(request), resolved)
	return publicationcontract.ProviderCreateResult{
		OK: result.OK, URL: result.URL, Number: result.Number, Preview: result.Preview,
	}, err
}

func (e *publicationProviderAdapter) inspect(ctx context.Context, intent publicationcontract.Intent) (publicationcontract.Inventory, bool, error) {
	if e.deps.Resolve == nil {
		return publicationcontract.Inventory{}, false, fmt.Errorf("publication provider resolver is required")
	}
	resolved, err := e.deps.Resolve(intent.Provider)
	if err != nil {
		return publicationcontract.Inventory{}, false, err
	}
	result, err := issueops.ReconcileRemotePullRequestViaProviderContext(ctx, publicationReconcileRequest(intent.Request), resolved)
	inventory := publicationcontract.Inventory{AuthoritativeZero: result.AuthoritativeZero}
	if result.Candidates != nil {
		inventory.Candidates = make([]publicationcontract.Candidate, len(result.Candidates))
		for index, candidate := range result.Candidates {
			inventory.Candidates[index] = publicationCandidate(candidate)
		}
	}
	return inventory, true, err
}

func portPublicationRequest(request publicationcontract.ProviderCreateRequest) port.IssueProviderCreatePullRequestRequest {
	return port.IssueProviderCreatePullRequestRequest{
		Repo: request.Repo, ProjectKey: request.ProjectKey, Title: request.Title, Body: request.Body,
		HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch,
		Labels: clonePublicationStrings(request.Labels), Assignees: clonePublicationStrings(request.Assignees),
		Draft: request.Draft, ExpectedHeadSHA: request.ExpectedHeadSHA, Confirm: request.Confirm,
		Host: request.Host, SessionID: request.SessionID, AgentID: request.AgentID, CWD: request.CWD,
	}
}

func publicationReconcileRequest(request publicationcontract.ProviderCreateRequest) port.IssueProviderReconcilePullRequestRequest {
	sum := sha256.Sum256([]byte(request.Body))
	return port.IssueProviderReconcilePullRequestRequest{
		Repo: request.Repo, ProjectKey: request.ProjectKey, HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch,
		ExpectedHeadSHA: request.ExpectedHeadSHA, Title: request.Title, BodySHA256: hex.EncodeToString(sum[:]),
		Labels: clonePublicationStrings(request.Labels), Assignees: clonePublicationStrings(request.Assignees), Draft: request.Draft,
	}
}

func publicationCandidate(candidate port.IssueProviderReconcilePullRequestCandidate) publicationcontract.Candidate {
	return publicationcontract.Candidate{
		URL: candidate.URL, ProjectKey: candidate.ProjectKey, SourceProjectKey: candidate.SourceProjectKey,
		HeadBranch: candidate.HeadBranch, BaseBranch: candidate.BaseBranch, HeadSHA: candidate.HeadSHA,
		Title: candidate.Title, BodySHA256: candidate.BodySHA256,
		Labels: clonePublicationStrings(candidate.Labels), Assignees: clonePublicationStrings(candidate.Assignees),
		Draft: candidate.Draft, State: candidate.State,
	}
}

func clonePublicationStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
