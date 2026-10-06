package stateroundtrip

import (
	selfverify "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"

	"fmt"
	"time"
)

func validateStateRoundtripWithDeps(binary, root string, seed int64, deps stateRoundtripValidationDeps) selfverify.StepResult {
	deps = deps.withDefaults()
	if deps.writeSnapshot == nil {
		return verifydomain.FailedStep("state roundtrip", fmt.Errorf("self-verification snapshot writer dependency is required"))
	}
	started := time.Now()
	tempState, err := deps.mkdirTemp("", "issueops-state-roundtrip-*")
	if err != nil {
		return verifydomain.FailedStep("state roundtrip", err)
	}
	defer func() { _ = deps.removeAll(tempState) }()

	key := fmt.Sprintf("self-verify-%d", seed)
	content := fmt.Sprintf("seed=%d\nLore: state roundtrip\n", seed)
	env := []string{"ISSUEOPS_STATE_DIR=" + tempState}
	stateResult := validateStateRoundtripStateCLI(validateStateRoundtripStateInput{
		binary:    binary,
		root:      root,
		tempState: tempState,
		key:       key,
		content:   content,
		env:       env,
		started:   started,
		deps:      deps,
	})
	if !stateResult.step.OK {
		return stateResult.step
	}

	return validateStateRoundtripSelfVerifyDeps(validateStateRoundtripSelfVerifyInput{
		binary:      binary,
		root:        root,
		seed:        seed,
		tempState:   tempState,
		key:         key,
		env:         env,
		started:     started,
		stdoutParts: stateResult.stdoutParts,
		commands:    stateResult.commands,
		deps:        deps,
	})
}
