// Package audit is the application service that records command policy
// evaluations in the harness audit log without executing the command.
package audit

import (
	"time"

	auditcontract "issueops/internal/contract/audit"
	policycontract "issueops/internal/contract/policy"
)

type policyEvaluator interface {
	Evaluate(policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation
}

type writer interface {
	Path() (string, error)
	Append(string, auditcontract.CommandAuditRecord) error
}

type clock interface{ Now() time.Time }

// Service evaluates command policy requests and appends each evaluation to the
// audit log. Evaluator, Writer and Clock are all required.
type Service struct {
	Evaluator policyEvaluator
	Writer    writer
	Clock     clock
}

// Audit evaluates request under the command policy and appends the redacted
// result to the audit log, returning the record with the log path filled in.
// The record is returned even when the policy denies the command; an error is
// returned only when the log path or the append fails.
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
