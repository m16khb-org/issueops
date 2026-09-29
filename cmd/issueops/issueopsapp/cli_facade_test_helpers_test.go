package issueopsapp

import (
	statestore "issueops/internal/adapter/outbound/state"
	qualityapp "issueops/internal/application/quality"
	qualitycontract "issueops/internal/contract/quality"
	verifyworkcontract "issueops/internal/contract/verifywork"
	"os"

	"issueops/cmd/issueops/basiccli"
	"issueops/cmd/issueops/installcli"
	"issueops/cmd/issueops/projectcli"
	"issueops/cmd/issueops/qualitycli"
	"issueops/cmd/issueops/statecli"
	statuscontract "issueops/internal/contract/status"
	"issueops/internal/port"
)

func runDocsWithRoot(args []string, root string) error {
	return basiccli.RunDocsWithRoot(args, root)
}

func runTraceAnalyze(args []string) error {
	return basiccli.RunTraceAnalyze(args)
}

func runGuardCheck(args []string) error {
	return basiccli.RunGuardCheck(args)
}

func runQualityInspectWithDeps(args []string, deps qualityapp.InspectDeps) error {
	cli := newQualityDependencies(issueOpsRoot(), statestore.StateDir())
	cli.Inspect = func(root string) qualitycontract.InspectResult { return inspectQualityForTest(root, deps) }
	return qualitycli.RunInspect(args, cli)
}

func validateInteractiveInstallInput(stdin *os.File) error {
	return installcli.ValidateInteractiveInput(stdin)
}

func printInstallNativeResult(result port.NativeInstallResult) {
	installcli.PrintNativeResult(result)
}

func preferredShellRC(home string) string {
	return installcli.PreferredShellRC(home)
}

func appendShellPathLinePlan(path string, dryRun bool) (port.InstallFile, error) {
	return installcli.AppendShellPathLinePlan(path, dryRun)
}

func shellRCAlreadyAddsLocalBin(path, home string) bool {
	return installcli.ShellRCAlreadyAddsLocalBin(path, home)
}

func runProjectBootstrap(args []string) error {
	return projectcli.RunBootstrap(newProjectBootstrapService("."), args)
}

func runProjectDocs(args []string) error {
	return projectcli.RunDocs(newProjectDocsService("."), args)
}

func runProjectRouteDocs(args []string) error {
	return projectcli.RunRouteDocs(newProjectDocsService("."), args)
}

func runProjectAppend(args []string) error {
	return projectcli.RunRecord(newProjectDocsService("."), args)
}

func runProjectCommitSuggest(args []string) error {
	return projectcli.RunCommitSuggest(args)
}

func runProjectLintDiagnose(args []string) error {
	return projectcli.RunLintDiagnose(args)
}

func runStateWrite(args []string) error {
	return statecli.RunWrite(stateDependencies(), args)
}

func runStateRead(args []string) error {
	return statecli.RunRead(stateDependencies(), args)
}

func runStateList(args []string) error {
	return statecli.RunList(stateDependencies(), args)
}

func runStatePrune(args []string) error {
	return statecli.RunPrune(stateDependencies(), args)
}

func runStateDoctor(args []string) error {
	return statecli.RunDoctor(stateDependencies(), args)
}

func buildHarnessStatus(repo string) HarnessStatus {
	return newStatusService().Run(repo)
}

func buildVerifyWork(repo string, all bool, argv []string) VerifyWorkResult {
	return newVerifyWorkService().Run(repo, all, argv)
}

func runWorkerEnqueue(args []string) error {
	return newWorkerCommand().RunEnqueue(args)
}

func runWorkerRun(args []string) error {
	return newWorkerCommand().RunReadOnly(args)
}

func runWorkerStatus(args []string) error {
	return newWorkerCommand().RunStatus(args)
}

func runWorkerList(args []string) error {
	return newWorkerCommand().RunList(args)
}

func runWorkerCancel(args []string) error {
	return newWorkerCommand().RunCancel(args)
}

type HarnessStatus = statuscontract.Result

type VerifyWorkResult = verifyworkcontract.Result
