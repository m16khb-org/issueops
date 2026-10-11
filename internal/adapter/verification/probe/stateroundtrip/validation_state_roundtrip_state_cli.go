package stateroundtrip

import (
	selfverify "issueops/internal/contract/selfverify"
	"slices"

	"time"

	statecontract "issueops/internal/contract/state"
)

type validateStateRoundtripStateInput struct {
	binary    string
	root      string
	tempState string
	key       string
	content   string
	env       []string
	started   time.Time
	deps      stateRoundtripValidationDeps
}

type validateStateRoundtripStateResult struct {
	step        selfverify.StepResult
	stdoutParts []string
	commands    []string
}

type stateRoundtripStateSession struct {
	input       validateStateRoundtripStateInput
	stdoutParts []string
	commands    []string
}

func validateStateRoundtripStateCLI(input validateStateRoundtripStateInput) validateStateRoundtripStateResult {
	session := stateRoundtripStateSession{input: input}
	if step := session.validateWriteReadList(); !step.OK {
		return session.result(step)
	}
	if step := session.validatePrune(); !step.OK {
		return session.result(step)
	}
	return session.result(selfverify.StepResult{OK: true})
}

func (s *stateRoundtripStateSession) run(label string, command ...string) selfverify.StepResult {
	step := s.input.deps.run(s.input.root, label, 30*time.Second, "", s.input.env, command...)
	s.stdoutParts = append(s.stdoutParts, step.Stdout)
	s.commands = append(s.commands, step.Command)
	return step
}

func (s *stateRoundtripStateSession) fail(errs ...string) selfverify.StepResult {
	return assertionStepWithOutput("state roundtrip", s.input.started, errs, s.stdoutParts, s.commands)
}

func (s *stateRoundtripStateSession) combineFailed(step selfverify.StepResult) selfverify.StepResult {
	return combineFailedStep("state roundtrip", s.input.started, step, s.stdoutParts, s.commands)
}

func (s *stateRoundtripStateSession) result(step selfverify.StepResult) validateStateRoundtripStateResult {
	return validateStateRoundtripStateResult{step: step, stdoutParts: s.stdoutParts, commands: s.commands}
}

// recordMatches reports whether a state result carries the roundtrip record unchanged.
func (s *stateRoundtripStateSession) recordMatches(result statecontract.StateResult) bool {
	return result.OK && result.Record.Key == s.input.key && result.Record.Content == s.input.content && result.Record.Bytes == len([]byte(s.input.content))
}

func (s *stateRoundtripStateSession) validateWriteReadList() selfverify.StepResult {
	writeResult, failed, ok := runStateJSON[statecontract.StateResult](s, "state write", s.input.binary, "state", "write", "--key", s.input.key, "--value", s.input.content, "--json")
	if !ok {
		return failed
	}
	if !s.recordMatches(writeResult) {
		return s.fail("write result did not match expected record")
	}

	readResult, failed, ok := runStateJSON[statecontract.StateResult](s, "state read", s.input.binary, "state", "read", "--key", s.input.key, "--json")
	if !ok {
		return failed
	}
	if !s.recordMatches(readResult) {
		return s.fail("read result did not match expected record")
	}

	listResult, failed, ok := runStateJSON[statecontract.StateListResult](s, "state list", s.input.binary, "state", "list", "--json")
	if !ok {
		return failed
	}
	if !listResult.OK || !slices.Contains(listResult.Keys, s.input.key) {
		return s.fail("state list did not include roundtrip key")
	}
	return selfverify.StepResult{OK: true}
}
