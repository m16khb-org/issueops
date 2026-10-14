package gates

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/gates"
	policy "issueops/internal/contract/policy"
	domain "issueops/internal/domain/gates"
)

func TestLintJudgesPolicyWithTheRequestGatesCheckWouldSend(t *testing.T) {
	var linted policy.CommandPolicyRequest
	service := Service{
		Store: &mutationStore{data: []byte("- G1: x | CHECK: grep -rn token src | EXPECT: ok\n- G2: y | CHECK: printf ok | EXPECT: ok\n")},
		Evaluate: func(req policy.CommandPolicyRequest) policy.CommandPolicyEvaluation {
			if req.Argv[0] == "grep" {
				linted = req
				return policy.CommandPolicyEvaluation{Allowed: false, DenyReasons: []string{"secret_like_argument"}}
			}
			return policy.CommandPolicyEvaluation{Allowed: true}
		},
	}
	req := model.LintRequest{CWD: "/repo", PlanFile: "plan.md", EnvAllowlist: []string{"HOME", "PATH"}, WriteAllowed: true, NetworkAllowed: true}
	result, err := service.Lint(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Clean || result.GatesWithProblems != 1 || !result.Gates[0].PolicyDenied ||
		!strings.Contains(result.Gates[0].Problems[0], "secret_like_argument") || len(result.Gates[1].Problems) != 0 {
		t.Fatalf("result = %+v, want only G1 denied by policy", result)
	}
	check := domain.NormalizeCheck(model.CheckRequest{CWD: "/repo", EnvAllowlist: req.EnvAllowlist, WriteAllowed: true, NetworkAllowed: true})
	if want := checkPolicyRequest(check.WorkspaceRoot, check.CWD, check, []string{"grep", "-rn", "token", "src"}); !reflect.DeepEqual(linted, want) {
		t.Fatalf("lint policy request = %+v, want the gates check request %+v", linted, want)
	}
}

func TestLintWithoutAnySourceIsAUsageError(t *testing.T) {
	_, err := (Service{Store: &mutationStore{}}).Lint(model.LintRequest{CWD: "/repo"})
	if !errors.Is(err, model.ErrNothingToLint) {
		t.Fatalf("err = %v, want ErrNothingToLint", err)
	}
}

func TestLintPlanWithUnreadableGateLinesIsNotClean(t *testing.T) {
	plan := "- G1: x | CHECK: printf ok | EXPECT: ok\n- G2: `go test ./...` → EXPECT ok\n"
	result, err := (Service{Store: &mutationStore{data: []byte(plan)}}).Lint(model.LintRequest{CWD: "/repo", PlanFile: "plan.md"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Clean || result.GatesWithProblems != 0 || !reflect.DeepEqual(result.UnparsedLines, []string{"plan.md:2"}) {
		t.Fatalf("result = %+v, want not clean because plan.md:2 could not be read as a gate", result)
	}
}
