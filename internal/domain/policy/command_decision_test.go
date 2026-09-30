package policy

import (
	"reflect"
	"testing"
	"time"

	policycontract "issueops/internal/contract/policy"
)

func TestCommandDecisionCombinesObservedFactsWithoutMutatingWarnings(t *testing.T) {
	warnings := []string{"policy_override_parse_failed"}
	decision := EvaluateCommandDecision(policycontract.CommandPolicyRequest{
		WorkspaceRoot: "/repo", CWD: "/repo", Argv: []string{"sh", "token=secret"},
		Timeout: "20m", EnvAllowlist: []string{"BAD-NAME"},
	}, CommandFacts{
		RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true,
		Timeout: 20 * time.Minute, TimeoutValid: true, ShellCommand: true,
		UsesNetwork: true, Writes: true, ReadOnlyAllowed: false,
		PRTargetDeny: "pr_target_mismatch", PRTargetExpected: "main",
		Warnings: warnings,
	})
	want := []string{"command_not_in_read_only_allowlist", "invalid_env_allowlist_name", "network_not_allowed", "pr_target_mismatch", "secret_like_argument", "shell_interpreter_not_allowed", "timeout_exceeds_15m", "write_not_allowed"}
	if decision.Allowed || !reflect.DeepEqual(decision.DenyReasons, want) ||
		!reflect.DeepEqual(decision.Warnings, []string{"policy_override_parse_failed", "pr_target_branch_expected=main"}) {
		t.Fatalf("decision=%+v", decision)
	}
	if !reflect.DeepEqual(warnings, []string{"policy_override_parse_failed"}) {
		t.Fatalf("warnings mutated: %v", warnings)
	}
}

func TestCommandDecisionRequiresObservedRootAndCWD(t *testing.T) {
	decision := EvaluateCommandDecision(policycontract.CommandPolicyRequest{}, CommandFacts{})
	want := []string{"argv_required", "cwd_required", "invalid_timeout", "workspace_root_required"}
	if decision.Allowed || !reflect.DeepEqual(decision.DenyReasons, want) || len(decision.Warnings) != 0 {
		t.Fatalf("missing input decision=%+v", decision)
	}
}
