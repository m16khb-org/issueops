package issueopsreview

import (
	"errors"
	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	port "issueops/internal/port/issueopsreview"
	"reflect"
	"testing"
)

func TestPlanningRecorderRejectsAuthorityBeforeRequestValidation(t *testing.T) {
	denied := errors.New("actor rejected")
	calls := map[string]func(PlanningRecorder) error{
		"intent": func(s PlanningRecorder) error {
			_, e := s.Intent("state", "cycle", model.IssueOpsIntentRecordRequest{})
			return e
		},
		"plan prep": func(s PlanningRecorder) error {
			_, e := s.PlanPrep("state", "cycle", model.IssueOpsPlanPrepRequest{})
			return e
		},
		"design": func(s PlanningRecorder) error {
			_, e := s.Design("state", "cycle", reviewcontract.DesignReviewRequest{})
			return e
		},
		"compatibility": func(s PlanningRecorder) error {
			_, e := s.Compatibility("state", "cycle", reviewcontract.CompatibilityReviewRequest{})
			return e
		},
		"devils advocate": func(s PlanningRecorder) error {
			_, e := s.DevilsAdvocate("state", "cycle", reviewcontract.DevilsAdvocateReviewRequest{})
			return e
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			var events []string
			s := PlanningRecorder{Store: port.ReviewMutationStore{
				WithLock: func(_, _ string, fn func() error) error {
					events = append(events, "lock")
					defer func() { events = append(events, "unlock") }()
					return fn()
				},
				Read: func(_, _ string) (model.IssueOpsRecord, error) {
					events = append(events, "read")
					return model.IssueOpsRecord{}, nil
				},
				ValidateMutation: func(model.IssueOpsRecord) error { events = append(events, "authority"); return denied },
			}}
			err := call(s)
			if !errors.Is(err, denied) || !reflect.DeepEqual(events, []string{"lock", "read", "authority", "unlock"}) {
				t.Fatalf("err=%v events=%v", err, events)
			}
		})
	}
}

func TestPlanningRecorderIntentLocksBothReadsAndPreservesWriteResult(t *testing.T) {
	var events []string
	locked := false
	writeErr := errors.New("persist failed")
	s := PlanningRecorder{Store: port.ReviewMutationStore{
		WithLock: func(_, _ string, fn func() error) error {
			locked = true
			events = append(events, "lock")
			defer func() { locked = false; events = append(events, "unlock") }()
			return fn()
		},
		Read: func(_, _ string) (model.IssueOpsRecord, error) {
			if !locked {
				t.Fatal("unlocked read")
			}
			events = append(events, "read")
			return model.IssueOpsRecord{ID: "cycle"}, nil
		},
		ValidateMutation: func(model.IssueOpsRecord) error { events = append(events, "authority"); return nil },
		Now:              func() string { events = append(events, "clock"); return "2026-09-30T00:00:00Z" },
		Write: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			if !locked || r.Intent == nil || r.UpdatedAt != r.Intent.RecordedAt {
				t.Fatalf("invalid write: %+v", r)
			}
			events = append(events, "write")
			return r, writeErr
		},
	}}
	result, err := s.Intent("state", "cycle", model.IssueOpsIntentRecordRequest{RawRequest: "Fix the login bug", InterpretedIntent: "Restore expired session refresh while preserving authenticated user identity", SuccessCriteria: []string{"expired session refresh succeeds"}, IntentClass: "standard"})
	if !errors.Is(err, writeErr) || result.ID != "cycle" || !reflect.DeepEqual(events, []string{"lock", "read", "authority", "read", "clock", "clock", "write", "unlock"}) {
		t.Fatalf("result=%+v err=%v events=%v", result, err, events)
	}
}

func TestPlanningRecorderRegressionReasonPrecedesLock(t *testing.T) {
	s := PlanningRecorder{}
	if _, err := s.Regress("state", "cycle", " "); err == nil {
		t.Fatal("empty reason accepted")
	}
}
