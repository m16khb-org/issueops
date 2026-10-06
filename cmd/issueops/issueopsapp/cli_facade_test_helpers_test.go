package issueopsapp

import (
	statestore "issueops/internal/adapter/outbound/state"
	qualityapp "issueops/internal/application/quality"
	qualitycontract "issueops/internal/contract/quality"
	verifyworkcontract "issueops/internal/contract/verifywork"

	"issueops/cmd/issueops/qualitycli"
	statuscontract "issueops/internal/contract/status"
)

func runDocsWithRoot(args []string, root string) error {
	command := newBasicCommand()
	command.IssueOpsRoot = root
	return command.RunDocs(args)
}

func runTraceAnalyze(args []string) error {
	return newBasicCommand().RunTrace(append([]string{"analyze"}, args...))
}

func runGuardCheck(args []string) error {
	return newBasicCommand().RunGuard(append([]string{"check"}, args...))
}

func runQualityInspectWithDeps(args []string, deps qualityapp.InspectDeps) error {
	cli := newQualityDependencies(issueOpsRoot(), statestore.StateDir())
	cli.Inspect = func(root string) qualitycontract.InspectResult { return inspectQualityForTest(root, deps) }
	return qualitycli.RunInspect(args, cli)
}

func buildHarnessStatus(repo string) statuscontract.Result {
	return newStatusService().Run(repo)
}

func buildVerifyWork(repo string, all bool, argv []string) verifyworkcontract.Result {
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
