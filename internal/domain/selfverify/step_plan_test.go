package selfverify

import "testing"

func TestStepPlanReusesOnlySuccessfulFullSuiteEvidence(t *testing.T) {
	order := StepOrder()
	if len(order) == 0 || order[3] != "Go test match guard" || order[4] != "risk QA tier" || order[5] != "go test" || order[6] != "contract golden tests" {
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

func TestNotRunStepsKeepsPlanOrderAndExcludesExecutedLabels(t *testing.T) {
	order := StepOrder()
	got := NotRunSteps([]string{order[0], order[1], "create temp workspace"})
	if len(got) != len(order)-2 || got[0] != order[2] || got[len(got)-1] != order[len(order)-1] {
		t.Fatalf("not run=%v", got)
	}
	if all := NotRunSteps(order); len(all) != 0 || all == nil {
		t.Fatalf("fully executed plan must report an empty non-nil list: %#v", all)
	}
	if none := NotRunSteps(nil); len(none) != len(order) {
		t.Fatalf("nothing executed must report the whole plan: %v", none)
	}
}

func TestCoverageForLabelsMarksClaimsWithNoExecutedLabel(t *testing.T) {
	coverage, _ := CoverageForLabels([]string{"go test"})
	byClaim := map[string]bool{}
	for _, item := range coverage {
		byClaim[item.Claim] = item.NotRun
	}
	// test suite contract: one of four labels ran -> partially covered, not "not run".
	if byClaim["test suite contract"] || !byClaim["release build artifact"] {
		t.Fatalf("not_run by claim=%v", byClaim)
	}
}
