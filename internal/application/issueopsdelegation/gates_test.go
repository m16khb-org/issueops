package issueopsdelegation

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestChildGatesScopeOneScanToCurrentParent(t *testing.T) {
	parent := model.IssueOpsRecord{ID: "parent", Repo: "/repo", ChildCycles: []model.IssueOpsChildCycleRef{{CycleID: "missing"}}}
	records := []model.IssueOpsRecord{
		{ID: "active", Repo: "/repo", Phase: model.IssueOpsPhaseImplement, Delegation: &model.IssueOpsDelegationContract{ParentCycleID: "parent"}},
		{ID: "other-repo", Repo: "/other", Phase: model.IssueOpsPhaseImplement, Delegation: &model.IssueOpsDelegationContract{ParentCycleID: "parent"}},
		{ID: "other-parent", Repo: "/repo", Phase: model.IssueOpsPhaseImplement, Delegation: &model.IssueOpsDelegationContract{ParentCycleID: "other"}},
	}
	reads := 0
	gates := ChildGates{Scan: func(root string) ([]model.IssueOpsRecord, error) {
		reads++
		if root != "state" {
			t.Fatalf("untrimmed state root: %q", root)
		}
		return records, nil
	}}
	missing, warnings := gates.PRMissing(" state ", parent)
	if reads != 1 || len(warnings) != 0 || !reflect.DeepEqual(missing, []string{"child_incomplete:active", "child_incomplete:missing"}) {
		t.Fatalf("reads=%d missing=%v warnings=%v", reads, missing, warnings)
	}
	active, err := gates.ActiveIDs("state", parent)
	if err != nil || reads != 2 || !reflect.DeepEqual(active, []string{"active", "missing"}) {
		t.Fatalf("reads=%d active=%v err=%v", reads, active, err)
	}
}

func TestChildGatesFailClosedOnScanErrorAndSkipUnconfiguredState(t *testing.T) {
	reads := 0
	gates := ChildGates{Scan: func(string) ([]model.IssueOpsRecord, error) { reads++; return nil, errors.New("storage unreadable") }}
	parent := model.IssueOpsRecord{ID: "parent", Repo: "/repo"}
	missing, warnings := gates.PRMissing("state", parent)
	if !reflect.DeepEqual(missing, []string{"children_complete"}) || !reflect.DeepEqual(warnings, []string{"failed to scan delegated children: storage unreadable"}) {
		t.Fatalf("scan failure treated as no children: %v %v", missing, warnings)
	}
	active, err := gates.ActiveIDs("state", parent)
	if len(active) != 0 || err == nil || !strings.Contains(err.Error(), "children_active scan failed: storage unreadable") {
		t.Fatalf("active child scan failure lost: %v %v", active, err)
	}
	missing, warnings = gates.PRMissing(" \t", parent)
	active, err = gates.ActiveIDs("", parent)
	if reads != 2 || len(missing) != 0 || len(warnings) != 0 || len(active) != 0 || err != nil {
		t.Fatalf("unconfigured state was scanned: reads=%d missing=%v warnings=%v active=%v err=%v", reads, missing, warnings, active, err)
	}
}
