package selfverify

import (
	"reflect"
	"testing"
)

func TestLLMEvalModeNormalizationAndValidation(t *testing.T) {
	if NormalizeLLMEvalMode("  GATE ") != "gate" || NormalizeLLMEvalMode("") != "advisory" {
		t.Fatal("normalize")
	}
	if ValidateLLMEvalMode(" Gate ") != nil || ValidateLLMEvalMode("") != nil {
		t.Fatal("valid modes rejected")
	}
	if err := ValidateLLMEvalMode("strict"); err == nil || err.Error() != "llm-eval-mode must be advisory or gate" {
		t.Fatalf("err=%v", err)
	}
}

func TestParseLLMEvalEnv(t *testing.T) {
	cases := []struct {
		in      string
		enabled bool
		mode    string
		bad     bool
	}{
		{"", false, "advisory", false}, {" OFF ", false, "advisory", false},
		{"yes", true, "advisory", false}, {"advisory", true, "advisory", false},
		{"Gate", true, "gate", false}, {"maybe", false, "advisory", true},
	}
	for _, tc := range cases {
		enabled, mode, err := ParseLLMEvalEnv(tc.in)
		if enabled != tc.enabled || mode != tc.mode || (err != nil) != tc.bad {
			t.Fatalf("%q: %v %q %v", tc.in, enabled, mode, err)
		}
	}
}

func TestLLMEvalGateReasonsOnlyInGateMode(t *testing.T) {
	if got := LLMEvalGateReasons("advisory", false, 0, 90, []string{"x"}); got != nil {
		t.Fatalf("advisory=%v", got)
	}
	if got := LLMEvalGateReasons("gate", true, 95, 90, nil); len(got) != 0 {
		t.Fatalf("passing gate=%v", got)
	}
	got := LLMEvalGateReasons("gate", false, 80, 90, []string{"a", "b"})
	want := []string{"llm_eval_not_ok", "score 80.00 below target 90.00", "blockers: a; b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v", got)
	}
}
