package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
)

func TestComposeObservedPRReadinessKeepsGitAndArtifactEvidence(t *testing.T) {
	record := model.IssueOpsRecord{AISlopCleanHead: "record-head", AISlopCleanFingerprint: "record-fingerprint"}
	base := model.IssueOpsReadiness{Ready: false, Missing: []string{"prior"}}
	facts := ObservedPRReadinessFacts{
		Git:         cycledomain.PRGitFacts{RootAvailable: false},
		Artifact:    ObservedPRFacts{PlanInWorktree: true},
		CurrentHead: "observed-head", CurrentFingerprint: "observed-fingerprint",
	}
	got := ComposeObservedPRReadiness(record, base, facts)
	if got.Ready || !reflect.DeepEqual(got.Missing, []string{"prior", "repo", "worktree_path"}) {
		t.Fatalf("readiness=%+v", got)
	}
	if got.CurrentHead != facts.CurrentHead || got.CurrentFingerprint != facts.CurrentFingerprint || got.AISlopCleanHead != record.AISlopCleanHead || got.AISlopCleanFingerprint != record.AISlopCleanFingerprint {
		t.Fatalf("evidence binding=%+v", got)
	}
}

func TestApplyChildPRGatePreservesWarningsAndSortsMissing(t *testing.T) {
	ready := model.IssueOpsReadiness{Ready: true, Missing: []string{"z"}, Warnings: []string{"first"}}
	got := ApplyChildPRGate(ready, []string{"a", "z"}, []string{"child"})
	if got.Ready || !reflect.DeepEqual(got.Missing, []string{"a", "z"}) || !reflect.DeepEqual(got.Warnings, []string{"first", "child"}) {
		t.Fatalf("child readiness=%+v", got)
	}
}
