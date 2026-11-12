package issueopsretention

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func TestServicePrunesOnlyProvablyRetainedDoneCycles(t *testing.T) {
	now := time.Date(2026, 8, 11, 4, 0, 0, 0, time.UTC)
	old := now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano)
	recent := now.Format(time.RFC3339Nano)
	released := issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusReleased}
	active := issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusActive}
	repository := &fakeRepository{
		ids: []string{"old", "recent", "active", "unreflected", "reflected"},
		records: map[string]issueopscontract.Record{
			"old": {
				ID:        "old",
				Phase:     issueopscontract.PhaseDone,
				UpdatedAt: old,
				Execution: &issueopscontract.Execution{Lease: released},
			},
			"recent": {
				ID:        "recent",
				Phase:     issueopscontract.PhaseDone,
				UpdatedAt: recent,
				Execution: &issueopscontract.Execution{Lease: released},
			},
			"active": {
				ID:        "active",
				Phase:     issueopscontract.PhaseDone,
				UpdatedAt: old,
				Execution: &issueopscontract.Execution{Lease: active},
			},
			"unreflected": {
				ID:             "unreflected",
				Phase:          issueopscontract.PhaseDone,
				UpdatedAt:      old,
				RemoteArtifact: &issueopscontract.RemoteArtifactVerification{},
			},
			"reflected": {
				ID:               "reflected",
				Phase:            issueopscontract.PhaseDone,
				UpdatedAt:        old,
				RemoteArtifact:   &issueopscontract.RemoteArtifactVerification{},
				RemoteCompletion: &issueopscontract.RemoteCompletion{ReflectedAt: "2026-08-01T00:00:00Z"},
			},
		},
	}
	service := NewService(repository, fixedClock{now: now})

	preview, err := service.Prune(context.Background(), "/state", 30*24*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || len(preview.Pruned) != 2 || preview.Pruned[0] != "old" || preview.Pruned[1] != "reflected" {
		t.Fatalf("preview selected wrong records: %+v", preview)
	}
	if len(repository.deleted) != 0 {
		t.Fatalf("preview deleted records: %v", repository.deleted)
	}

	result, err := service.Prune(context.Background(), "/state", 30*24*time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.DryRun || len(repository.deleted) != 2 || repository.deleted[0] != "old" || repository.deleted[1] != "reflected" {
		t.Fatalf("confirmed prune deleted wrong records: result=%+v deleted=%v", result, repository.deleted)
	}
}

func TestServiceRejectsNonPositiveMaxAge(t *testing.T) {
	service := NewService(&fakeRepository{}, fixedClock{now: time.Now()})
	if _, err := service.Prune(context.Background(), "/state", 0, false); err == nil {
		t.Fatal("max age must be positive")
	}
}

func TestServiceReportsUnreadableRecordsAndContinues(t *testing.T) {
	now := time.Date(2026, 8, 11, 4, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		ids: []string{"broken", "old"},
		records: map[string]issueopscontract.Record{
			"old": {
				ID:        "old",
				Phase:     issueopscontract.PhaseDone,
				UpdatedAt: now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano),
				Execution: &issueopscontract.Execution{
					Lease: issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusReleased},
				},
			},
		},
		readErrors: map[string]error{"broken": errors.New("invalid state")},
	}
	service := NewService(repository, fixedClock{now: now})

	result, err := service.Prune(context.Background(), "/state", 30*24*time.Hour, false)

	if !errors.Is(err, errIncomplete) {
		t.Fatalf("error = %v, want errIncomplete", err)
	}
	if result.OK {
		t.Fatal("retention with unreadable records must not report OK")
	}
	if len(result.Unreadable) != 1 ||
		result.Unreadable[0] != "broken" ||
		result.ReadErrors != 1 ||
		len(result.UnreadableDiagnostics) != 1 ||
		result.UnreadableDiagnostics[0].Code != "read_failed" ||
		len(result.Pruned) != 1 ||
		result.Pruned[0] != "old" {
		t.Fatalf("retention result = %+v", result)
	}
}

