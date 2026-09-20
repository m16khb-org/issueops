package issueopsapp

import "testing"

func TestConformanceModelOverridesRejectMalformedAndDuplicateValues(t *testing.T) {
	models, err := conformanceModelOverrides([]string{"codex=default", "claude=sonnet"})
	if err != nil || models["codex"] != "default" || models["claude"] != "sonnet" {
		t.Fatalf("models=%v err=%v", models, err)
	}
	for _, values := range [][]string{{"missing"}, {"codex="}, {"codex=one", "codex=two"}} {
		if _, err := conformanceModelOverrides(values); err == nil {
			t.Fatalf("invalid overrides accepted: %v", values)
		}
	}
}

func TestToolConformanceRunnersIncludeNativeOmo(t *testing.T) {
	runners := toolConformanceRunners("/private/bin/issueops")
	for _, host := range []string{"codex", "claude", "omo"} {
		runner := runners[host]
		if runner == nil || runner.Name() != host {
			t.Fatalf("runner %q = %#v", host, runner)
		}
	}
	if len(runners) != 3 {
		t.Fatalf("runners = %#v", runners)
	}
}
