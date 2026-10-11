package policy

import (
	"os"
	"slices"

	policycontract "issueops/internal/contract/policy"
)

func containsString(items []string, want string) bool {
	return slices.Contains(items, want)
}

func existsForTest(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func evaluateCommandPolicy(req policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	return Evaluator{}.service().Evaluate(req)
}

func fakeRunCommand(req policycontract.CommandPolicyRequest) policycontract.CommandFakeRunResult {
	return Evaluator{}.service().FakeRun(req)
}

func runReadOnlyCommand(req policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return Evaluator{}.RunReadOnly(req)
}
