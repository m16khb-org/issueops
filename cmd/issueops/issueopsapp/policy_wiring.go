package issueopsapp

import (
	"issueops/cmd/issueops/policycli"
	auditadapter "issueops/internal/adapter/audit"
	issueopsadapter "issueops/internal/adapter/issueops"
	policyadapter "issueops/internal/adapter/policy"
	auditapp "issueops/internal/application/audit"
	policyapp "issueops/internal/application/policy"
)

func newPolicyService() policyapp.Service {
	reader := newActiveCycleReader(issueopsadapter.IssueOpsStateRoot())
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
	return policycli.Command{DefaultRoot: resolveTarget(""), Policy: policy, Audit: newCommandAuditService(policy)}
}

func runPolicy(args []string) error { return newPolicyCommand().Run(args) }
