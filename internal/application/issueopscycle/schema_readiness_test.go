package issueopscycle

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestSchemaEvidenceMissingForPathsMapsConditionalReview(t *testing.T) {
	record := model.IssueOpsRecord{}
	changed := []string{"internal/adapter/outbound/issueopslease/migrations/001.sql"}
	if got := SchemaEvidenceMissingForPaths(record, changed, ""); got != "schema_evidence" {
		t.Fatalf("missing schema evidence=%q", got)
	}
	record.SchemaEvidence = &model.IssueOpsSchemaEvidence{Waived: true, WaiverRationale: "measured elsewhere", ReviewedFingerprint: "old"}
	if got := SchemaEvidenceMissingForPaths(record, changed, "new"); got != "schema_evidence_stale" {
		t.Fatalf("stale schema evidence=%q", got)
	}
}

func TestObservedSchemaEvidenceMissingReobservesOnlyStrictExecution(t *testing.T) {
	record := model.IssueOpsRecord{}
	calls := 0
	observe := func(model.IssueOpsRecord) []string {
		calls++
		return []string{"db/migrations/001.sql"}
	}
	if got := ObservedSchemaEvidenceMissing(record, true, nil, "", observe); got != "" || calls != 0 {
		t.Fatalf("no execution schema=%q calls=%d", got, calls)
	}
	record.Execution = &model.Execution{}
	if got := ObservedSchemaEvidenceMissing(record, false, nil, "", observe); got != "" || calls != 0 {
		t.Fatalf("local observed paths schema=%q calls=%d", got, calls)
	}
	if got := ObservedSchemaEvidenceMissing(record, true, nil, "", observe); got != "schema_evidence" || calls != 1 {
		t.Fatalf("strict reobservation schema=%q calls=%d", got, calls)
	}
}
