package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestCmuxAttemptRejectsSameLineageAndStagedGeneration(t *testing.T) {
	request := model.ExecutionCmuxHandoffRequest{ID: "cycle", Generation: 3, PromptSHA256: strings.Repeat("a", 64), MaterialSHA256: strings.Repeat("b", 64)}
	_, lineage := ManualCmuxHandoffIDs(request)
	for _, tc := range []struct {
		name        string
		observation model.IssueOpsHandoffDeliveryObservation
		want        string
	}{
		{"same lineage", model.IssueOpsHandoffDeliveryObservation{LifecycleID: "cycle", LineageID: lineage}, "attempt already exists"},
		{"staged generation", model.IssueOpsHandoffDeliveryObservation{LifecycleID: "cycle", SourceGeneration: 3, Launcher: model.IssueOpsHandoffDeliveryLauncher{Name: model.IssueOpsHandoffDeliveryLauncherCmux}, CallStaged: model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateObserved}}, "generation already has a staged attempt"},
		{"another cycle", model.IssueOpsHandoffDeliveryObservation{LifecycleID: "other", LineageID: lineage}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCmuxAttempt(request, []model.IssueOpsHandoffDeliveryObservation{tc.observation})
			if (err == nil) != (tc.want == "") || (err != nil && !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
func TestCmuxReleasedGenerationAndResultAuthority(t *testing.T) {
	r := model.IssueOpsRecord{Execution: &model.Execution{Mode: model.ExecutionModeDirect, Lease: model.WriteLease{Status: model.LeaseStatusReleased, Generation: 3}}}
	if err := ValidateCmuxReleasedRecord(r, 3); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCmuxReleasedRecord(r, 4); err == nil {
		t.Fatal("wrong generation accepted")
	}
	obs := model.IssueOpsHandoffDeliveryObservation{InputAccepted: CmuxObserved("now", "raw input"), Ambiguous: CmuxNotObserved()}
	got := CmuxResult(model.ExecutionCmuxHandoffRequest{}, obs, "input_accepted", "")
	if !got.OK || got.NativeTurnObserved || got.OwnerClaimed {
		t.Fatalf("raw input promoted authority: %+v", got)
	}
	obs.Ambiguous = CmuxObserved("now", "response lost")
	if CmuxResult(model.ExecutionCmuxHandoffRequest{}, obs, "ambiguous", "").OK {
		t.Fatal("ambiguous input accepted")
	}
}
