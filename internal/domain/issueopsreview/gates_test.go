package issueopsreview

import (
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestReviewPublicationGates(t *testing.T) {
	if got := ImplementationReviewMissing(false, reviewcontract.ReviewGateEvidence{}, ""); got != "" {
		t.Fatalf("unprepared execution requires review: %q", got)
	}
	if got := ImplementationReviewMissing(true, reviewcontract.ReviewGateEvidence{}, ""); got != "implementation_review" {
		t.Fatalf("missing implementation review = %q", got)
	}
	review := reviewcontract.ReviewGateEvidence{Present: true, Verdict: "revise", ReviewedFingerprint: "old"}
	if got := ImplementationReviewMissing(true, review, "new"); got != "implementation_review_verdict_revise" {
		t.Fatalf("verdict must precede stale check: %q", got)
	}
	review.Verdict = "pass"
	if got := ImplementationReviewMissing(true, review, "new"); got != "implementation_review_stale" {
		t.Fatalf("stale implementation review = %q", got)
	}
	if got := ProjectDocsReviewMissing(reviewcontract.ReviewGateEvidence{}, ""); got != "project_docs_review" {
		t.Fatalf("missing project docs review = %q", got)
	}
	if got := ProjectDocsReviewMissing(review, "new"); got != "project_docs_review_stale" {
		t.Fatalf("stale project docs review = %q", got)
	}
}

func TestSchemaEvidencePublicationGate(t *testing.T) {
	if got := SchemaEvidenceMissing(false, reviewcontract.SchemaGateEvidence{}, ""); got != "" {
		t.Fatalf("non-schema change requires evidence: %q", got)
	}
	if got := SchemaEvidenceMissing(true, reviewcontract.SchemaGateEvidence{}, ""); got != "schema_evidence" {
		t.Fatalf("missing schema evidence = %q", got)
	}
	evidence := reviewcontract.SchemaGateEvidence{Present: true, Waived: true, WaiverRationale: "reason", ReviewedFingerprint: "old"}
	if got := SchemaEvidenceMissing(true, evidence, "new"); got != "schema_evidence_stale" {
		t.Fatalf("stale schema waiver = %q", got)
	}
	evidence.Waived = false
	evidence.MeasurementCount, evidence.SourceCount = 1, 0
	if got := SchemaEvidenceMissing(true, evidence, ""); got != "schema_evidence" {
		t.Fatalf("partial measurements accepted: %q", got)
	}
}

func TestChangeSetTouchesSchema(t *testing.T) {
	if ChangeSetTouchesSchema([]string{"README.md", "main.go"}) {
		t.Fatal("unrelated change set activated schema gate")
	}
	if !ChangeSetTouchesSchema([]string{"README.md", "internal/adapter/outbound/issueopslease/migrations/001.sql"}) {
		t.Fatal("migration change did not activate schema gate")
	}
}
