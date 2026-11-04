package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
)

func TestComposeObservedPRReadinessKeepsGitAndArtifactEvidence(t *testing.T) {
	record := model.Record{AISlopCleanHead: "record-head", AISlopCleanFingerprint: "record-fingerprint"}
	base := model.Readiness{Ready: false, Missing: []string{"prior"}}
	facts := observedPRReadinessFacts{
		Git:         cycledomain.PRGitFacts{RootAvailable: false},
		Artifact:    observedPRFacts{PlanInWorktree: true},
		CurrentHead: "observed-head", CurrentFingerprint: "observed-fingerprint",
	}
	got := composeObservedPRReadiness(record, base, facts)
	if got.Ready || !reflect.DeepEqual(got.Missing, []string{"prior", "repo", "worktree_path"}) {
		t.Fatalf("readiness=%+v", got)
	}
	if got.CurrentHead != facts.CurrentHead || got.CurrentFingerprint != facts.CurrentFingerprint || got.AISlopCleanHead != record.AISlopCleanHead || got.AISlopCleanFingerprint != record.AISlopCleanFingerprint {
		t.Fatalf("evidence binding=%+v", got)
	}
}

func TestApplyChildPRGatePreservesWarningsAndSortsMissing(t *testing.T) {
	ready := model.Readiness{Ready: true, Missing: []string{"z"}, Warnings: []string{"first"}}
	got := applyChildPRGate(ready, []string{"a", "z"}, []string{"child"})
	if got.Ready || !reflect.DeepEqual(got.Missing, []string{"a", "z"}) || !reflect.DeepEqual(got.Warnings, []string{"first", "child"}) {
		t.Fatalf("child readiness=%+v", got)
	}
}
