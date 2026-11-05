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
		observation model.HandoffDeliveryObservation
		want        string
	}{
		{"same lineage", model.HandoffDeliveryObservation{LifecycleID: "cycle", LineageID: lineage}, "attempt already exists"},
		{"staged generation", model.HandoffDeliveryObservation{LifecycleID: "cycle", SourceGeneration: 3, Launcher: model.HandoffDeliveryLauncher{Name: model.HandoffDeliveryLauncherCmux}, CallStaged: model.HandoffDeliveryState{Status: model.HandoffDeliveryStateObserved}}, "generation already has a staged attempt"},
		{"another cycle", model.HandoffDeliveryObservation{LifecycleID: "other", LineageID: lineage}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCmuxAttempt(request, []model.HandoffDeliveryObservation{tc.observation})
			if (err == nil) != (tc.want == "") || (err != nil && !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
func TestCmuxReleasedGenerationAndResultAuthority(t *testing.T) {
	r := model.Record{Execution: &model.Execution{Lease: model.WriteLease{Status: model.LeaseStatusReleased, Generation: 3}}}
	if err := ValidateCmuxReleasedRecord(r, 3); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCmuxReleasedRecord(r, 4); err == nil {
		t.Fatal("wrong generation accepted")
	}
	obs := model.HandoffDeliveryObservation{InputAccepted: CmuxObserved("now", "raw input"), Ambiguous: CmuxNotObserved()}
	got := CmuxResult(model.ExecutionCmuxHandoffRequest{}, obs, "input_accepted", "")
	if !got.OK || got.NativeTurnObserved || got.OwnerClaimed {
		t.Fatalf("raw input promoted authority: %+v", got)
	}
	obs.Ambiguous = CmuxObserved("now", "response lost")
	if CmuxResult(model.ExecutionCmuxHandoffRequest{}, obs, "ambiguous", "").OK {
		t.Fatal("ambiguous input accepted")
	}
}
