package audit

import (
	policyadapter "issueops/internal/adapter/policy"
	auditapp "issueops/internal/application/audit"
	auditcontract "issueops/internal/contract/audit"
	policycontract "issueops/internal/contract/policy"
)

func AuditCommandPolicy(req policycontract.CommandPolicyRequest) (auditcontract.CommandAuditRecord, error) {
	return (auditapp.Service{Evaluator: policyadapter.NewEvaluator(nil), Writer: NewCommandWriter(), Clock: Clock{}}).Audit(req)
}
