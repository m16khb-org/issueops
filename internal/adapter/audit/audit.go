package audit

import (
	"encoding/json"
	"fmt"
	auditcontract "issueops/internal/contract/audit"
	"os"
	"path/filepath"
	"time"

	auditapp "issueops/internal/application/audit"
	policycontract "issueops/internal/contract/policy"
)

// AuditCommandPolicy는 명령 요청을 평가해 redacted policy 결정을 JSONL audit
// log에 append한다. 명령 자체를 실행하지는 않는다.
func AuditCommandPolicy(req policycontract.CommandPolicyRequest) (auditcontract.CommandAuditRecord, error) {
	return (auditapp.Service{Evaluator: policyEvaluator{EvaluateCommandPolicy}, Writer: commandAuditWriter{}, Clock: auditClock{}}).Audit(req)
}

type policyEvaluator struct {
	evaluate func(policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation
}

func (evaluator policyEvaluator) Evaluate(req policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	return evaluator.evaluate(req)
}

type auditClock struct{}

func (auditClock) Now() time.Time { return time.Now() }

type commandAuditWriter struct{}

func (commandAuditWriter) Path() (string, error) { return commandAuditLogPath() }

func (commandAuditWriter) Append(path string, record auditcontract.CommandAuditRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func commandAuditLogPath() (string, error) {
	if path := os.Getenv("ISSUEOPS_AUDIT_LOG"); path != "" {
		return filepath.Abs(path)
	}
	dir := os.Getenv("ISSUEOPS_STATE_DIR")
	if dir == "" {
		base, err := os.UserHomeDir()
		if err != nil || base == "" {
			return "", fmt.Errorf("resolve home for audit log: %w", err)
		}
		dir = filepath.Join(base, ".local", "state", "issueops")
	}
	return filepath.Join(dir, "audit", "command-policy.jsonl"), nil
}
