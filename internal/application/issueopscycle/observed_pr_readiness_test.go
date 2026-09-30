package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestObservedPRReadinessMissingKeepsStaleAndPathGates(t *testing.T) {
	record := model.IssueOpsRecord{
		PlanPath: "plan.md", WorktreePath: "tree", AISlopCleanAt: "now", AISlopCleanFingerprint: "old",
		Execution:            &model.Execution{},
		ImplementationReview: &model.IssueOpsImplementationReview{Verdict: "pass", ReviewedFingerprint: "old"},
		ProjectDocsReview:    &model.IssueOpsProjectDocsReview{ReviewedFingerprint: "old"},
	}
	facts := ObservedPRFacts{CurrentFingerprint: "new", SchemaMissing: "schema_evidence_stale"}
	want := []string{"implementation_review_stale", "project_docs_review_stale", "schema_evidence_stale", "ai_slop_clean_stale", "plan_exists", "plan_in_worktree", "worktree_exists"}
	if got := ObservedPRReadinessMissing(record, facts); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
	// Non-stale review errors already came from local readiness and must not be repeated.
	record.ImplementationReview.Verdict = "revise"
	record.ProjectDocsReview = nil
	facts = ObservedPRFacts{PlanExists: true, PlanInWorktree: true, WorktreeValid: true}
	if got := ObservedPRReadinessMissing(record, facts); !reflect.DeepEqual(got, []string{"current_fingerprint"}) {
		t.Fatalf("non-stale review must not be repeated: %v", got)
	}
}
