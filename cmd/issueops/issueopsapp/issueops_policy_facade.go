package issueopsapp

import (
	"issueops/cmd/issueops/issueopscli"
	"issueops/cmd/issueops/issueopscli/remotecmd"
	issueopscontract "issueops/internal/contract/issueops"

	clicatalog "issueops/internal/adapter/inbound/catalog/cli"

	basesyncoutbound "issueops/internal/adapter/outbound/issueopsbasesync"

	provenanceadapter "issueops/internal/adapter/outbound/issueopsprovenance"
)

func runIssueOps(args []string) error {
	return issueopscli.RunIssueOpsWithDependencies(args, issueOpsCLIDependencies())
}

func issueOpsCLIDependencies() issueopscli.Dependencies {
	execution := productionIssueOpsExecutionDependencies()
	return issueopscli.Dependencies{
		Verification:   newRemoteVerificationHandlers(),
		Remote:         newIssueOpsRemote(issueOpsStateRoot()),
		Benchmark:      newBenchmarkCommand(),
		Cleanup:        newIssueOpsCleanup(issueOpsStateRoot()),
		CleanupRuntime: newIssueOpsCleanupRuntime(issueOpsStateRoot()),
		Execution:      newIssueOpsExecutionRunners(), HandoffCmux: issueOpsCmuxHandoffHandler,
		Runtime: newIssueOpsCLIRuntime(issueOpsStateRoot()), Gates: newIssueOpsCLIGates(),
		Usage: clicatalog.LifecycleUsage(), ChildUsage: clicatalog.ChildUsage(),
		Prepare:  execution.Prepare,
		BaseSync: basesyncoutbound.NewInspector(basesyncoutbound.RunGit),
		Status:   issueOpsExecutionStatusHandler, Replace: newIssueOpsReplacementHandler(),
		Claim: issueopscontract.ExecutionClaimHandler(issueOpsClaimHandler), Release: issueopscontract.ExecutionReleaseHandler(issueOpsReleaseHandler),
		Reseed:     issueopscontract.ExecutionReseedHandler(issueOpsReseedHandler),
		Complete:   issueopscontract.ExecutionCompleteHandler(issueOpsCompleteHandler),
		Provenance: provenanceadapter.NewExecutableObserver(),
		Publication: remotecmd.PublicationHandlers{
			Create:    issueopscontract.RemotePullRequestCreateHandler(issueOpsPublicationCreateHandler),
			Reconcile: issueopscontract.RemotePullRequestReconcileHandler(issueOpsPublicationReconcileHandler),
		},
	}
}
