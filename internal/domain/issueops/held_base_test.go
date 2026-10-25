package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

// Only commits from a base the cycle retargeted away from may be held by
// content; commits of the current base need ancestry (a tree-equal rewrite of
// the current base is a history purge).
func TestBaseHoldsWorkBaseAllowsContentOnlyForEarlierBases(t *testing.T) {
	record := model.IssueOpsRecord{
		BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "main", BaseSHA: "parent-tip", Retargets: []model.IssueOpsBranchRetarget{{FromBase: "41-parent", ToBase: "main"}}},
		Execution: &model.Execution{SyncBaseEvents: []model.ExecutionSyncBaseEvent{
			{BaseBranch: "41-parent", BaseOID: "parent-mid"},
			{BaseBranch: "main", BaseOID: "main-m1"},
		}},
	}
	commits := BaseCommitsInWork(record)
	want := []WorkBaseCommit{{"parent-tip", true}, {"parent-mid", true}, {"main-m1", false}}
	if len(commits) != len(want) {
		t.Fatalf("commits = %+v", commits)
	}
	for i := range want {
		if commits[i] != want[i] {
			t.Fatalf("commits = %+v, want %+v", commits, want)
		}
	}
	none := func(string) bool { return false }
	all := func(string) bool { return true }
	if !BaseHoldsWorkBase(commits, func(oid string) bool { return oid == "main-m1" }, all) {
		t.Fatal("squash-merged parent content with main-m1 an ancestor must be held")
	}
	if BaseHoldsWorkBase(commits, none, all) {
		t.Fatal("main-m1 dropped by a rewrite of main must not be held by content")
	}
	if BaseHoldsWorkBase(commits, func(oid string) bool { return oid == "main-m1" }, none) {
		t.Fatal("parent content missing from main must not be held")
	}
}
