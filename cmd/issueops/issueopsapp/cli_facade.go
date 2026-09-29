package issueopsapp

import (
	"issueops/cmd/issueops/channelcli"
	"issueops/cmd/issueops/gatescli"
	"issueops/cmd/issueops/installcli"
	"issueops/cmd/issueops/loopcli"
	"issueops/cmd/issueops/projectcli"
	"issueops/cmd/issueops/qualitycli"
	"issueops/cmd/issueops/statecli"
	"issueops/cmd/issueops/statuscli"
	"issueops/cmd/issueops/webfetchcli"
	statestore "issueops/internal/adapter/outbound/state"
)

func wireBasicCLIDeps() {
	configureDocsReaders()
	configureStateStores()
	configureIssueOpsRuntime()
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
	configureAdapterStateAccess()
	configureRepoPathResolvers()
	configureIssueOpsBenchmark()
	configureIssueOpsCleanup()
	configureIssueOpsRemote()
	configureIssueOpsOrphanAndLoopGate()
	configureIssueOpsLeaseNextCommands()
	configureIssueOpsExecutionRunners()
	configureIssueOpsCLIRuntime()
	installcli.Configure(installDependencies())

}

func runDocs(args []string) error {
	return newBasicCommand().RunDocs(args)
}

func runPreflight(args []string) error {
	return newBasicCommand().RunPreflight(args)
}

func runTrace(args []string) error {
	return newBasicCommand().RunTrace(args)
}

func runGuard(args []string) error {
	return newBasicCommand().RunGuard(args)
}

func runQuality(args []string) error {
	return qualitycli.Run(args, newQualityDependencies(issueOpsRoot(), statestore.StateDir()))
}

func runInspect(args []string) error {
	return newBasicCommand().RunInspect(args)
}

func runDoctor(args []string) error {
	return newDoctorCommand().Run(args)
}

func runInstall(args []string) error {
	return installcli.RunInstall(args)
}

func runProject(args []string) error {
	return projectcli.Run(projectcli.Dependencies{Docs: newProjectDocsService("."), Bootstrap: newProjectBootstrapService(".")}, args)
}

func runState(args []string) error {
	return statecli.Run(stateDependencies(), args)
}

func runStatus(args []string) error {
	return (statuscli.Command{Service: newStatusService()}).Run(args)
}

func runVerifyWork(args []string) error {
	return statuscli.RunVerifyWork(newVerifyWorkService(), args)
}

func runWorker(args []string) error {
	return newWorkerCommand().Run(args)
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
