package augmentplan

import (
	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	"strings"
	"testing"
)

func TestPlanVerificationScoreAndEvidenceShareOneRead(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	previous := StateRead
	t.Cleanup(func() { StateRead = previous })
	reads := 0
	StateRead = func(key string) (state.StateResult, error) {
		reads++
		content := `{"schema_version":1,"kind":"self_verification_summary","ok":true}`
		if reads > 1 {
			content = `{"schema_version":1,"kind":"self_verification_summary","ok":false}`
		}
		return state.StateResult{Record: state.RecordEnvelope{Content: content}}, nil
	}
	result := Plan(contract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}, t.TempDir(), "test")
	if reads != 1 {
		t.Errorf("summary reads=%d want=1", reads)
	}
	for _, goal := range result.Goals {
		if goal.Name == "verification_qa" {
			if goal.Score != 100 || !goal.Passed || len(goal.Evidence) != 1 || !strings.Contains(goal.Evidence[0], "ok=true") {
				t.Fatalf("inconsistent verification goal: %+v", goal)
			}
			return
		}
	}
	t.Fatal("verification goal missing")
}

func TestVerificationGoalRequiresCurrentSummaryKindAndSchema(t *testing.T) {
	previous := StateRead
	t.Cleanup(func() { StateRead = previous })
	for _, tc := range []struct {
		name, content string
		passed        bool
	}{
		{"missing-schema", `{"ok":true}`, false},
		{"zero-schema", `{"schema_version":0,"kind":"self_verification_summary","ok":true}`, false},
		{"future-schema", `{"schema_version":999,"kind":"self_verification_summary","ok":true}`, false},
		{"wrong-kind", `{"schema_version":1,"kind":"other","ok":true}`, false},
		{"current-pass", `{"schema_version":1,"kind":"self_verification_summary","ok":true}`, true},
		{"current-fail", `{"schema_version":1,"kind":"self_verification_summary","ok":false}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			StateRead = func(key string) (state.StateResult, error) {
				if key != "self-verify-latest" {
					t.Fatalf("unexpected key %q", key)
				}
				return state.StateResult{Record: state.RecordEnvelope{Content: tc.content}}, nil
			}
			if got := selfVerificationPassed(); got != tc.passed {
				t.Fatalf("verification evidence admission=%v want=%v", got, tc.passed)
			}
		})
	}
}
