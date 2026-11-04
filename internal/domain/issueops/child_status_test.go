package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestChildStatusDistinguishesArchivedReceiptsAndLiveReappearance(t *testing.T) {
	parent := model.Record{ID: "parent", Repo: "/repo", Phase: model.PhaseDone, ChildCycles: []model.ChildCycleRef{
		{CycleID: "accepted", ValidationVerdict: "accepted", ValidatedAt: "time", ValidationEvidence: []string{"verified"}},
		{CycleID: "dropped", ValidationVerdict: "dropped", ValidatedAt: "time", ValidationReason: "removed from scope"},
		{CycleID: "missing-evidence", ValidationVerdict: "accepted", ValidatedAt: "time"},
		{CycleID: "reappeared", ValidationVerdict: "accepted", ValidatedAt: "time", ValidationEvidence: []string{"old receipt"}},
		{CycleID: "short-reason", ValidationVerdict: "dropped", ValidatedAt: "time", ValidationReason: "short"},
	}}
	records := []model.Record{
		{ID: "reappeared", Repo: "/repo", Phase: model.PhaseProblem, UpdatedAt: "updated", Delegation: &model.DelegationContract{ParentCycleID: "parent"}},
		{ID: "scanned", Repo: "/repo", Phase: model.PhaseProblem, CreatedAt: "created", Delegation: &model.DelegationContract{ParentCycleID: "parent"}},
		{ID: "other-repo", Repo: "/other", Delegation: &model.DelegationContract{ParentCycleID: "parent"}},
		{ID: "other-parent", Repo: "/repo", Delegation: &model.DelegationContract{ParentCycleID: "other"}},
	}
	status := BuildChildStatus(parent, SelectChildren(parent, records))
	if !reflect.DeepEqual(status.Orphaned, []string{"missing-evidence", "short-reason"}) {
		t.Fatalf("orphans: %v", status.Orphaned)
	}
	var ids []string
	entries := map[string]model.ChildStatusEntry{}
	for _, entry := range status.Children {
		ids = append(ids, entry.CycleID)
		entries[entry.CycleID] = entry
	}
	if !reflect.DeepEqual(ids, []string{"accepted", "dropped", "missing-evidence", "reappeared", "scanned", "short-reason"}) {
		t.Fatalf("children: %v", ids)
	}
	if entries["accepted"].Phase != model.PhaseDone || entries["accepted"].Orphaned || entries["dropped"].Orphaned {
		t.Fatal("durable cleanup receipts lost")
	}
	if entry := entries["reappeared"]; entry.Phase != model.PhaseProblem || entry.ParentClosedState != "parent_closed" || !entry.Scanned || !entry.Indexed || entry.LastActiveAt != "updated" {
		t.Fatalf("reappeared: %+v", entry)
	}
	if entry := entries["scanned"]; entry.Indexed || !entry.Scanned || entry.LastActiveAt != "created" {
		t.Fatalf("scanned: %+v", entry)
	}
}

func TestRepairChildIndexPreservesExistingReceiptsAndInput(t *testing.T) {
	refs := make([]model.ChildCycleRef, 1, 4)
	refs[0] = model.ChildCycleRef{CycleID: "old", Title: "keep", ValidationVerdict: "accepted"}
	parent := model.Record{ChildCycles: refs}
	children := map[string]model.Record{
		"old": {ID: "old", Branch: "overwritten"},
		"b":   {ID: "b", Branch: "branch-b", CreatedAt: "created", Delegation: &model.DelegationContract{DelegatedAt: "delegated", ChildIssueURL: " url "}},
		"a":   {ID: "a", Branch: "branch-a", CreatedAt: "created"},
	}
	updated, appended := RepairChildIndex(parent, children)
	if !reflect.DeepEqual(appended, []string{"a", "b"}) {
		t.Fatalf("append order: %v", appended)
	}
	if !reflect.DeepEqual(updated.ChildCycles[0], refs[0]) {
		t.Fatal("existing receipt replaced")
	}
	if updated.ChildCycles[2].CreatedAt != "delegated" || updated.ChildCycles[2].ChildIssueURL != "url" {
		t.Fatalf("derived ref: %+v", updated.ChildCycles[2])
	}
	if len(parent.ChildCycles) != 1 || refs[:cap(refs)][1].CycleID != "" {
		t.Fatal("repair mutated input storage")
	}
	again, appended := RepairChildIndex(updated, children)
	if len(appended) != 0 || !reflect.DeepEqual(again, updated) {
		t.Fatal("repair is not idempotent")
	}
}

func TestMalformedArchivedAcceptanceCannotSatisfyChildGates(t *testing.T) {
	parent := model.Record{ID: "parent", ChildCycles: []model.ChildCycleRef{{CycleID: "child", ValidationVerdict: "accepted", ValidatedAt: "time"}}}
	status := BuildChildStatus(parent, nil)
	if !status.Children[0].Orphaned {
		t.Fatal("missing evidence must remain orphaned")
	}
	if got := ChildPRGateMissing(status.Children); !reflect.DeepEqual(got, []string{"child_incomplete:child"}) {
		t.Fatalf("orphan cleared PR gate: %v", got)
	}
	if got := ActiveChildIDs(status.Children); !reflect.DeepEqual(got, []string{"child"}) {
		t.Fatalf("orphan cleared active child gate: %v", got)
	}
}
