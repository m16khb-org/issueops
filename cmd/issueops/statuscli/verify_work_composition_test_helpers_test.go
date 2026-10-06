package statuscli

import (
	guardadapter "issueops/internal/adapter/guard"
	policyadapter "issueops/internal/adapter/policy"
	preflightadapter "issueops/internal/adapter/preflight"
	projectdocsadapter "issueops/internal/adapter/projectdocs"
	verifyworkadapter "issueops/internal/adapter/verifywork"
	guardapp "issueops/internal/application/guard"
	preflightapp "issueops/internal/application/preflight"
	verifyworkapp "issueops/internal/application/verifywork"
	preflightcontract "issueops/internal/contract/preflight"
	verifyworkcontract "issueops/internal/contract/verifywork"
)

func BuildVerifyWork(repo string, all bool, argv []string) verifyworkcontract.Result {
	preflight := preflightapp.Service{Observer: preflightadapter.GitObserver{}}
	guard := guardapp.Service{Source: guardadapter.Source{}}
	policy := policyadapter.NewEvaluator(nil)
	service := verifyworkapp.Service{
		ResolveTarget: defaultResolveTarget, GitStatus: verifyworkadapter.GitStatus,
		Preflight: func(root string) preflightcontract.PreflightResult {
			return preflight.Check(root, defaultIssueOpsRoot())
		},
		Guard: guard.Check, RunCommand: policy.RunReadOnly, ProjectSignals: projectdocsadapter.AnalyzeProjectSignals,
	}
	return service.Run(repo, all, argv)
}
