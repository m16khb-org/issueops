package issueopsapp

import (
	"context"
	"issueops/cmd/issueops/issueopscli"
	"issueops/cmd/issueops/issueopscli/remoteverify"
	issueopsadapter "issueops/internal/adapter/issueops"
	healthadapter "issueops/internal/adapter/operationalhealth"
	orcaadapter "issueops/internal/adapter/orca"
	cleanupapp "issueops/internal/application/issueopscleanup"
	issueopscontract "issueops/internal/contract/issueops"
	health "issueops/internal/contract/operationalhealth"
)

func newIssueOpsCLIGates() issueopscli.LoopGateDeps {
	gate := newGateReadiness()
	return issueopscli.LoopGateDeps{
		AdvancePhaseWithActor: func(stateRoot, id, to string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return gate.AdvancePhaseWithActor(stateRoot, id, to, actor)
		},
		StrictPRReadinessWithState: gate.StrictPRReadinessWithState,
	}
}

func orphanCleaner(root string) cleanupapp.OrphanCleaner {
	environment := issueopsadapter.OrphanEnvironment{StateRoot: root}
	collector := newOperationalHealthCollector(root, environment, nil)
	environment.LocalInventory = func(ctx context.Context, repo string) (health.Snapshot, error) {
		return collector.CollectLocal(ctx, repo), nil
	}
	return cleanupapp.OrphanCleaner{Environment: environment, Collect: func(ctx context.Context, repo string) (health.Snapshot, error) {
		return (newOperationalHealthCollector(root, healthadapter.ExecGitRunner{}, orcaadapter.New())).Collect(ctx, repo), nil
	}, VerifyMerged: remoteverify.VerifyRemoteArtifactMergedLive}
}
