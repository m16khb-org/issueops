package issueopsapp

import (
	"context"
	"issueops/cmd/issueops/issueopscli"
	issueopsadapter "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/gatesgate"
	healthadapter "issueops/internal/adapter/operationalhealth"
	cleanupapp "issueops/internal/application/issueopscleanup"
	issueopscontract "issueops/internal/contract/issueops"
	orphancontract "issueops/internal/contract/issueopsorphancleanup"
	health "issueops/internal/contract/operationalhealth"
)

// issueops CLI는 고아 정리와 루프 게이트 구현을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureIssueOpsOrphanAndLoopGate() {
	issueopscli.ConfigureOrphanCleanup(issueopscli.OrphanCleanupDeps{
		Preview: func(ctx context.Context, req orphancontract.Request, deps issueopscli.OrphanDependencies) (orphancontract.Result, error) {
			return orphanCleaner(deps).Preview(ctx, req)
		},
		Apply: func(ctx context.Context, req orphancontract.Request, apply orphancontract.ApplyRequest, deps issueopscli.OrphanDependencies) (orphancontract.Result, error) {
			return orphanCleaner(deps).Apply(ctx, req, apply)
		},
	})
	issueopscli.ConfigureLoopGate(issueopscli.LoopGateDeps{
		AdvancePhaseWithActor: func(stateRoot, id, to string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return gatesgate.AdvancePhaseWithActor(stateRoot, id, to, actor)
		},
		StrictPRReadinessWithState: gatesgate.StrictPRReadinessWithState,
	})
}

func orphanCleaner(deps issueopscli.OrphanDependencies) cleanupapp.OrphanCleaner {
	environment := issueopsadapter.OrphanEnvironment{StateRoot: issueopsadapter.IssueOpsStateRoot()}
	collector := healthadapter.Collector{Git: environment}
	environment.LocalInventory = func(ctx context.Context, repo string) (health.Snapshot, error) {
		return collector.CollectLocal(ctx, repo), nil
	}
	return cleanupapp.OrphanCleaner{Environment: environment, Collect: deps.Collect, VerifyMerged: deps.VerifyMerged}
}
