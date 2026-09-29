package issueopscli

import (
	"context"
	orcaadapter "issueops/internal/adapter/orca"

	issueopsadapter "issueops/internal/adapter/issueops"
	healthadapter "issueops/internal/adapter/operationalhealth"
	cleanupapp "issueops/internal/application/issueopscleanup"
	issueopscontract "issueops/internal/contract/issueops"
	health "issueops/internal/contract/operationalhealth"
)

// 프로덕션에서는 issueopsapp이 주입한다. 이 계약 테스트는 실제 게이트 판정과
// 고아 정리를 검증하므로 같은 배선을 재현한다.
func wireOrphanAndLoopGateForTests() {
	testIssueOpsGates = LoopGateDeps{
		AdvancePhaseWithActor: func(stateRoot, id, to string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return advanceLoopPhaseForTest(stateRoot, id, to, actor)
		},
		StrictPRReadinessWithState: strictLoopReadinessForTest,
	}
}

func orphanCleaner() cleanupapp.OrphanCleaner {
	environment := issueopsadapter.OrphanEnvironment{StateRoot: issueopsadapter.IssueOpsStateRoot()}
	collector := healthadapter.Collector{Git: environment, IssueOps: healthadapter.IssueOpsReader{StateRoot: environment.StateRoot, ListIDs: issueopsadapter.ListIssueOpsIDs, ListLeaseHolders: issueopsadapter.ListLeaseHolderIndexes, Read: issueopsadapter.ReadIssueOpsExisting}, InspectNativeProcess: issueopsadapter.InspectNativeProcessReceipt}
	environment.LocalInventory = func(ctx context.Context, repo string) (health.Snapshot, error) {
		return collector.CollectLocal(ctx, repo), nil
	}
	return cleanupapp.OrphanCleaner{Environment: environment, Collect: func(ctx context.Context, repo string) (health.Snapshot, error) {
		return (healthadapter.Collector{Git: healthadapter.ExecGitRunner{}, Orca: orcaadapter.New(), IssueOps: healthadapter.IssueOpsReader{StateRoot: environment.StateRoot, ListIDs: issueopsadapter.ListIssueOpsIDs, ListLeaseHolders: issueopsadapter.ListLeaseHolderIndexes, Read: issueopsadapter.ReadIssueOpsExisting}, InspectNativeProcess: issueopsadapter.InspectNativeProcessReceipt}).Collect(ctx, repo), nil
	}, VerifyMerged: testRemoteVerifier().Merged}
}
