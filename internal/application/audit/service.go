package audit

import (
	"time"

	auditcontract "issueops/internal/contract/audit"
	policycontract "issueops/internal/contract/policy"
)

type PolicyEvaluator interface {
	Evaluate(policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation
}

type Writer interface {
	Path() (string, error)
	Append(string, auditcontract.CommandAuditRecord) error
}

type Clock interface{ Now() time.Time }

type Service struct {
	Evaluator PolicyEvaluator
	Writer    Writer
	Clock     Clock
}

func (service Service) Audit(request policycontract.CommandPolicyRequest) (auditcontract.CommandAuditRecord, error) {
	evaluation := service.Evaluator.Evaluate(request)
	record := auditcontract.CommandAuditRecord{
		OK: evaluation.Allowed, Kind: "command_policy_audit",
		AuditLogID:  evaluation.AuditLogID,
		GeneratedAt: service.Clock.Now().UTC().Format(time.RFC3339Nano),
		Policy:      evaluation,
	}
	path, err := service.Writer.Path()
	if err != nil {
		return record, err
	}
	record.LogPath = path
	if err := service.Writer.Append(path, record); err != nil {
		return record, err
	}
	return record, nil
}
