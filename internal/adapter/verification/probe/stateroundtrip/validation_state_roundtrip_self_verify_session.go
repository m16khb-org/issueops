package stateroundtrip

import (
	"encoding/json"

	selfverify "issueops/internal/contract/selfverify"
)

import "time"

type validateStateRoundtripSelfVerifyInput struct {
	binary      string
	root        string
	seed        int64
	tempState   string
	key         string
	env         []string
	started     time.Time
	stdoutParts []string
	commands    []string
	deps        stateRoundtripValidationDeps
}

type stateRoundtripSelfVerifySession struct {
	input       validateStateRoundtripSelfVerifyInput
	stdoutParts []string
	commands    []string
}

func newStateRoundtripSelfVerifySession(input validateStateRoundtripSelfVerifyInput) *stateRoundtripSelfVerifySession {
	return &stateRoundtripSelfVerifySession{
		input:       input,
		stdoutParts: input.stdoutParts,
		commands:    input.commands,
	}
}

func (s *stateRoundtripSelfVerifySession) run(label string, command ...string) selfverify.StepResult {
	step := s.input.deps.run(s.input.root, label, 30*time.Second, "", s.input.env, command...)
	s.stdoutParts = append(s.stdoutParts, step.Stdout)
	s.commands = append(s.commands, step.Command)
	return step
}

func (s *stateRoundtripSelfVerifySession) fail(errs ...string) selfverify.StepResult {
	return assertionStepWithOutput("state roundtrip", s.input.started, errs, s.stdoutParts, s.commands)
}

func (s *stateRoundtripSelfVerifySession) combineFailed(step selfverify.StepResult) selfverify.StepResult {
	return combineFailedStep("state roundtrip", s.input.started, step, s.stdoutParts, s.commands)
}

// runSelfVerifyJSON은 CLI 단계를 실행하고 stdout을 T로 해석한다.
// 실행 실패나 해석 실패 시 호출자가 그대로 반환할 실패 단계를 함께 돌려준다.
func runSelfVerifyJSON[T any](s *stateRoundtripSelfVerifySession, label string, args ...string) (T, *selfverify.StepResult) {
	var result T
	step := s.run(label, append([]string{s.input.binary}, args...)...)
	if !step.OK {
		failed := s.combineFailed(step)
		return result, &failed
	}
	if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		failed := s.fail(err.Error())
		return result, &failed
	}
	return result, nil
}
