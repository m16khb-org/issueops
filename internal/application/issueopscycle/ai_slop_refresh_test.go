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
		Readiness: func(model.Record) model.Readiness {
			events = append(events, "readiness")
			return model.Readiness{Missing: []string{"implementation_changes"}}
		},
		Now:         func() string { events = append(events, "clock"); return "now" },
		Head:        func(model.Record) string { events = append(events, "head"); return "head" },
		Fingerprint: func(model.Record) string { events = append(events, "fingerprint"); return "fingerprint" },
		TouchWrite: func(_ string, rec model.Record) (model.Record, error) {
			events = append(events, "write")
			return rec, nil
		},
	}
	_, err := RefreshAISlopClean(store, "state", model.Record{ID: "io-1"})
	if err == nil || !strings.Contains(err.Error(), "implementation_changes") || !reflect.DeepEqual(events, []string{"readiness"}) {
		t.Fatalf("err=%v events=%v", err, events)
	}
}

func TestRefreshAISlopCleanObservesThenWrites(t *testing.T) {
	var events []string
	store := cycleport.AISlopCleanRefreshStore{
		Readiness: func(model.Record) model.Readiness {
			events = append(events, "readiness")
			return model.Readiness{Ready: true}
		},
		Now:         func() string { events = append(events, "clock"); return "2026-09-28T00:00:00Z" },
		Head:        func(model.Record) string { events = append(events, "head"); return "head-1" },
		Fingerprint: func(model.Record) string { events = append(events, "fingerprint"); return "fingerprint-1" },
		TouchWrite: func(_ string, rec model.Record) (model.Record, error) {
			events = append(events, "write")
			return rec, nil
		},
	}
	record, err := RefreshAISlopClean(store, "state", model.Record{ID: "io-1", Phase: model.PhaseFeedback})
	if err != nil || !reflect.DeepEqual(events, []string{"readiness", "clock", "head", "fingerprint", "write"}) ||
		record.Phase != model.PhaseFeedback || record.AISlopCleanAt != "2026-09-28T00:00:00Z" ||
		record.AISlopCleanHead != "head-1" || record.AISlopCleanFingerprint != "fingerprint-1" {
		t.Fatalf("record=%+v err=%v events=%v", record, err, events)
	}
}
