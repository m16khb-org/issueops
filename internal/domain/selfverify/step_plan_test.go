package selfverify

import "testing"

func TestStepPlanReusesOnlySuccessfulFullSuiteEvidence(t *testing.T) {
	order := StepOrder()
	if len(order) == 0 || order[3] != "risk QA tier" || order[4] != "go test" || order[5] != "contract golden tests" {
		t.Fatalf("unexpected verification order: %v", order)
	}
	if ReuseRiskRaceAsFullTest(false, true) || ReuseRiskRaceAsFullTest(true, false) || !ReuseRiskRaceAsFullTest(true, true) {
		t.Fatal("risk race reuse requires a successful full test")
	}
	if ReuseFullTestAsGolden(false) || !ReuseFullTestAsGolden(true) {
		t.Fatal("golden reuse requires a successful full test")
	}
	if ContinueAfterFailure(false) || !ContinueAfterFailure(true) {
		t.Fatal("collect-all failure policy changed")
	}
}
