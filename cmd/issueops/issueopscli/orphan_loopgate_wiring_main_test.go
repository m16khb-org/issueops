package issueopscli

import (
	"context"

	issueopsadapter "issueops/internal/adapter/issueops"
	healthadapter "issueops/internal/adapter/operationalhealth"
	cleanupapp "issueops/internal/application/issueopscleanup"
	issueopscontract "issueops/internal/contract/issueops"
	orphancontract "issueops/internal/contract/issueopsorphancleanup"
	health "issueops/internal/contract/operationalhealth"
)

// 프로덕션에서는 issueopsapp이 주입한다. 이 계약 테스트는 실제 게이트 판정과
// 고아 정리를 검증하므로 같은 배선을 재현한다.
func wireOrphanAndLoopGateForTests() {
	ConfigureOrphanCleanup(OrphanCleanupDeps{
		Preview: func(ctx context.Context, req orphancontract.Request, deps OrphanDependencies) (orphancontract.Result, error) {
			return orphanCleaner(deps).Preview(ctx, req)
		},
		Apply: func(ctx context.Context, req orphancontract.Request, apply orphancontract.ApplyRequest, deps OrphanDependencies) (orphancontract.Result, error) {
			return orphanCleaner(deps).Apply(ctx, req, apply)
		},
	})
	testIssueOpsGates = LoopGateDeps{
		AdvancePhaseWithActor: func(stateRoot, id, to string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return advanceLoopPhaseForTest(stateRoot, id, to, actor)
		},
		StrictPRReadinessWithState: strictLoopReadinessForTest,
	}
}

func orphanCleaner(deps OrphanDependencies) cleanupapp.OrphanCleaner {
	environment := issueopsadapter.OrphanEnvironment{StateRoot: issueopsadapter.IssueOpsStateRoot()}
	collector := healthadapter.Collector{Git: environment}
	environment.LocalInventory = func(ctx context.Context, repo string) (health.Snapshot, error) {
		return collector.CollectLocal(ctx, repo), nil
	}
	return cleanupapp.OrphanCleaner{Environment: environment, Collect: deps.Collect, VerifyMerged: deps.VerifyMerged}
}
