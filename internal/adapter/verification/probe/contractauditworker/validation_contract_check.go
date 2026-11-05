package contractauditworker

import (
	selfverify "issueops/internal/contract/selfverify"

	"encoding/json"
	"time"

	verifydomain "issueops/internal/domain/selfverify"
)

// ValidateContractCheck runs `contract check --json` and verifies that it passed,
// reported a hash, and lists the worker, contract, and policy CLI commands.
func ValidateContractCheck(binary, root string) selfverify.StepResult {
	return validateContractCheckWithDeps(binary, root, validationDeps{})
}

func validateContractCheckWithDeps(binary, root string, deps validationDeps) selfverify.StepResult {
	deps = deps.withDefaults()
	step := deps.RunCommandStep(root, "contract check", 30*time.Second, "", binary, "contract", "check", "--json")
	if !step.OK {
		return step
	}
	var result struct {
		OK          bool   `json:"ok"`
		Hash        string `json:"hash"`
		CLICommands []struct {
			Name string `json:"name"`
		} `json:"cli_commands"`
	}
	if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		return verifydomain.FailedStep("contract check", err)
	}
	errs := []string{}
	if !result.OK || result.Hash == "" {
		errs = append(errs, "contract did not pass or hash is empty")
	}
	for _, want := range []string{"worker", "contract", "policy"} {
		found := false
		for _, command := range result.CLICommands {
			if command.Name == want {
				found = true
			}
		}
		if !found {
			errs = append(errs, "missing CLI command "+want)
		}
	}
	return verifydomain.AssertionStep("contract check", step.DurationMS, errs)
}
