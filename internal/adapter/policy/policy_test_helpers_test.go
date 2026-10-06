package policy

import (
	"os"

	policycontract "issueops/internal/contract/policy"
)

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
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
