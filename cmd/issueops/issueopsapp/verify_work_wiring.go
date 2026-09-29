package issueopsapp

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
)

func newVerifyWorkService() verifyworkapp.Service {
	harnessRoot := issueOpsRoot()
	reader := newActiveCycleReader(issueOpsStateRoot())
	policy := policyadapter.NewEvaluator(reader.PreparedBaseBranchForWorkspace)
	preflight := preflightapp.Service{Observer: preflightadapter.GitObserver{}}
	guard := guardapp.Service{Source: guardadapter.Source{}}
	return verifyworkapp.Service{
		ResolveTarget:  resolveTarget,
		GitStatus:      verifyworkadapter.GitStatus,
		Preflight:      func(root string) preflightcontract.PreflightResult { return preflight.Check(root, harnessRoot) },
		Guard:          guard.Check,
		RunCommand:     policy.RunReadOnly,
		ProjectSignals: projectdocsadapter.AnalyzeProjectSignals,
	}
}
