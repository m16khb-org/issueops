package policy

import (
	"reflect"
	"slices"
	"testing"

	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

type observeFunc func(policycontract.CommandPolicyRequest) Observation

func (fn observeFunc) Observe(request policycontract.CommandPolicyRequest) Observation {
	return fn(request)
}

type overrideFunc func(string) OverrideSnapshot

func (fn overrideFunc) Load(root string) OverrideSnapshot { return fn(root) }

func TestServiceLoadsAndClassifiesWorkspaceOverridePerEvaluation(t *testing.T) {
	loads := 0
	service := Service{
		Observer: observeFunc(func(request policycontract.CommandPolicyRequest) Observation {
			return Observation{
				Root: request.WorkspaceRoot, CWD: request.CWD,
				Facts: policydomain.CommandFacts{
					RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true,
				},
			}
		}),
		Overrides: overrideFunc(func(root string) OverrideSnapshot {
			if root != "/repo" {
				t.Fatalf("override root = %q", root)
			}
			loads++
			if loads == 1 {
				return OverrideSnapshot{Values: &policycontract.PolicyOverrides{AdditionalReadOnlyCommands: []string{"repo-tool"}}}
			}
			return OverrideSnapshot{Warning: "policy_override_parse_failed"}
		}),
	}
	request := policycontract.CommandPolicyRequest{
		WorkspaceRoot: "/repo", CWD: "/repo", Argv: []string{"repo-tool"}, Timeout: "30s",
	}
	first := service.Evaluate(request)
	if !first.Allowed {
		t.Fatalf("first evaluation did not apply override: %+v", first)
	}
	second := service.Evaluate(request)
	if second.Allowed || !slices.Contains(second.DenyReasons, "command_not_in_read_only_allowlist") ||
		!slices.Contains(second.Warnings, "policy_override_parse_failed") || loads != 2 {
		t.Fatalf("second evaluation reused override: %+v, loads=%d", second, loads)
	}
}

func TestServiceEvaluatesOneObservedSnapshot(t *testing.T) {
	calls := 0
	service := Service{Observer: observeFunc(func(request policycontract.CommandPolicyRequest) Observation {
		calls++
		if request.WorkspaceRoot != "/repo" {
			t.Fatalf("request changed before observation: %+v", request)
		}
		return Observation{
			Root: "/repo", CWD: "/repo",
			AuditLogID: "audit-1", GeneratedAt: "2026-09-25T00:00:00Z",
			Facts: policydomain.CommandFacts{
				RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true,
				ReadOnlyAllowed: true,
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
