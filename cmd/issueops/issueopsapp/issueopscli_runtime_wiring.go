package issueopsapp

import (
	"context"
	"issueops/cmd/issueops/issueopscli"
	branchpreflight "issueops/internal/adapter/preflight"
	"os"
	"time"

	issueopscore "issueops/internal/adapter/issueops"
	branchapp "issueops/internal/application/issueopsbranch"
	reviewapp "issueops/internal/application/issueopsreview"
	issueopscontract "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

// IssueOps CLI는 사이클 저장소 구현을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func newIssueOpsCLIRuntime(stateRoot string) issueopscli.IssueOpsCLIDeps {
	observer := issueOpsCLIRecordObserver(os.Getenv("TRACEPARENT"), os.Stderr)
	readiness := newCycleReadiness()
	changes := newChangeReader()
	artifacts := issueOpsArtifactHandlers(observer)
	decisions := issueOpsDecisionHandlers(observer)
	routing := issueOpsRoutingHandlers(observer)
	return issueopscli.IssueOpsCLIDeps{
		AcceptIssueOpsChildWithActor: func(root, parentID, childID string, evidence []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error) {
			return newChildValidator(root).Accept(context.Background(), parentID, childID, evidence, &actor)
		},
		AddIssueOpsDecisionWithActor: decisions.AddWithActor,
		DropIssueOpsChildWithActor: func(root, parentID, childID, reason string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error) {
			return newChildValidator(root).Drop(context.Background(), parentID, childID, reason, &actor)
		},
		IssueOpsChildStatusWithActor: func(root, id string, repair bool, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildStatusResult, error) {
			return newChildStatusService(root).Status(context.Background(), id, repair, &actor)
		},
		IssueOpsPRReadiness: readiness.PR,
		IssueOpsNext:        issueOpsNextHandler(artifacts.Names, observer),
		IssueOpsStateRoot:   func() string { return stateRoot },
		IssueOpsStatus:      issueOpsStatusHandler(observer),
		LinkIssueOpsChildWithActor: func(root, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newIssueLinker(root).Child(context.Background(), id, childURL, title, &actor)
		},
		LinkIssueOpsIssueWithActor: func(root, id, issueURL string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newIssueLinker(root).Issue(context.Background(), id, issueURL, &actor)
		},
		LinkIssueOpsPlanWithActor: func(root, id, planPath string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newWorkspaceLinker(root).Plan(context.Background(), id, planPath, &actor)
		},
		LinkIssueOpsRelatedWithActor: func(root, id, linkType, relatedURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newIssueLinker(root).Related(context.Background(), id, linkType, relatedURL, title, &actor)
		},
		LinkIssueOpsWorktreeWithActor: func(root, id, worktreePath string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newWorkspaceLinker(root).Worktree(context.Background(), id, worktreePath, &actor)
		},
		ListIssueOpsCycles: issueOpsInventoryListHandler(observer),
		IssueOpsReviewMetrics: func(stateRoot, id, repo string) (issueopscontract.IssueOpsReviewMetricsResult, error) {
			return (reviewapp.MetricsReader{ReadRecord: issueopscore.ReadIssueOps, Now: time.Now,
				ListCycleIDs: issueOpsCycleIDLister(observer),
			}).Read(stateRoot, id, repo)
		},
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		PrepareIssueOpsBranchWithActor: func(root, id string, req issueopscontract.IssueOpsBranchPrepareRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newBranchPreparer(root).Prepare(context.Background(), id, req, &actor)
		},
		RetargetIssueOpsBranchWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsBranchRetargetRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newBranchRetargeter(stateRoot, newRemoteVerifier().ObserveTarget).Retarget(context.Background(), id, req, actor)
		},
		AwaitIssueOpsBranchLink: func(ctx context.Context, stateRoot string, req issueopscontract.AwaitBranchLinkRequest) (issueopscontract.AwaitBranchLinkResult, error) {
			return (branchapp.LinkAwaiter{
				Load: func(id string) (issueopscontract.IssueOpsRecord, error) {
					return issueopscore.ReadIssueOps(stateRoot, id)
				},
				RemoteRef:             (issueopscore.LinkedBranchRemoteRef{}).Observe,
				ObserveLinkedBranches: issueopscore.ObserveGitHubLinkedBranches(issueopscore.LiveProviderCLI),
				Sleep:                 issueopscore.SleepWithContext, Now: time.Now,
			}).Await(ctx, req)
		},
		PruneIssueOps: issueOpsRetentionPruneHandler(observer),
		ReadIssueOps:  issueopscore.ReadIssueOps,
		RecordIssueOpsAISlopCleanEvidenceWithActor: func(root, id string, categories, verification []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.RecordAISlopCleanEvidence(reviewport.AISlopCleanStore{ReviewMutationStore: issueopscore.NewReviewMutationStore(&actor), Refresh: newCyclePhaseService(&actor).Refresh}, root, id, categories, verification)
		},
		RecordIssueOpsCompatibilityReviewWithActor: func(root, id string, req issueopscontract.IssueOpsCompatibilityReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newPlanningRecorder(&actor).Compatibility(root, id, req)
		},
		RecordIssueOpsDesignReviewWithActor: func(root, id string, req issueopscontract.IssueOpsDesignReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newPlanningRecorder(&actor).Design(root, id, req)
		},
		RecordIssueOpsDevilsAdvocateReviewWithActor: func(root, id string, req issueopscontract.IssueOpsDevilsAdvocateReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newPlanningRecorder(&actor).DevilsAdvocate(root, id, req)
		},
		RecordIssueOpsDomainReviewWithActor: func(root, id string, req issueopscontract.IssueOpsDomainReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.RecordDomainReview(issueopscore.NewReviewMutationStore(&actor), root, id, req)
		},
		RecordIssueOpsImplementationReviewWithActor: func(root, id string, req issueopscontract.IssueOpsImplementationReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.RecordImplementationReview(issueopscore.NewEvidenceReviewStore(&actor, changes.ChangeFingerprint), root, id, req)
		},
		RecordIssueOpsProjectDocsReviewWithActor: func(root, id string, req issueopscontract.IssueOpsProjectDocsReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.RecordProjectDocsReview(reviewport.ProjectDocsReviewStore{EvidenceReviewStore: issueopscore.NewEvidenceReviewStore(&actor, changes.ChangeFingerprint), ChangedPaths: changes.ChangedPaths, Root: issueopscore.ReviewDocumentPaths{}.Root, RelativePath: issueopscore.ReviewDocumentPaths{}.RelativePath, FileExists: issueopscore.ReviewDocumentPaths{}.FileExists}, root, id, req)
		},
		RecordIssueOpsSchemaEvidenceWithActor: func(root, id string, req issueopscontract.IssueOpsSchemaEvidenceRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.RecordSchemaEvidence(issueopscore.NewEvidenceReviewStore(&actor, changes.ChangeFingerprint), root, id, req)
		},
		RecordIssueOpsIntentWithActor: func(root, id string, req issueopscontract.IssueOpsIntentRecordRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newPlanningRecorder(&actor).Intent(root, id, req)
		},
		RecordIssueOpsPlanPrepWithActor: func(root, id string, req issueopscontract.IssueOpsPlanPrepRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newPlanningRecorder(&actor).PlanPrep(root, id, req)
		},
		RecordIssueOpsRoutingWithActor: routing.Record,
		RegressIssueOpsForReplanWithActor: func(root, id, reason string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newPlanningRecorder(&actor).Regress(root, id, reason)
		},
		RejectIssueOpsChildWithActor: func(root, parentID, childID, reason string, evidence []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error) {
			return newChildValidator(root).Reject(context.Background(), parentID, childID, reason, evidence, &actor)
		},
		ResolveIssueOpsFeedbackWithActor: func(root, id string, index int, resolution string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.ResolveFeedback(issueopscore.NewReviewMutationStore(&actor), root, id, index, resolution)
		},
		ScoreLiveRoutingFidelity:    routing.Score,
		StageIssueOpsArtifact:       artifacts.Stage,
		StagedIssueOpsArtifactNames: artifacts.Names,
		StartIssueOps: func(stateRoot string, req issueopscontract.IssueOpsStartRequest) (issueopscontract.IssueOpsRecord, error) {
			return (branchapp.Starter{Records: issueopscore.CycleRecordStore{StateRoot: stateRoot}, Identity: issueopscore.CycleStartIdentity{RunGit: branchpreflight.GitCmd}, Now: time.Now}).Start(context.Background(), req)
		},
		StartIssueOpsChildWithActor: func(root string, req issueopscontract.IssueOpsChildStartRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildStartResult, error) {
			return newChildStarter(root).Start(context.Background(), req, &actor)
		},
		UnstageIssueOpsArtifact: artifacts.Unstage,
	}
}
