package issueopsapp

import (
	"issueops/internal/adapter/docs"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/preflight"
	"issueops/internal/adapter/projectdocs"

	"issueops/cmd/issueops/basiccli"
	"issueops/cmd/issueops/channelcli"
	"issueops/cmd/issueops/gatescli"
	"issueops/cmd/issueops/installcli"
	"issueops/cmd/issueops/loopcli"
	"issueops/cmd/issueops/projectcli"
	"issueops/cmd/issueops/qualitycli"
	"issueops/cmd/issueops/statecli"
	"issueops/cmd/issueops/statuscli"
	"issueops/cmd/issueops/webfetchcli"
	"issueops/cmd/issueops/workercli"
	"issueops/internal/adapter/operationalhealth"
	"issueops/internal/adapter/orca"
)

func wireBasicCLIDeps() {
	configureDocsReaders()
	configureStateStores()
	configureIssueOpsRuntime()
	configureTail8()
	configureDoctorLoopGate()
	configureHookPrompts()
	configureInstallPlans()
	configureStateDatabases()
	configureTail5()
	configureAdapterTail()
	configureTailCapabilities2()
	configureTailCapabilities()
	configureIssueOpsReaders()
	configureInstallReaders()
	configureProjectDocReaders()
	configurePolicyAndGitObservers()
	configureGatesGate()
	configureAdapterStateAccess()
	configureWorkerJobs()
	configureRepoPathResolvers()
	configureProjectBootstrap()
	configureDoctorLifecycle()
	configureDoctorRunner()
	configureIssueOpsBenchmark()
	configureIssueOpsCleanup()
	configureIssueOpsRemote()
	configureIssueOpsOrphanAndLoopGate()
	configureIssueOpsLeaseNextCommands()
	configureIssueOpsExecutionRunners()
	configureIssueOpsCLIRuntime()
	operationalCollector := operationalhealth.Collector{Git: operationalhealth.ExecGitRunner{}, Orca: orca.New()}
	basiccli.Configure(basiccli.Deps{
		GitPreflight:             preflight.GitPreflight,
		IssueOpsRoot:             issueOpsRoot,
		ResolveTarget:            resolveTarget,
		Version:                  version,
		InspectHarness:           inspectHarness,
		CheckDaemonStatus:        checkDaemonStatus,
		CollectOperationalHealth: operationalCollector.Collect,
		DocsIndex:                docs.DocsIndex,
	})
	installcli.Configure(installDependencies())
	statuscli.Configure(statuscli.Deps{
		AnalyzeProjectSignals: projectdocs.AnalyzeProjectSignals,
		IssueOpsRoot:          issueOpsRoot,
		ResolveTarget:         resolveTarget,
		Version:               version,
		InspectHarness:        inspectHarness,
		CheckDaemonStatus:     checkDaemonStatus,
		GitPreflight:          preflight.GitPreflight,
	})
	workercli.Configure(workercli.Deps{ResolveTarget: resolveTarget})
}

func runDocs(args []string) error {
	return basiccli.RunDocs(args)
}

func runPreflight(args []string) error {
	return basiccli.RunPreflight(args)
}

func runTrace(args []string) error {
	return basiccli.RunTrace(args)
}

func runGuard(args []string) error {
	return basiccli.RunGuard(args)
}

func runQuality(args []string) error {
	return qualitycli.Run(args, newQualityDependencies(issueOpsRoot(), statestore.StateDir()))
}

func runInspect(args []string) error {
	return basiccli.RunInspect(args)
}

func runDoctor(args []string) error {
	return basiccli.RunDoctor(args)
}

func runInstall(args []string) error {
	return installcli.RunInstall(args)
}

func runProject(args []string) error {
	return projectcli.Run(args)
}

func runState(args []string) error {
	return statecli.Run(stateDependencies(), args)
}

func runStatus(args []string) error {
	return statuscli.RunStatus(args)
}

func runVerifyWork(args []string) error {
	return statuscli.RunVerifyWork(args)
}

func runWorker(args []string) error {
	return workercli.Run(args)
}

func runLoop(args []string) error {
	return loopcli.Run(loopDependencies(), args)
}

func runGates(args []string) error {
	return gatescli.Run(gatesDependencies(), args)
}

func runChannel(args []string) error {
	return channelcli.Run(channelDependencies(), args)
}

func runWebFetch(args []string) error {
	return webfetchcli.Run(args)
}
