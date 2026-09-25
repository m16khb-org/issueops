package intentdesign

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestRecordIntentWritesCleanRedactedContract(t *testing.T) {
	written := false
	store := Store{
		Read: func(stateRoot, id string) (model.IssueOpsRecord, error) {
			if stateRoot != "state" || id != "io-1" {
				t.Fatalf("Read(%q, %q)", stateRoot, id)
			}
			return model.IssueOpsRecord{OK: true, ID: id}, nil
		},
		TouchWrite: func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			written = true
			if stateRoot != "state" {
				t.Fatalf("TouchWrite stateRoot = %q", stateRoot)
			}
			if record.Intent == nil {
				t.Fatal("intent was not recorded")
			}
			if got := record.Intent.SuccessCriteria; !stringSlicesEqual(got, []string{"tests pass", "docs updated"}) {
				t.Fatalf("success criteria = %#v", got)
			}
			if got := record.Intent.Constraints; !stringSlicesEqual(got, []string{"no schema changes"}) {
				t.Fatalf("constraints = %#v", got)
			}
			return record, nil
		},
	}
	record, err := RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{
		RawRequest:        "fix flaky quality gate in cmd/issueops",
		InterpretedIntent: "stabilize quality gate coverage workflow for harness commands",
		SuccessCriteria:   []string{" tests pass ", "", "docs updated", "tests pass"},
		Constraints:       []string{"no schema changes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !written || record.Intent == nil {
		t.Fatalf("record = %#v written=%v", record, written)
	}
}

func TestRecordIntentRejectsWeakContractsBeforeRead(t *testing.T) {
	store := Store{Read: func(string, string) (model.IssueOpsRecord, error) {
		t.Fatal("invalid intent should fail before store read")
		return model.IssueOpsRecord{}, nil
	}}
	cases := []model.IssueOpsIntentRecordRequest{
		{InterpretedIntent: "intent", SuccessCriteria: []string{"pass"}},
		{RawRequest: "raw", SuccessCriteria: []string{"pass"}},
		{RawRequest: "same request", InterpretedIntent: "same request", SuccessCriteria: []string{"pass"}},
		{RawRequest: "add login feature for all users", InterpretedIntent: "add login feature for all users", SuccessCriteria: []string{"pass"}},
		{RawRequest: "raw request with enough detail", InterpretedIntent: "different interpreted outcome"},
	}
	for _, req := range cases {
		if _, err := RecordIntent(store, "state", "io-1", req); err == nil {
			t.Fatalf("RecordIntent(%#v) succeeded", req)
		}
	}
}

func TestRecordDesignReviewWritesApprovedReview(t *testing.T) {
	store := Store{
		Read: func(stateRoot, id string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{OK: true, ID: id, Intent: &model.IssueOpsIntentContract{RawRequest: "raw"}}, nil
		},
		PlanReadiness: func(record model.IssueOpsRecord) model.IssueOpsReadiness {
			if record.Intent == nil {
				return model.IssueOpsReadiness{OK: true, Ready: false, Missing: []string{"intent"}}
			}
			return model.IssueOpsReadiness{OK: true, Ready: true}
		},
		TouchWrite: func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			if record.DesignReview == nil {
				t.Fatal("design review was not recorded")
			}
			if !record.DesignReview.Approved {
				t.Fatal("design review should be approved")
			}
			if got := record.DesignReview.Verification; !stringSlicesEqual(got, []string{"go test", DesignReviewEvidenceExample}) {
				t.Fatalf("verification = %#v", got)
			}
			return record, nil
		},
	}
	record, err := RecordDesignReview(store, "state", "io-1", model.IssueOpsDesignReviewRequest{
		ProblemSummary: "quality signal is low",
		ProposedDesign: "add focused tests",
		RefactorPlan:   "keep production unchanged",
		Alternatives:   []string{"raise threshold", "ignore package"},
		Risks:          []string{"brittle tests"},
		Verification:   []string{"go test", DesignReviewEvidenceExample},
		Approved:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.DesignReview == nil {
		t.Fatalf("record = %#v", record)
	}
}

func TestRecordDesignReviewRejectsIncompleteApprovedReview(t *testing.T) {
	store := Store{
		Read: func(stateRoot, id string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{OK: true, ID: id, Intent: &model.IssueOpsIntentContract{RawRequest: "raw"}}, nil
		},
		PlanReadiness: func(record model.IssueOpsRecord) model.IssueOpsReadiness {
			return model.IssueOpsReadiness{OK: true, Ready: true}
		},
		TouchWrite: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			t.Fatal("invalid design review should fail before write")
			return model.IssueOpsRecord{}, nil
		},
	}
	req := model.IssueOpsDesignReviewRequest{
		ProblemSummary: "quality signal is low",
		ProposedDesign: "add focused tests",
		RefactorPlan:   "keep production unchanged",
		Alternatives:   []string{"raise threshold"},
		Risks:          []string{"brittle tests"},
		Verification:   []string{"go test"},
		Approved:       true,
	}
	if _, err := RecordDesignReview(store, "state", "io-1", req); err == nil {
		t.Fatal("approved design review without design review evidence should fail")
	}
	req.Verification = []string{DesignReviewEvidenceExample}
	req.OpenQuestions = []string{"what next"}
	if _, err := RecordDesignReview(store, "state", "io-1", req); err == nil {
		t.Fatal("approved design review with open questions should fail")
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
