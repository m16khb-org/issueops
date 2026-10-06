package issueopscli

import (
	"context"
	ownerapp "issueops/internal/application/issueopsowner"
	reviewapp "issueops/internal/application/issueopsreview"
	"os"
	"path/filepath"
	"time"

	"issueops/cmd/issueops/issueopscli/executioncmd"
	issueopsartifactinbound "issueops/internal/adapter/inbound/issueopsartifact"
	issueopsdecisioninbound "issueops/internal/adapter/inbound/issueopsdecision"
	issueopsinventoryinbound "issueops/internal/adapter/inbound/issueopsinventory"
	issueopsretentioninbound "issueops/internal/adapter/inbound/issueopsretention"
	issueopsroutinginbound "issueops/internal/adapter/inbound/issueopsrouting"
	issueopsstatusinbound "issueops/internal/adapter/inbound/issueopsstatus"
	issueopscore "issueops/internal/adapter/issueops"
	issueopsartifactoutbound "issueops/internal/adapter/outbound/issueopsartifact"
	issueopsauthorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	issueopsdecisionoutbound "issueops/internal/adapter/outbound/issueopsdecision"
	issueopsinventoryoutbound "issueops/internal/adapter/outbound/issueopsinventory"
	issueopsretentionoutbound "issueops/internal/adapter/outbound/issueopsretention"
	issueopsroutingoutbound "issueops/internal/adapter/outbound/issueopsrouting"
	issueopsstatusoutbound "issueops/internal/adapter/outbound/issueopsstatus"
	issueopsartifactapplication "issueops/internal/application/issueopsartifact"
	issueopsdecisionapplication "issueops/internal/application/issueopsdecision"
	issueopsinventoryapplication "issueops/internal/application/issueopsinventory"
	issueopsretentionapplication "issueops/internal/application/issueopsretention"
	issueopsroutingapplication "issueops/internal/application/issueopsrouting"
	issueopsstatusapplication "issueops/internal/application/issueopsstatus"
	"issueops/internal/domain/agentmodel"
	issueopsstatusdomain "issueops/internal/domain/issueopsstatus"

	issueopsnextinbound "issueops/internal/adapter/inbound/issueopsnext"
	issueopsnextapplication "issueops/internal/application/issueopsnext"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

// 프로덕션에서는 issueopsapp이 주입한다. IssueOps CLI 계약 테스트는 실제 사이클
// 저장소를 검증하므로 같은 배선을 재현한다.
func wireIssueOpsRuntimeForTests() {
	artifacts := issueopsartifactinbound.NewHandlers(
		issueopsartifactapplication.NewService(issueopsartifactoutbound.Repository{}),
	)
	decisions := issueopsdecisioninbound.NewHandlers(issueopsdecisionapplication.NewService(
		issueopsdecisionoutbound.Repository{},
		issueopsdecisionoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
		issueopscore.NativeActorVerifier(),
	))
	inventory := issueopsinventoryapplication.NewService(
		issueopsinventoryoutbound.Repository{},
		issueopsinventoryoutbound.SystemClock{},
		issueopsinventoryoutbound.CleanPath{},
	)
	retention := issueopsretentionapplication.NewService(
		issueopsretentionoutbound.Repository{},
		issueopsretentionoutbound.SystemClock{},
	)
	status := issueopsstatusapplication.NewService(
		issueopsstatusoutbound.Repository{},
		issueopsstatusdomain.NewProjector(testCycleReadiness().Completion),
	)
	routing := issueopsroutinginbound.NewHandlers(issueopsroutingapplication.NewService(
		issueopsroutingoutbound.Repository{},
		issueopsroutingoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
		issueopscore.NativeActorVerifier(),
	))
	listCycles := issueopsinventoryinbound.NewListHandler(inventory)
	next := issueopsnextapplication.NewService(issueopsnextapplication.Ports{
		ListCycles: func(ctx context.Context, stateRoot, repo string) (issueopsinventorycontract.ListResult, error) {
			return listCycles(stateRoot, repo)
		},
		ReadRecord: issueopscore.ReadIssueOps,
		Completion: testCycleReadiness().Completion,
		LocalReadiness: func(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
			ready, _ := testCycleReadiness().ObserveLocalPR(record)
			return ready
		},
		WriterlessCommand: ownerapp.WriterlessCommand,
		PlannerDefaults:   agentmodel.PlannerDefaults,
		StagedArtifacts:   artifacts.Names,
		Actor: func() (string, string, error) {
			host, sessionID, _, err := executioncmd.ResolveNativeSessionIdentity(os.Getenv)
			return host, sessionID, err
		},
		SourceRoot: issueopsinventoryoutbound.CleanPath{}.Normalize,
		CleanPath:  filepath.Clean,
		Env:        os.Getenv,
		Now:        time.Now,
	})
	testIssueOpsRuntime = IssueOpsCLIDeps{
		AcceptIssueOpsChildWithActor:  acceptChildWithActorForTest,
		AddIssueOpsDecisionWithActor:  decisions.AddWithActor,
		DropIssueOpsChildWithActor:    dropChildWithActorForTest,
		IssueOpsChildStatusWithActor:  childStatusWithActorForTest,
		IssueOpsPRReadiness:           testCycleReadiness().PR,
		IssueOpsNext:                  issueopsnextinbound.NewNextHandler(next),
		IssueOpsStateRoot:             issueOpsStateRootForTest,
		IssueOpsStatus:                issueopsstatusinbound.NewStatusHandler(status),
		LinkIssueOpsChildWithActor:    LinkIssueOpsChildWithActorForTest,
		LinkIssueOpsIssueWithActor:    LinkIssueOpsIssueWithActorForTest,
		LinkIssueOpsPlanWithActor:     LinkIssueOpsPlanWithActorForTest,
		LinkIssueOpsRelatedWithActor:  LinkIssueOpsRelatedWithActorForTest,
		LinkIssueOpsWorktreeWithActor: LinkIssueOpsWorktreeWithActorForTest,
		ListIssueOpsCycles:            listCycles,
		IssueOpsReviewMetrics: func(stateRoot, id, repo string) (issueopscontract.IssueOpsReviewMetricsResult, error) {
			return (reviewapp.MetricsReader{ReadRecord: issueopscore.ReadIssueOps, Now: time.Now,
				ListCycleIDs: func(stateRoot, repo string) ([]string, []string, error) {
					result, err := listCycles(stateRoot, repo)
					if err != nil {
						return nil, nil, err
					}
					ids := make([]string, 0, len(result.Entries))
					for _, entry := range result.Entries {
						ids = append(ids, entry.ID)
					}
					return ids, append([]string(nil), result.UnreadableIDs...), nil
				},
			}).Read(stateRoot, id, repo)
		},
		ObserveNativeProcessAncestry:               issueopscore.ObserveNativeProcessAncestry,
		PrepareIssueOpsBranchWithActor:             prepareBranchWithActorForTest,
		PruneIssueOps:                              issueopsretentioninbound.NewPruneHandler(retention),
		ReadIssueOps:                               issueopscore.ReadIssueOps,
		RecordIssueOpsAISlopCleanEvidenceWithActor: recordAISlopEvidenceForTest,
		RecordIssueOpsCompatibilityReviewWithActor: func(root, id string, req issueopscontract.IssueOpsCompatibilityReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return planningRecorderForTest(&actor).Compatibility(root, id, req)
		},
		RecordIssueOpsDesignReviewWithActor: func(root, id string, req issueopscontract.IssueOpsDesignReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return planningRecorderForTest(&actor).Design(root, id, req)
		},
		RecordIssueOpsDevilsAdvocateReviewWithActor: func(root, id string, req issueopscontract.IssueOpsDevilsAdvocateReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return planningRecorderForTest(&actor).DevilsAdvocate(root, id, req)
		},
		RecordIssueOpsDomainReviewWithActor: func(root, id string, req issueopscontract.IssueOpsDomainReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.RecordDomainReview(issueopscore.NewReviewMutationStore(&actor), root, id, req)
		},
		RecordIssueOpsImplementationReviewWithActor: recordImplementationReviewForTest,
		RecordIssueOpsProjectDocsReviewWithActor:    recordProjectDocsReviewForTest,
		RecordIssueOpsSchemaEvidenceWithActor:       recordSchemaEvidenceForTest,
		RecordIssueOpsIntentWithActor: func(root, id string, req issueopscontract.IssueOpsIntentRecordRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return planningRecorderForTest(&actor).Intent(root, id, req)
		},
		RecordIssueOpsPlanPrepWithActor: func(root, id string, req issueopscontract.IssueOpsPlanPrepRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return planningRecorderForTest(&actor).PlanPrep(root, id, req)
		},
		RecordIssueOpsRoutingWithActor: routing.Record,
		RegressIssueOpsForReplanWithActor: func(root, id, reason string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return planningRecorderForTest(&actor).Regress(root, id, reason)
		},
		RejectIssueOpsChildWithActor: rejectChildWithActorForTest,
		ResolveIssueOpsFeedbackWithActor: func(root, id string, index int, resolution string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.ResolveFeedback(issueopscore.NewReviewMutationStore(&actor), root, id, index, resolution)
		},
		ScoreLiveRoutingFidelity:    routing.Score,
		StageIssueOpsArtifact:       artifacts.Stage,
		StagedIssueOpsArtifactNames: artifacts.Names,
		StartIssueOps:               startIssueOpsFixture,
		StartIssueOpsChildWithActor: startChildWithActorForTest,
		UnstageIssueOpsArtifact:     artifacts.Unstage,
	}
}
