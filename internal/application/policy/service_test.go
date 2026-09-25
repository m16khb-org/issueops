package policy

import (
	"reflect"
	"testing"
	"time"

	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

type observeFunc func(policycontract.CommandPolicyRequest) Observation

func (fn observeFunc) Observe(request policycontract.CommandPolicyRequest) Observation {
	return fn(request)
}

func TestServiceEvaluatesOneObservedSnapshot(t *testing.T) {
	calls := 0
	service := Service{Observer: observeFunc(func(request policycontract.CommandPolicyRequest) Observation {
		calls++
		if request.WorkspaceRoot != "/repo" {
			t.Fatalf("request changed before observation: %+v", request)
		}
		return Observation{
			Root: "/repo", CWD: "/repo", Timeout: 30 * time.Second,
			AuditLogID: "audit-1", GeneratedAt: "2026-09-25T00:00:00Z",
			Facts: policydomain.CommandFacts{
				RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true,
				Timeout: 30 * time.Second, TimeoutValid: true, ReadOnlyAllowed: true,
			},
		}
	})}
	result := service.Evaluate(policycontract.CommandPolicyRequest{
		WorkspaceRoot: "/repo", CWD: "/repo", Argv: []string{"git", "status"},
		Timeout: "30s", EnvAllowlist: []string{"PATH", "PATH"},
	})
	if calls != 1 || !result.OK || !result.Allowed || result.AuditLogID != "audit-1" ||
		!reflect.DeepEqual(result.DenyReasons, []string{}) || !reflect.DeepEqual(result.Warnings, []string{}) ||
		!reflect.DeepEqual(result.EnvAllowlist, []string{"PATH", "PATH"}) {
		t.Fatalf("policy service result=%+v calls=%d", result, calls)
	}
}
