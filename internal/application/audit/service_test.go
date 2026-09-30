package audit

import (
	"errors"
	"testing"
	"time"

	auditcontract "issueops/internal/contract/audit"
	policycontract "issueops/internal/contract/policy"
)

type evaluatorFunc func(policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation

func (fn evaluatorFunc) Evaluate(request policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	return fn(request)
}

type writerFunc struct {
	path   func() (string, error)
	append func(string, auditcontract.CommandAuditRecord) error
}

func (writer writerFunc) Path() (string, error) { return writer.path() }
func (writer writerFunc) Append(path string, record auditcontract.CommandAuditRecord) error {
	return writer.append(path, record)
}

type fixedClock struct{ at time.Time }

func (clock fixedClock) Now() time.Time { return clock.at }

func TestAuditEvaluatesOnceBeforeAppendAndPreservesPathOnFailure(t *testing.T) {
	calls := 0
	appended := 0
	service := Service{
		Evaluator: evaluatorFunc(func(policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
			calls++
			return policycontract.CommandPolicyEvaluation{Allowed: false, AuditLogID: "audit-1"}
		}),
		Writer: writerFunc{
			path: func() (string, error) { return "/state/audit.jsonl", nil },
			append: func(path string, record auditcontract.CommandAuditRecord) error {
				appended++
				if calls != 1 || path != record.LogPath || record.AuditLogID != "audit-1" {
					t.Fatalf("append order/path changed: calls=%d path=%q record=%+v", calls, path, record)
				}
				return errors.New("write failed")
			},
		},
		Clock: fixedClock{at: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)},
	}
	record, err := service.Audit(policycontract.CommandPolicyRequest{})
	if err == nil || err.Error() != "write failed" || calls != 1 || appended != 1 ||
		record.LogPath != "/state/audit.jsonl" || record.GeneratedAt != "2026-09-25T00:00:00Z" {
		t.Fatalf("audit record=%+v err=%v calls=%d appended=%d", record, err, calls, appended)
	}
}
