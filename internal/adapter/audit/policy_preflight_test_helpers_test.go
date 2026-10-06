package audit

import (
	policyadapter "issueops/internal/adapter/policy"
	auditapp "issueops/internal/application/audit"
	policyapp "issueops/internal/application/policy"
	auditcontract "issueops/internal/contract/audit"
	policycontract "issueops/internal/contract/policy"
)

func AuditCommandPolicy(req policycontract.CommandPolicyRequest) (auditcontract.CommandAuditRecord, error) {
	return (auditapp.Service{Evaluator: policyapp.Service{Observer: policyadapter.CommandObserver{}, Overrides: policyadapter.OverrideLoader{}, Executor: policyadapter.CommandExecutor{}, Clock: policyadapter.Clock{}}, Writer: NewCommandWriter(), Clock: Clock{}}).Audit(req)
}
