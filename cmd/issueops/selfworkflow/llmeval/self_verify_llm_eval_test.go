package llmeval

import (
	"errors"
	selfverifyx "issueops/internal/contract/selfverify"
	selfverify "issueops/internal/domain/selfverify"

	"encoding/json"
	"strings"
	"testing"

	augmentcontract "issueops/internal/contract/selfaugment"
)

func TestSelfVerifyLLMEvalDefaultOmittedFromJSON(t *testing.T) {
	result := augmentcontract.SelfAugmentResult{OK: true, LoopKind: "self_verification"}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "llm_eval") {
		t.Fatalf("default self-verify JSON must omit llm_eval: %s", b)
	}
}

func TestResolveSelfVerifyLLMEvalConfigDefaultsOff(t *testing.T) {
	config, err := ResolveSelfVerifyLLMEvalConfig(false, false, "advisory", false, envLookupForSelfVerifyTest(nil))
	if err != nil {
		t.Fatal(err)
	}
	if config.Enabled || config.Mode != "advisory" {
		t.Fatalf("default LLM eval config should stay off/advisory, got %+v", config)
	}
}

func TestResolveSelfVerifyLLMEvalConfigUsesEnvGate(t *testing.T) {
	config, err := ResolveSelfVerifyLLMEvalConfig(false, false, "advisory", false, envLookupForSelfVerifyTest(map[string]string{
		selfverify.LLMEvalEnvName: "gate",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !config.Enabled || config.Mode != "gate" {
		t.Fatalf("ISSUEOPS_SELF_VERIFY_LLM_EVAL=gate should enable gate mode, got %+v", config)
	}
}

func TestResolveSelfVerifyLLMEvalConfigCLIOverridesEnv(t *testing.T) {
	config, err := ResolveSelfVerifyLLMEvalConfig(true, false, "advisory", false, envLookupForSelfVerifyTest(map[string]string{
		selfverify.LLMEvalEnvName: "strict",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if config.Enabled || config.Mode != "advisory" {
		t.Fatalf("explicit --llm-eval=false should ignore env and keep default advisory mode, got %+v", config)
	}
}

func TestResolveSelfVerifyLLMEvalConfigRejectsInvalidEnv(t *testing.T) {
	_, err := ResolveSelfVerifyLLMEvalConfig(false, false, "advisory", false, envLookupForSelfVerifyTest(map[string]string{
		selfverify.LLMEvalEnvName: "strict",
	}))
	if err == nil || !strings.Contains(err.Error(), selfverify.LLMEvalEnvName) {
		t.Fatalf("expected env validation error naming %s, got %v", selfverify.LLMEvalEnvName, err)
	}
}

func TestParseSelfVerifyLLMEvalEnvParsesDisabledAliasesAndRejectsUnknown(t *testing.T) {
	for _, value := range []string{"", "0", "false", "no", "off", "disabled"} {
		enabled, mode, err := selfverify.ParseLLMEvalEnv(value)
		if err != nil || enabled || mode != "advisory" {
			t.Fatalf("ParseSelfVerifyLLMEvalEnv(%q) enabled=%v mode=%q err=%v", value, enabled, mode, err)
		}
	}
	enabled, mode, err := selfverify.ParseLLMEvalEnv(" gate ")
	if err != nil || !enabled || mode != "gate" {
		t.Fatalf("gate env parse enabled=%v mode=%q err=%v", enabled, mode, err)
	}
	if _, _, err := selfverify.ParseLLMEvalEnv("maybe"); err == nil || !strings.Contains(err.Error(), selfverify.LLMEvalEnvName) {
		t.Fatalf("expected named env parse error, got %v", err)
	}
}

func TestBoundedLLMEvalErrorStaysWithinBudget(t *testing.T) {
	bounded := BoundedLLMEvalError("parse host judgement JSON", errors.New("bad"), strings.Repeat("x", 2048))
	if len(bounded) > 512 {
		t.Fatalf("host judgement error should be bounded, got %d bytes", len(bounded))
	}
}

func TestSelfVerifyLLMEvalRendersPromptOnlyResult(t *testing.T) {
	result := augmentcontract.SelfAugmentResult{OK: true, TerminationEligible: true}
	updated, err := ApplySelfVerifyLLMEval(result, selfverifyx.LLMEvalOptions{Enabled: true, Mode: "advisory", TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory prompt-only eval should be recorded, not returned as gate error: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || updated.LLMEval.OK || updated.LLMEval.Prompt == "" || !strings.Contains(updated.LLMEval.Error, "external LLM evaluation was removed") {
		t.Fatalf("prompt-only eval should produce structured llm_eval result: %+v", updated)
	}
}

func TestSelfVerifyLLMEvalResultClassifiesForegroundReadOnlyGate(t *testing.T) {
	result := augmentcontract.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: augmentcontract.SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true}}
	updated, _ := ApplySelfVerifyLLMEval(result, selfverifyx.LLMEvalOptions{Enabled: true, Mode: "advisory", TargetScore: 95})
	if updated.LLMEval == nil {
		t.Fatal("expected llm_eval result")
	}
	if updated.LLMEval.ExecutionClass != "foreground_blocking" || !updated.LLMEval.ReadOnly {
		t.Fatalf("expected foreground read-only LLM gate classification, got %+v", updated.LLMEval)
	}
}

func TestSelfVerifyLLMEvalGateFailsOnBlocker(t *testing.T) {
	result := augmentcontract.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: augmentcontract.SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true}}
	updated, err := ApplySelfVerifyLLMEval(result, selfverifyx.LLMEvalOptions{Enabled: true, Mode: "gate", TargetScore: 95})
	if err == nil || !strings.Contains(err.Error(), "LLM evaluation gate failed") {
		t.Fatalf("expected gate failure, got err=%v result=%+v", err, updated)
	}
	if updated.OK || updated.TerminationEligible || updated.Summary.TerminationEligible || updated.LLMEval == nil || updated.LLMEval.OK {
		t.Fatalf("gate failure must mark self-verify not OK: %+v", updated)
	}
}

func envLookupForSelfVerifyTest(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
