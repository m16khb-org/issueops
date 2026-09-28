package issueopsapp

import (
	"context"
	"os"
	"time"

	"issueops/cmd/issueops/issueopscli"
	"issueops/cmd/issueops/issueopscli/remoteverify"
	issueopscore "issueops/internal/adapter/issueops"
	branchapp "issueops/internal/application/issueopsbranch"
	issueopscontract "issueops/internal/contract/issueops"
)

// IssueOps CLI는 사이클 저장소 구현을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureIssueOpsCLIRuntime() {
	observer := issueOpsRecordObserver(os.Stderr)
	artifacts := issueOpsArtifactHandlers(observer)
	decisions := issueOpsDecisionHandlers(observer)
	routing := issueOpsRoutingHandlers(observer)
	issueopscli.ConfigureIssueOpsRuntime2(issueopscli.IssueOpsCLIDeps{
		AcceptIssueOpsChildWithActor:  issueopscore.AcceptIssueOpsChildWithActor,
		AddIssueOpsDecisionWithActor:  decisions.AddWithActor,
		DropIssueOpsChildWithActor:    issueopscore.DropIssueOpsChildWithActor,
		IssueOpsChildStatusWithActor:  issueopscore.IssueOpsChildStatusWithActor,
		IssueOpsPRReadiness:           issueopscore.IssueOpsPRReadiness,
		IssueOpsNext:                  issueOpsNextHandler(artifacts.Names, observer),
		IssueOpsStateRoot:             issueopscore.IssueOpsStateRoot,
		IssueOpsStatus:                issueOpsStatusHandler(observer),
		LinkIssueOpsChildWithActor:    issueopscore.LinkIssueOpsChildWithActor,
		LinkIssueOpsIssueWithActor:    issueopscore.LinkIssueOpsIssueWithActor,
		LinkIssueOpsPlanWithActor:     issueopscore.LinkIssueOpsPlanWithActor,
		LinkIssueOpsRelatedWithActor:  issueopscore.LinkIssueOpsRelatedWithActor,
		LinkIssueOpsWorktreeWithActor: issueopscore.LinkIssueOpsWorktreeWithActor,
		ListIssueOpsCycles:            issueOpsInventoryListHandler(observer),
		IssueOpsReviewMetrics: func(stateRoot, id, repo string) (issueopscontract.IssueOpsReviewMetricsResult, error) {
			return issueopscore.ReviewMetrics(stateRoot, id, repo, issueopscore.ReviewMetricsDeps{
				ListCycleIDs: issueOpsCycleIDLister(observer),
			})
		},
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		PrepareIssueOpsBranchWithActor: func(root, id string, req issueopscontract.IssueOpsBranchPrepareRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newBranchPreparer(root).Prepare(context.Background(), id, req, &actor)
		},
		RetargetIssueOpsBranchWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsBranchRetargetRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newBranchRetargeter(stateRoot, remoteverify.ObserveRemoteArtifactTargetLive).Retarget(context.Background(), id, req, actor)
		},
		AwaitIssueOpsBranchLink: func(ctx context.Context, stateRoot string, req issueopscontract.AwaitBranchLinkRequest) (issueopscontract.AwaitBranchLinkResult, error) {
			return issueopscore.AwaitBranchLink(ctx, stateRoot, req, issueopscore.AwaitBranchLinkDeps{
				ObserveLinkedBranches: issueopscore.ObserveGitHubLinkedBranches(issueopscore.LiveProviderCLI),
			})
		},
		PruneIssueOps: issueOpsRetentionPruneHandler(observer),
		ReadIssueOps:  issueopscore.ReadIssueOps,
		RecordIssueOpsAISlopCleanEvidenceWithActor:  issueopscore.RecordIssueOpsAISlopCleanEvidenceWithActor,
		RecordIssueOpsCompatibilityReviewWithActor:  issueopscore.RecordIssueOpsCompatibilityReviewWithActor,
		RecordIssueOpsDesignReviewWithActor:         issueopscore.RecordIssueOpsDesignReviewWithActor,
		RecordIssueOpsDevilsAdvocateReviewWithActor: issueopscore.RecordIssueOpsDevilsAdvocateReviewWithActor,
		RecordIssueOpsDomainReviewWithActor:         issueopscore.RecordIssueOpsDomainReviewWithActor,
		RecordIssueOpsImplementationReviewWithActor: issueopscore.RecordIssueOpsImplementationReviewWithActor,
		RecordIssueOpsProjectDocsReviewWithActor:    issueopscore.RecordIssueOpsProjectDocsReviewWithActor,
		RecordIssueOpsSchemaEvidenceWithActor:       issueopscore.RecordIssueOpsSchemaEvidenceWithActor,
		RecordIssueOpsIntentWithActor:               issueopscore.RecordIssueOpsIntentWithActor,
		RecordIssueOpsPlanPrepWithActor:             issueopscore.RecordIssueOpsPlanPrepWithActor,
		RecordIssueOpsRoutingWithActor:              routing.Record,
		RegressIssueOpsForReplanWithActor:           issueopscore.RegressIssueOpsForReplanWithActor,
		RejectIssueOpsChildWithActor:                issueopscore.RejectIssueOpsChildWithActor,
		ResolveIssueOpsFeedbackWithActor:            issueopscore.ResolveIssueOpsFeedbackWithActor,
		ScoreLiveRoutingFidelity:                    routing.Score,
		StageIssueOpsArtifact:                       artifacts.Stage,
		StagedIssueOpsArtifactNames:                 artifacts.Names,
		StartIssueOps: func(stateRoot string, req issueopscontract.IssueOpsStartRequest) (issueopscontract.IssueOpsRecord, error) {
			return (branchapp.Starter{Records: issueopscore.CycleRecordStore{StateRoot: stateRoot}, Identity: issueopscore.CycleStartIdentity{}, Now: time.Now}).Start(context.Background(), req)
		},
		StartIssueOpsChildWithActor: issueopscore.StartIssueOpsChildWithActor,
		UnstageIssueOpsArtifact:     artifacts.Unstage,
	})
}
