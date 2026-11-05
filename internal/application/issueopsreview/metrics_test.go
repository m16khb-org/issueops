package issueopsreview

import (
	"errors"
	contract "issueops/internal/contract/issueops"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMetricsReaderKeepsUnreadablePopulationAndReadOrder(t *testing.T) {
	var events []string
	service := MetricsReader{
		Now: func() time.Time {
			events = append(events, "clock")
			return time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("KST", 9*3600))
		},
		ListCycleIDs: func(root, repo string) ([]string, []string, error) {
			events = append(events, "list:"+root+":"+repo)
			return []string{"ok", "corrupt", "ok"}, []string{"unlisted"}, nil
		},
		ReadRecord: func(root, id string) (contract.Record, error) {
			events = append(events, "read:"+root+":"+id)
			if id == "corrupt" {
				return contract.Record{}, errors.New("invalid state")
			}
			return contract.Record{ID: id}, nil
		},
	}
	got, err := service.Read("state", "", " /repo ")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"clock", "list:state:/repo", "read:state:ok", "read:state:corrupt", "read:state:ok"}
	if !reflect.DeepEqual(events, want) || !got.OK || len(got.Cycles) != 2 || got.ReadErrors != 2 || got.GeneratedAt != "2026-01-01T18:04:05Z" {
		t.Fatalf("got=%+v events=%v", got, events)
	}
	if len(got.Warnings) < 2 || !strings.Contains(got.Warnings[0], "unlisted: unreadable record") {
		t.Fatalf("lost unreadable cycle: %v", got.Warnings)
	}
	found := false
	for _, w := range got.Warnings {
		if strings.Contains(w, "corrupt: unreadable record, excluded from the aggregate (invalid state)") {
			found = true
		}
	}
	if !found {
		t.Fatalf("lost read error: %v", got.Warnings)
	}
}

func TestMetricsReaderFailsSingleLookupAndDoesNotObserveInvalidScope(t *testing.T) {
	failure := errors.New("read failed")
	calls := 0
	service := MetricsReader{Now: func() time.Time { calls++; return time.Time{} }, ReadRecord: func(root, id string) (contract.Record, error) {
		calls++
		if root != "state" || id != "one" {
			t.Fatalf("lookup root=%s id=%s", root, id)
		}
		return contract.Record{}, failure
	}}
	for _, scope := range [][2]string{{"", ""}, {" id ", " /repo "}} {
		got, err := service.Read("state", scope[0], scope[1])
		if err == nil || err.Error() != "review metrics requires exactly one of --id or --repo" || got.OK || calls != 0 {
			t.Fatalf("invalid scope observed: %+v %v calls=%d", got, err, calls)
		}
	}
	got, err := service.Read("state", " one ", "")
	if !errors.Is(err, failure) || got.OK || calls != 2 || len(got.Cycles) != 0 || got.GeneratedAt != "" {
		t.Fatalf("single failure=%+v %v calls=%d", got, err, calls)
	}
	got, err = service.Read("state", "", "/repo")
	if err == nil || err.Error() != "review metrics repo listing is unavailable" || got.OK || calls != 3 {
		t.Fatalf("missing lister=%+v %v calls=%d", got, err, calls)
	}
	service.ListCycleIDs = func(string, string) ([]string, []string, error) { return nil, nil, failure }
	got, err = service.Read("state", "", "/repo")
	if !errors.Is(err, failure) || got.OK || calls != 4 {
		t.Fatalf("listing failure=%+v %v calls=%d", got, err, calls)
	}
}

func TestMetricsReaderAddsRetainedOutcomesWithoutDoubleCountingLiveRecords(t *testing.T) {
	retained := func(id, outcome string, verdicts ...string) contract.CycleOutcome {
		return contract.CycleOutcome{ID: id, Outcome: outcome, Metrics: contract.ReviewMetricsCycle{
			ID: id, Outcome: outcome, DevilsAdvocateRounds: len(verdicts), DevilsAdvocateVerdicts: verdicts,
		}}
	}
	service := MetricsReader{
		Now:          time.Now,
		ListCycleIDs: func(string, string) ([]string, []string, error) { return []string{"live"}, nil, nil },
		ReadRecord: func(_, id string) (contract.Record, error) {
			if id != "live" {
				return contract.Record{}, errors.New("issueops record not found")
			}
			return contract.Record{ID: id}, nil
		},
		ListOutcomes: func(_, repo string) ([]contract.CycleOutcome, []string, error) {
			// "live" also has a stale retained row; the live record must win.
			return []contract.CycleOutcome{
				retained("live", contract.CycleOutcomeFinished, "pass"),
				retained("gone", contract.CycleOutcomeFinished, "revise", "pass"),
				retained("dropped", contract.CycleOutcomeAbandoned, "stop"),
			}, []string{"garbled"}, nil
		},
	}
	got, err := service.Read("state", "", "/repo")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, cycle := range got.Cycles {
		ids = append(ids, cycle.ID+":"+cycle.Outcome)
	}
	if !reflect.DeepEqual(ids, []string{"live:", "gone:finished", "dropped:abandoned"}) || got.ReadErrors != 1 {
		t.Fatalf("cycles=%v read_errors=%d", ids, got.ReadErrors)
	}
	// The retained cycles are the only reviewed ones: rounds 2 and 1 over three
	// verdicts (revise, pass, stop); the stale "live" row adds nothing.
	if got.Aggregate.Cycles != 3 || got.Aggregate.ReviewedCycles != 2 || got.Aggregate.MeanRounds != 1.5 || got.Aggregate.StopRatio != 1.0/3 {
		t.Fatalf("aggregate=%+v", got.Aggregate)
	}

	single, err := service.Read("state", "gone", "")
	if err != nil || len(single.Cycles) != 1 || single.Cycles[0].Outcome != contract.CycleOutcomeFinished {
		t.Fatalf("retained single lookup=%+v err=%v", single, err)
	}
	if _, err := service.Read("state", "never", ""); err == nil || err.Error() != "issueops record not found" {
		t.Fatalf("unknown id must keep the record error: %v", err)
	}
}

func TestMetricsReaderFailsRepoQueryWhenRetainedOutcomesCannotBeListed(t *testing.T) {
	listErr := errors.New("outcome store unavailable")
	service := MetricsReader{
		Now:          time.Now,
		ListCycleIDs: func(string, string) ([]string, []string, error) { return nil, nil, nil },
		ReadRecord: func(string, string) (contract.Record, error) {
			t.Fatal("no cycle to read")
			return contract.Record{}, nil
		},
		ListOutcomes: func(string, string) ([]contract.CycleOutcome, []string, error) { return nil, nil, listErr },
	}
	got, err := service.Read("state", "", "/repo")
	if !errors.Is(err, listErr) || got.OK {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