func TestServiceBoundsUnreadableDiagnosticsWithoutStoppingPrune(t *testing.T) {
	now := time.Date(2026, 8, 11, 4, 0, 0, 0, time.UTC)
	ids := make([]string, 0, unreadableReportLimit+6)
	readErrors := make(map[string]error, unreadableReportLimit+5)
	for index := range unreadableReportLimit + 5 {
		id := fmt.Sprintf("broken-%02d", index)
		ids = append(ids, id)
		readErrors[id] = errors.New("invalid state")
	}
	ids = append(ids, "old")
	repository := &fakeRepository{
		ids: ids,
		records: map[string]issueopscontract.Record{
			"old": {
				ID:        "old",
				Phase:     issueopscontract.PhaseDone,
				UpdatedAt: now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano),
				Execution: &issueopscontract.Execution{Lease: issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusReleased}},
			},
		},
		readErrors: readErrors,
	}

	result, err := NewService(repository, fixedClock{now: now}).Prune(context.Background(), "/state", 30*24*time.Hour, false)
	if !errors.Is(err, errIncomplete) {
		t.Fatalf("error = %v, want errIncomplete", err)
	}
	if result.ReadErrors != unreadableReportLimit+5 ||
		len(result.Unreadable) != unreadableReportLimit ||
		len(result.UnreadableDiagnostics) != unreadableReportLimit ||
		len(result.Pruned) != 1 || result.Pruned[0] != "old" {
		t.Fatalf("bounded retention result = %+v", result)
	}
}

func TestServiceReturnsPartialReceiptWhenLaterDeleteFails(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	old := issueopscontract.Record{
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            "old",
		Phase:         issueopscontract.PhaseDone,
		UpdatedAt:     now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano),
	}
	later := old
	later.ID = "later"
	repository := &fakeRepository{
		ids:          []string{"old", "later"},
		records:      map[string]issueopscontract.Record{"old": old, "later": later},
		deleteErrors: map[string]error{"later": errors.New("compare-and-swap failed")},
	}

	result, err := NewService(repository, fixedClock{now: now}).Prune(
		context.Background(),
		"/state",
		30*24*time.Hour,
		true,
	)

	if !errors.Is(err, errIncomplete) {
		t.Fatalf("error = %v", err)
	}
	if result.OK ||
		result.Error != errIncomplete.Error() ||
		result.DeleteErrors != 1 ||
		len(result.Pruned) != 1 || result.Pruned[0] != "old" ||
		len(result.Failed) != 1 || result.Failed[0] != "later" ||
		len(result.DeleteDiagnostics) != 1 || result.DeleteDiagnostics[0].Code != "delete_failed" {
		t.Fatalf("partial retention result = %+v", result)
	}
	// Only the delete that succeeded left an outcome; the failed one left none.
	if _, ok := repository.outcomes["old"]; !ok || len(repository.outcomes) != 1 {
		t.Fatalf("outcomes = %v, want exactly the one for old", repository.outcomes)
	}
	if _, ok := repository.outcomes["later"]; ok {
		t.Fatalf("the failed delete of later left an outcome: %v", repository.outcomes)
	}
}

type fakeRepository struct {
	ids          []string
	records      map[string]issueopscontract.Record
	readErrors   map[string]error
	deleteErrors map[string]error
	deleted      []string
	outcomes     map[string]issueopscontract.CycleOutcome
}

func (repository *fakeRepository) ListIDs(context.Context, string) ([]string, error) {
	return repository.ids, nil
}

func (repository *fakeRepository) ReadUnchecked(_ context.Context, _ string, id string) (issueopscontract.Record, error) {
	if err := repository.readErrors[id]; err != nil {
		return issueopscontract.Record{}, err
	}
	return repository.records[id], nil
}

func (repository *fakeRepository) DeleteIfUnchanged(_ context.Context, _ string, id string, _ issueopscontract.Record, outcome issueopscontract.CycleOutcome) error {
	if err := repository.deleteErrors[id]; err != nil {
		return err
	}
	repository.deleted = append(repository.deleted, id)
	if repository.outcomes == nil {
		repository.outcomes = map[string]issueopscontract.CycleOutcome{}
	}
	repository.outcomes[id] = outcome
	return nil
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}

