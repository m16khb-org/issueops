package selfverify

import "testing"

func TestStepPlanReusesOnlySuccessfulFullSuiteEvidence(t *testing.T) {
	order := StepOrder()
	if len(order) == 0 || order[2] != "risk QA tier" || order[3] != "go test" || order[4] != "contract golden tests" {
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
