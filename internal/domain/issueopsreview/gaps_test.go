package issueopsreview

import (
	"slices"
	"strings"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestValidateAISlopCleanEvidenceRequiresCategoriesBeforeVerification(t *testing.T) {
	if err := ValidateAISlopCleanEvidence(nil, nil); err == nil || !strings.Contains(err.Error(), "cleanup category") {
		t.Fatalf("missing categories must be rejected first: %v", err)
	}
	if err := ValidateAISlopCleanEvidence([]string{"dead-code"}, nil); err == nil || !strings.Contains(err.Error(), "verification entry") {
		t.Fatalf("missing verification error = %v", err)
	}
	if err := ValidateAISlopCleanEvidence([]string{"dead-code"}, []string{"go test"}); err != nil {
		t.Fatalf("complete evidence rejected: %v", err)
	}
}

func TestInheritedParentReviewIsWaivedPassNamingParent(t *testing.T) {
	review := InheritedParentReview("io-7", "2026-10-01T00:00:00Z")
	if review.Verdict != "pass" || !review.Waived || review.ReviewerPattern != ParentReviewPattern ||
		review.RecordedAt != "2026-10-01T00:00:00Z" || review.IssueReflectedAt != "" ||
		review.WaiverRationale != "delegated:io-7 parent DA verdict pass" {
		t.Fatalf("inherited review = %+v", review)
	}
}

func TestNonPlanPrepMissingDropsOnlyPlanPrepItems(t *testing.T) {
	got := NonPlanPrepMissing([]string{"plan_prep_grill", "design_review", "plan_prep_x", "my_plan_prep_y"})
	if want := []string{"design_review", "my_plan_prep_y"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := NonPlanPrepMissing(nil); got == nil || len(got) != 0 {
		t.Fatalf("empty input must give a non-nil empty slice, got %#v", got)
	}
}

func TestCheckRegressionChildrenAllowsNoActiveChildren(t *testing.T) {
	if err := CheckRegressionChildren(nil); err != nil {
		t.Fatalf("no active children must pass: %v", err)
	}
}

func TestReviseRoundCapErrorMessage(t *testing.T) {
	if got := (&ReviseRoundCapError{Count: 5}).Error(); got != "revise round cap reached" {
		t.Fatalf("message = %q", got)
	}
}

func TestValidateReviewRejectionBranches(t *testing.T) {
	base := reviewcontract.DevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"f"}}
	for _, tc := range []struct {
		name   string
		mutate func(*reviewcontract.DevilsAdvocateReviewRequest)
		want   string
	}{
		{"unknown verdict", func(r *reviewcontract.DevilsAdvocateReviewRequest) { r.Verdict = "approve" }, "verdict must be pass, revise, or stop"},
		{"unknown reviewer context", func(r *reviewcontract.DevilsAdvocateReviewRequest) { r.ReviewerContext = "human" }, "reviewer_context must be subagent or inline"},
		{"waiver without rationale", func(r *reviewcontract.DevilsAdvocateReviewRequest) { r.Waived, r.WaiverRationale = true, "  " }, "waived review requires waiver_rationale"},
		{"unwaived stop without findings", func(r *reviewcontract.DevilsAdvocateReviewRequest) { r.Verdict, r.Findings = "stop", []string{" "} }, "stop verdict requires findings or an explicit waiver"},
		{"unwaived revise without findings", func(r *reviewcontract.DevilsAdvocateReviewRequest) { r.Verdict, r.Findings = "revise", nil }, "revise verdict requires findings or an explicit waiver"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.mutate(&req)
			if _, err := ValidateReview(req, "now"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	waived := base
	waived.Verdict, waived.Findings, waived.Waived, waived.WaiverRationale = "stop", nil, true, " accepted risk "
	review, err := ValidateReview(waived, "now")
	if err != nil || review.Verdict != "stop" || !review.Waived || review.WaiverRationale != "accepted risk" || review.ReviewerPattern != "devils-advocate-review" {
		t.Fatalf("waived stop without findings = %+v %v", review, err)
	}
}

func TestApplyReviewWithoutPreviousReturnsNextUnchanged(t *testing.T) {
	next := reviewcontract.DevilsAdvocateReview{Verdict: "revise", RecordedAt: "t"}
	got, err := ApplyReview(nil, next)
	if err != nil || got.Verdict != "revise" || got.History != nil {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestApplyReviewWaivedReviseIgnoresCapAndWaivedRoundsDoNotCount(t *testing.T) {
	// History holds four unwaived rounds and the previous review is waived: if
	// waived rounds counted, the revise below would hit the cap of five.
	previous := &reviewcontract.DevilsAdvocateReview{Verdict: "revise", Waived: true, History: []reviewcontract.DevilsAdvocateRound{
		{Verdict: "revise"}, {Verdict: "revise"}, {Verdict: "revise"}, {Verdict: "revise"},
	}}
	fifth, err := ApplyReview(previous, reviewcontract.DevilsAdvocateReview{Verdict: "revise"})
	if err != nil || len(fifth.History) != 5 {
		t.Fatalf("waived round must not count toward the cap: %+v %v", fifth, err)
	}
	// Now five unwaived rounds exist: an unwaived revise is capped, a waived one is not.
	if _, err := ApplyReview(&fifth, reviewcontract.DevilsAdvocateReview{Verdict: "revise"}); err == nil {
		t.Fatal("unwaived revise past the cap must be rejected")
	}
	// ...but a waived revise passes.
	got, err := ApplyReview(&fifth, reviewcontract.DevilsAdvocateReview{Verdict: "revise", Waived: true})
	if err != nil || len(got.History) != 6 {
		t.Fatalf("waived revise = %+v %v", got, err)
	}
}

func TestPrepareDesignReviewRequiredFieldsInOrder(t *testing.T) {
	valid := reviewcontract.DesignReviewRequest{ProblemSummary: "p", ProposedDesign: "d", Verification: []string{"v"}}
	for _, tc := range []struct {
		name   string
		mutate func(*reviewcontract.DesignReviewRequest)
		want   string
	}{
		{"problem", func(r *reviewcontract.DesignReviewRequest) {
			r.ProblemSummary, r.ProposedDesign, r.Verification = " ", "", nil
		}, "problem_summary is required"},
		{"design", func(r *reviewcontract.DesignReviewRequest) { r.ProposedDesign, r.Verification = "", nil }, "proposed_design is required"},
		{"verification blank", func(r *reviewcontract.DesignReviewRequest) { r.Verification = []string{" "} }, "verification is required"},
		{"verification nul", func(r *reviewcontract.DesignReviewRequest) { r.Verification = []string{"a\x00b"} }, "verification is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)
			if _, err := PrepareDesignReview(req); err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	// Unapproved reviews may carry open questions; duplicates collapse.
	req := valid
	req.OpenQuestions = []string{"q", " q ", ""}
	review, err := PrepareDesignReview(req)
	if err != nil || len(review.OpenQuestions) != 1 || review.Approved {
		t.Fatalf("review = %+v %v", review, err)
	}
}

func TestFinalizeDesignReviewApprovedRequirementsInOrder(t *testing.T) {
	approved := reviewcontract.DesignReview{
		Approved: true, RefactorPlan: "plan", Alternatives: []string{"a"}, Risks: []string{"r"},
		Verification: []string{"design review done"},
	}
	for _, tc := range []struct {
		name   string
		mutate func(*reviewcontract.DesignReview)
		want   string
	}{
		{"refactor plan", func(r *reviewcontract.DesignReview) {
			r.RefactorPlan, r.Alternatives, r.Risks, r.Verification = "", nil, nil, []string{"ran tests"}
		}, "requires refactor_plan"},
		{"alternatives", func(r *reviewcontract.DesignReview) {
			r.Alternatives, r.Risks, r.Verification = nil, nil, []string{"ran tests"}
		}, "requires alternatives"},
		{"risks", func(r *reviewcontract.DesignReview) { r.Risks, r.Verification = nil, []string{"ran tests"} }, "requires risks"},
		{"evidence", func(r *reviewcontract.DesignReview) { r.Verification = []string{"ran tests"} }, ErrMissingDesignReviewEvidence.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			review := approved
			tc.mutate(&review)
			if _, err := FinalizeDesignReview(review, "now"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	// An unapproved review needs none of those and is only stamped.
	got, err := FinalizeDesignReview(reviewcontract.DesignReview{}, "now")
	if err != nil || got.ReviewedAt != "now" {
		t.Fatalf("unapproved finalize = %+v %v", got, err)
	}
}

func TestProjectDocsReviewMissingAcceptsMatchingOrUnfingerprinted(t *testing.T) {
	review := reviewcontract.ReviewGateEvidence{Present: true, ReviewedFingerprint: "fp"}
	if got := ProjectDocsReviewMissing(review, "fp"); got != "" {
		t.Fatalf("matching fingerprint = %q", got)
	}
	if got := ProjectDocsReviewMissing(review, ""); got != "" {
		t.Fatalf("empty current fingerprint disables stale check, got %q", got)
	}
}

func TestSchemaEvidenceMissingBranches(t *testing.T) {
	for _, tc := range []struct {
		name     string
		touches  bool
		evidence reviewcontract.SchemaGateEvidence
		current  string
		want     string
	}{
		{"untouched schema needs nothing", false, reviewcontract.SchemaGateEvidence{}, "fp", ""},
		{"absent evidence", true, reviewcontract.SchemaGateEvidence{}, "", "schema_evidence"},
		{"waived without rationale", true, reviewcontract.SchemaGateEvidence{Present: true, Waived: true, WaiverRationale: " "}, "", "schema_evidence"},
		{"waived with rationale", true, reviewcontract.SchemaGateEvidence{Present: true, Waived: true, WaiverRationale: "no ddl"}, "", ""},
		{"no measurements", true, reviewcontract.SchemaGateEvidence{Present: true, SourceCount: 1}, "", "schema_evidence"},
		{"no sources", true, reviewcontract.SchemaGateEvidence{Present: true, MeasurementCount: 1}, "", "schema_evidence"},
		{"complete", true, reviewcontract.SchemaGateEvidence{Present: true, MeasurementCount: 1, SourceCount: 1, ReviewedFingerprint: "fp"}, "fp", ""},
		{"stale", true, reviewcontract.SchemaGateEvidence{Present: true, MeasurementCount: 1, SourceCount: 1, ReviewedFingerprint: "old"}, "new", "schema_evidence_stale"},
		{"waived but stale", true, reviewcontract.SchemaGateEvidence{Present: true, Waived: true, WaiverRationale: "r", ReviewedFingerprint: "old"}, "new", "schema_evidence_stale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SchemaEvidenceMissing(tc.touches, tc.evidence, tc.current); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestImplementationReviewMissingPassingMatchingReview(t *testing.T) {
	review := reviewcontract.ReviewGateEvidence{Present: true, Verdict: "pass", ReviewedFingerprint: "fp"}
	if got := ImplementationReviewMissing(true, review, "fp"); got != "" {
		t.Fatalf("fresh passing review = %q", got)
	}
	if got := ImplementationReviewMissing(true, review, ""); got != "" {
		t.Fatalf("empty fingerprint disables stale check, got %q", got)
	}
}

func TestValidateCompatibilityReviewRejectionOrder(t *testing.T) {
	valid := reviewcontract.CompatibilityReviewRequest{
		BackwardCompatibility: []string{"b"}, SideEffects: []string{"s"}, Verification: []string{"v"}, RollbackPlan: "r",
	}
	for _, tc := range []struct {
		name   string
		mutate func(*reviewcontract.CompatibilityReviewRequest)
		want   string
	}{
		{"backward", func(r *reviewcontract.CompatibilityReviewRequest) {
			r.BackwardCompatibility, r.SideEffects, r.Verification, r.RollbackPlan = []string{" "}, nil, nil, " "
			r.Approved, r.Blockers = true, []string{"b"}
		}, "backward_compatibility requires at least one entry"},
		{"side effects", func(r *reviewcontract.CompatibilityReviewRequest) {
			r.SideEffects, r.Verification, r.RollbackPlan = nil, nil, " "
			r.Approved, r.Blockers = true, []string{"b"}
		}, "side_effects requires at least one entry"},
		{"verification", func(r *reviewcontract.CompatibilityReviewRequest) {
			r.Verification, r.RollbackPlan = nil, " "
			r.Approved, r.Blockers = true, []string{"b"}
		}, "verification requires at least one entry"},
		{"rollback", func(r *reviewcontract.CompatibilityReviewRequest) {
			r.RollbackPlan = " "
			r.Approved, r.Blockers = true, []string{"b"}
		}, "rollback_plan is required"},
		{"approved with blockers", func(r *reviewcontract.CompatibilityReviewRequest) { r.Approved, r.Blockers = true, []string{"b"} }, "approved compatibility review must not have blockers"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)
			if _, err := ValidateCompatibilityReview(req, "now"); err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	open := valid
	open.Blockers = []string{"needs migration"}
	review, err := ValidateCompatibilityReview(open, "now")
	if err != nil || review.Approved || len(review.Blockers) != 1 || review.ReviewedAt != "now" {
		t.Fatalf("unapproved review with blockers = %+v %v", review, err)
	}
}
