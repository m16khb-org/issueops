package mcpcli

import (
	auditadapter "issueops/internal/adapter/audit"
	policyadapter "issueops/internal/adapter/policy"
	auditapp "issueops/internal/application/audit"
	policyapp "issueops/internal/application/policy"
)

func testPolicyService() policyapp.Service {
	return policyapp.Service{Observer: policyadapter.CommandObserver{}, Overrides: policyadapter.OverrideLoader{}, Executor: policyadapter.CommandExecutor{}, Clock: policyadapter.Clock{}}
}
func testAuditService() auditapp.Service {
	return auditapp.Service{Evaluator: testPolicyService(), Writer: auditadapter.NewCommandWriter(), Clock: auditadapter.Clock{}}
}
