package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestManualHandoffAuthorityRejectsEveryClaimFieldBeforeAppend(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-manual", Execution: &model.Execution{Mode: model.ExecutionModeDirect, Lease: model.WriteLease{Status: model.LeaseStatusReleased, Generation: 3}}}
	observation := model.IssueOpsHandoffDeliveryObservation{LifecycleID: record.ID, SourceGeneration: 3, AttemptID: "manual-direct:io-manual:3:attempt", LineageID: "manual-direct:lineage", Launcher: model.IssueOpsHandoffDeliveryLauncher{Name: "cmux"}, OwnerClaimed: model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateNotObserved}}
	if err := ValidateManualHandoffDeliveryObservation(record, observation); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.IssueOpsHandoffDeliveryObservation){
		"actor":      func(o *model.IssueOpsHandoffDeliveryObservation) { o.OwnerActor = &model.NativeActor{} },
		"state":      func(o *model.IssueOpsHandoffDeliveryObservation) { o.OwnerClaimed.Status = "" },
		"claimed":    func(o *model.IssueOpsHandoffDeliveryObservation) { o.OwnerClaim.Claimed = true },
		"generation": func(o *model.IssueOpsHandoffDeliveryObservation) { o.OwnerClaim.Generation = 1 },
		"time":       func(o *model.IssueOpsHandoffDeliveryObservation) { o.OwnerClaim.ClaimedAt = "x" },
		"host":       func(o *model.IssueOpsHandoffDeliveryObservation) { o.OwnerClaim.Actor.Host = "codex" },
		"session":    func(o *model.IssueOpsHandoffDeliveryObservation) { o.OwnerClaim.Actor.SessionID = "x" },
		"agent":      func(o *model.IssueOpsHandoffDeliveryObservation) { o.OwnerClaim.Actor.AgentID = "x" },
		"process": func(o *model.IssueOpsHandoffDeliveryObservation) {
			o.OwnerClaim.Actor.SessionProcess = &model.NativeProcessReceipt{}
		},
		"ancestry": func(o *model.IssueOpsHandoffDeliveryObservation) {
			o.OwnerClaim.Actor.ProcessAncestry = []model.NativeProcessReceipt{{}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			o := observation
			mutate(&o)
			if err := ValidateManualHandoffDeliveryObservation(record, o); err == nil || err.Error() != "manual handoff delivery observation cannot produce owner claim evidence" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestManualHandoffAuthorityPreservesRefusalOrder(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-manual", Execution: &model.Execution{Mode: model.ExecutionModeDirect, Lease: model.WriteLease{Status: model.LeaseStatusReleased, Generation: 3}}}
	observation := model.IssueOpsHandoffDeliveryObservation{LifecycleID: record.ID, SourceGeneration: 2, AttemptID: "wrong", LineageID: "wrong", Launcher: model.IssueOpsHandoffDeliveryLauncher{Name: "wrong"}, OwnerActor: &model.NativeActor{}}
	checks := []struct {
		fix  func()
		want string
	}{
		{func() {}, "manual handoff delivery observation requires the exact released direct execution generation"},
		{func() { observation.SourceGeneration = 3 }, "manual handoff delivery observation requires an isolated manual-direct namespace"},
		{func() {
			observation.AttemptID = "manual-direct:io-manual:3:attempt"
			observation.LineageID = "manual-direct:lineage"
		}, "manual handoff delivery observation launcher must be Orca, Herdr, or cmux"},
		{func() { observation.Launcher.Name = "herdr" }, "manual handoff delivery observation cannot produce owner claim evidence"},
	}
	for _, check := range checks {
		check.fix()
		if err := ValidateManualHandoffDeliveryObservation(record, observation); err == nil || err.Error() != check.want {
			t.Fatalf("err=%v want=%s", err, check.want)
		}
	}
}
