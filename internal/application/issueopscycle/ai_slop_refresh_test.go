package issueopscycle

import (
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func TestRefreshAISlopCleanRejectsBeforeObservingOrWriting(t *testing.T) {
	var events []string
	store := cycleport.AISlopCleanRefreshStore{
		Readiness: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			events = append(events, "readiness")
			return model.IssueOpsReadiness{Missing: []string{"implementation_changes"}}
		},
		Now:         func() string { events = append(events, "clock"); return "now" },
		Head:        func(model.IssueOpsRecord) string { events = append(events, "head"); return "head" },
		Fingerprint: func(model.IssueOpsRecord) string { events = append(events, "fingerprint"); return "fingerprint" },
		TouchWrite: func(_ string, rec model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			events = append(events, "write")
			return rec, nil
		},
	}
	_, err := RefreshAISlopClean(store, "state", model.IssueOpsRecord{ID: "io-1"})
	if err == nil || !strings.Contains(err.Error(), "implementation_changes") || !reflect.DeepEqual(events, []string{"readiness"}) {
		t.Fatalf("err=%v events=%v", err, events)
	}
}

func TestRefreshAISlopCleanObservesThenWrites(t *testing.T) {
	var events []string
	store := cycleport.AISlopCleanRefreshStore{
		Readiness: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			events = append(events, "readiness")
			return model.IssueOpsReadiness{Ready: true}
		},
		Now:         func() string { events = append(events, "clock"); return "2026-09-28T00:00:00Z" },
		Head:        func(model.IssueOpsRecord) string { events = append(events, "head"); return "head-1" },
		Fingerprint: func(model.IssueOpsRecord) string { events = append(events, "fingerprint"); return "fingerprint-1" },
		TouchWrite: func(_ string, rec model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			events = append(events, "write")
			return rec, nil
		},
	}
	record, err := RefreshAISlopClean(store, "state", model.IssueOpsRecord{ID: "io-1", Phase: model.IssueOpsPhaseFeedback})
	if err != nil || !reflect.DeepEqual(events, []string{"readiness", "clock", "head", "fingerprint", "write"}) ||
		record.Phase != model.IssueOpsPhaseFeedback || record.AISlopCleanAt != "2026-09-28T00:00:00Z" ||
		record.AISlopCleanHead != "head-1" || record.AISlopCleanFingerprint != "fingerprint-1" {
		t.Fatalf("record=%+v err=%v events=%v", record, err, events)
	}
}
