package issueopsapp

import (
	pathutil "issueops/cmd/issueops/pathutil"
	"issueops/cmd/issueops/policycli"
	auditadapter "issueops/internal/adapter/audit"

	policyadapter "issueops/internal/adapter/policy"
	auditapp "issueops/internal/application/audit"
	policyapp "issueops/internal/application/policy"
)

func newPolicyService() policyapp.Service {
	reader := newActiveCycleReader(issueOpsStateRoot())
	return policyapp.Service{
		PreparedBaseBranch: reader.PreparedBaseBranchForWorkspace,
		Observer:           policyadapter.CommandObserver{}, Overrides: policyadapter.OverrideLoader{},
		Executor: policyadapter.CommandExecutor{}, Clock: policyadapter.Clock{},
	}
}

func newCommandAuditService(policy policyapp.Service) auditapp.Service {
	return auditapp.Service{Evaluator: policy, Writer: auditadapter.NewCommandWriter(), Clock: auditadapter.Clock{}}
}

func newPolicyCommand() policycli.Command {
	policy := newPolicyService()
	return policycli.Command{DefaultRoot: pathutil.ResolveTarget(""), Policy: policy, Audit: newCommandAuditService(policy)}
}

func runPolicy(args []string) error { return newPolicyCommand().Run(args) }
