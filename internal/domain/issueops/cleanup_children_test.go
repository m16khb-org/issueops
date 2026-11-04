package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestChildCleanupRequiresRequestedEvidenceAndLinkedParent(t *testing.T) {
	record := model.Record{ID: "parent"}
	result, err := PrepareChildCleanup(record, model.CloseChildrenRequest{})
	if err == nil || !reflect.DeepEqual(result.Missing, []string{"merge_evidence"}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	result, err = PrepareChildCleanup(record, model.CloseChildrenRequest{MergeEvidenceRequested: true})
	if err == nil || !reflect.DeepEqual(result.Missing, []string{"parent_issue"}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/1"
	result, err = PrepareChildCleanup(record, model.CloseChildrenRequest{Merged: true, Confirm: true})
	if err != nil || !result.OK || !result.Merged || !result.Confirmed || result.DryRun {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestChildCleanupEvidenceNeverBypassesUnmergedArtifact(t *testing.T) {
	record := model.Record{RemoteArtifact: &model.RemoteArtifactVerification{}}
	basis, err := ChildCleanupEvidenceBasis(record, false)
	if err == nil || basis != "" {
		t.Fatalf("unmerged parent accepted: %s %v", basis, err)
	}
	basis, err = ChildCleanupEvidenceBasis(record, true)
	if err != nil || basis != childCleanupParentMergeVerified {
		t.Fatalf("merged parent rejected: %s %v", basis, err)
	}
	record.RemoteArtifact = nil
	basis, err = ChildCleanupEvidenceBasis(record, false)
	if err != nil || basis != "" {
		t.Fatalf("fallback must still require observation: %s %v", basis, err)
	}
	for _, state := range []string{"", "open", "unknown"} {
		if err := ValidateClosedChildObservation("child", state); err == nil {
			t.Fatalf("accepted %q", state)
		}
	}
	if err := ValidateClosedChildObservation("child", "CLOSED"); err != nil {
		t.Fatal(err)
	}
}

func TestChildCleanupReceiptIsImmutableAndPreservesFirstClose(t *testing.T) {
	record := model.Record{IssueLinks: []model.IssueLink{{Type: "related", URL: "related"}, {Type: "child", URL: "one", ClosedAt: "first"}, {Type: "child", URL: "two"}}}
	before := append([]model.IssueLink(nil), record.IssueLinks...)
	got := ApplyChildCleanupReceipts(record, []int{1, 2}, "now")
	if !reflect.DeepEqual(record.IssueLinks, before) {
		t.Fatal("input receipt mutated")
	}
	if got.UpdatedAt != "now" || got.IssueLinks[0] != before[0] || got.IssueLinks[1].ClosedAt != "first" || got.IssueLinks[2].ClosedAt != "now" {
		t.Fatalf("receipt=%+v", got)
	}
	for _, index := range []int{1, 2} {
		if got.IssueLinks[index].CloseVerifiedAt != "now" || got.IssueLinks[index].CloseReason != "completed" {
			t.Fatalf("receipt=%+v", got)
		}
	}
	for _, child := range []model.CloseChildResult{{URL: "one", Closed: true}, {URL: "one", HierarchyVerified: true}} {
		if err := ValidateChildCloseConfirmation(child, true); err == nil {
			t.Fatal("unverified close accepted")
		}
		if err := ValidateChildCloseConfirmation(child, false); err != nil {
			t.Fatal("preview should not require close")
		}
	}
	if err := ValidateChildCloseConfirmation(model.CloseChildResult{Closed: true, HierarchyVerified: true}, true); err != nil {
		t.Fatal(err)
	}
}
