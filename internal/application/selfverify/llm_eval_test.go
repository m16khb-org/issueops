package selfverify

import (
	"errors"
	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	"strings"
	"testing"
)

func TestApplyLLMEvalAdmissionAndFailedPacketRespectGateMode(t *testing.T) {
	for _, tc := range []struct {
		name, mode            string
		enabled, called, pass bool
	}{
		{"disabled", "bad", false, false, true}, {"invalid", "bad", true, false, true},
		{"advisory", "advisory", true, true, true}, {"gate", "gate", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			input := augment.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: augment.SelfAugmentSummary{TerminationEligible: true}}
			got, err := ApplyLLMEval(input, verify.LLMEvalOptions{Enabled: tc.enabled, Mode: tc.mode, TargetScore: 95}, LLMEvalDeps{
				BuildPrompt: func(augment.SelfAugmentResult) (string, int, error) {
					calls++
					return "", 17, errors.New("packet rejected")
				},
				BoundError: func(prefix string, err error, output string) string { return prefix + ": " + err.Error() },
			})
			if (calls > 0) != tc.called || calls > 1 || got.OK != tc.pass {
				t.Fatalf("calls=%d got=%+v err=%v", calls, got, err)
			}
			if tc.called {
				if got.LLMEval == nil || got.LLMEval.OK || got.LLMEval.EvidencePacketBytes != 17 || !got.LLMEval.ReadOnly || got.LLMEval.ExecutionClass != "foreground_blocking" || !strings.Contains(got.LLMEval.Error, "packet rejected") {
					t.Fatal(got.LLMEval)
				}
				if tc.pass != (err == nil) || got.TerminationEligible != tc.pass || got.Summary.TerminationEligible != tc.pass {
					t.Fatalf("gate propagation lost: %+v err=%v", got, err)
				}
			} else if tc.enabled != (err != nil) || got.LLMEval != nil {
				t.Fatalf("admission did not stop before evaluation: %+v err=%v", got, err)
			}
		})
	}
}
