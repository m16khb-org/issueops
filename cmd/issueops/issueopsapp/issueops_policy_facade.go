package issueopsapp

import (
	"issueops/cmd/issueops/issueopscli"
	"issueops/cmd/issueops/issueopscli/remotecmd"
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	"issueops/internal/adapter/issueops"
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
		Prepare: execution.Prepare, Orca: execution.Orca, OrcaOwner: execution.OrcaOwner,
		BaseSync: basesyncoutbound.NewInspector(basesyncoutbound.RunGit), ReadIssue: execution.ReadIssue,
		Claim: issueops.ExecutionClaimHandler(issueOpsClaimHandler), Release: issueops.ExecutionReleaseHandler(issueOpsReleaseHandler),
		Reseed: issueops.ExecutionReseedHandler(issueOpsReseedHandler), Resume: issueops.ExecutionResumeHandler(issueOpsResumeHandler),
		Reconcile: issueops.ExecutionReconcileHandler(issueOpsReconcileHandler), Complete: issueops.ExecutionCompleteHandler(issueOpsCompleteHandler),
		Provenance: provenanceadapter.NewExecutableObserver(),
		Publication: remotecmd.PublicationHandlers{
			Create:    issueops.RemotePullRequestCreateHandler(issueOpsPublicationCreateHandler),
			Reconcile: issueops.RemotePullRequestReconcileHandler(issueOpsPublicationReconcileHandler),
		},
	}
}