func TestServiceConfirmedPruneRetainsPrunedOutcome(t *testing.T) {
	now := time.Date(2026, 8, 11, 4, 0, 0, 123, time.FixedZone("KST", 9*60*60))
	old := now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano)
	released := issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusReleased}
	repository := &fakeRepository{
		ids: []string{"first", "second", "recent"},
		records: map[string]issueopscontract.Record{
			"first": {
				ID:            "first",
				Repo:          "/repo/first",
				Phase:         issueopscontract.PhaseDone,
				CreatedAt:     "2026-05-01T00:00:00Z",
				UpdatedAt:     old,
				Execution:     &issueopscontract.Execution{Lease: released},
				RegressEvents: []issueopscontract.RegressEvent{{Reason: "r", FromPhase: issueopscontract.PhaseDone, At: "2026-05-02T00:00:00Z"}},
			},
			"second": {
				ID:        "second",
				Repo:      "/repo/second",
				Phase:     issueopscontract.PhaseDone,
				CreatedAt: "2026-06-01T00:00:00Z",
				UpdatedAt: old,
				Execution: &issueopscontract.Execution{Lease: released},
				RegressEvents: []issueopscontract.RegressEvent{
					{Reason: "r", FromPhase: issueopscontract.PhaseDone, At: "2026-06-02T00:00:00Z"},
					{Reason: "r", FromPhase: issueopscontract.PhaseDone, At: "2026-06-03T00:00:00Z"},
				},
			},
			"recent": {
				ID:        "recent",
				Repo:      "/repo/recent",
				Phase:     issueopscontract.PhaseDone,
				CreatedAt: "2026-08-01T00:00:00Z",
				UpdatedAt: now.Format(time.RFC3339Nano),
				Execution: &issueopscontract.Execution{Lease: released},
			},
		},
	}
	targets := []string{"first", "second"}
	firstTimeline := issueopsdomain.BuildCycleTimeline(repository.records["first"])
	secondTimeline := issueopsdomain.BuildCycleTimeline(repository.records["second"])
	if reflect.DeepEqual(firstTimeline, secondTimeline) {
		t.Fatalf("both targets share the timeline %+v; the test cannot tell their outcomes apart", firstTimeline)
	}
	service := NewService(repository, fixedClock{now: now})

	preview, err := service.Prune(context.Background(), "/state", 30*24*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(preview.Pruned, targets) || len(repository.deleted) != 0 || len(repository.outcomes) != 0 {
		t.Fatalf("preview pruned=%v deleted=%v outcomes=%v, want %v and no writes", preview.Pruned, repository.deleted, repository.outcomes, targets)
	}

	result, err := service.Prune(context.Background(), "/state", 30*24*time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Pruned, targets) || !slices.Equal(repository.deleted, targets) {
		t.Fatalf("confirm pruned=%v deleted=%v, want %v", result.Pruned, repository.deleted, targets)
	}
	if len(repository.outcomes) != len(targets) {
		t.Fatalf("outcomes = %v, want exactly %v", repository.outcomes, targets)
	}
	for _, id := range targets {
		record := repository.records[id]
		outcome, ok := repository.outcomes[id]
		if !ok {
			t.Fatalf("no outcome for %s in %v", id, repository.outcomes)
		}
		if outcome.Outcome != "pruned" || outcome.ID != record.ID || outcome.Repo != record.Repo ||
			outcome.SchemaVersion != 1 || outcome.Metrics.Outcome != "pruned" ||
			outcome.RetainedAt != now.UTC().Format(time.RFC3339) {
			t.Fatalf("outcome for %s = %+v", id, outcome)
		}
		if want := issueopsdomain.BuildCycleTimeline(record); outcome.Timeline == nil || !reflect.DeepEqual(*outcome.Timeline, want) {
			t.Fatalf("timeline for %s = %+v, want %+v", id, outcome.Timeline, want)
		}
	}
	if _, ok := repository.outcomes["recent"]; ok {
		t.Fatalf("a record younger than the cutoff got an outcome: %v", repository.outcomes)
	}
}
