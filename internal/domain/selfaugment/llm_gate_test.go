package selfaugment

import (
	contract "issueops/internal/contract/selfaugment"
	"testing"
)

func TestLLMGatePreservesThresholdAndOnlyRevokesTermination(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		ok         bool
		score      float64
		blockers   []string
		pass       bool
	}{
		{"advisory", "advisory", false, 0, []string{"problem"}, true},
		{"equal", "gate", true, 95, nil, true},
		{"below", "gate", true, 94.99, nil, false},
		{"not-ok", "gate", false, 100, nil, false},
		{"blocker", "gate", true, 100, []string{"problem"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := contract.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: contract.SelfAugmentSummary{TerminationEligible: true}, LLMEval: &contract.SelfVerifyLLMEvalResult{Mode: tc.mode, OK: tc.ok, Score: tc.score, Blockers: tc.blockers}}
			got, err := ApplySelfVerifyLLMGate(input, 95)
			if got.OK != tc.pass || got.TerminationEligible != tc.pass || got.Summary.TerminationEligible != tc.pass || (err == nil) != tc.pass {
				t.Fatalf("gate result=%+v err=%v", got, err)
			}
		})
	}
	priorFailure := contract.SelfAugmentResult{LLMEval: &contract.SelfVerifyLLMEvalResult{Mode: "gate", OK: true, Score: 100}}
	got, err := ApplySelfVerifyLLMGate(priorFailure, 95)
	if err != nil || got.OK || got.TerminationEligible || got.Summary.TerminationEligible {
		t.Fatal("LLM success promoted deterministic failure")
	}
}
