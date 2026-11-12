package issueopsreview

import (
	"encoding/json"
	"errors"
	"io/fs"
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

func TestMetricsReaderReportsRetention(t *testing.T) {
	row := func(id, outcome, retainedAt string) contract.CycleOutcome {
		return contract.CycleOutcome{ID: id, Outcome: outcome, RetainedAt: retainedAt, Metrics: contract.ReviewMetricsCycle{ID: id, Outcome: outcome}}
	}
	cases := []struct {
		name       string
		rows       []contract.CycleOutcome
		unreadable []string
		want       contract.ReviewMetricsRetention
		cycles     int
		wantRaw    string
	}{
		{name: "no rows", want: contract.ReviewMetricsRetention{}, cycles: 1, wantRaw: `"retention":{"finished":0,"abandoned":0,"pruned":0}`},
		{name: "one finished row", rows: []contract.CycleOutcome{row("a", contract.CycleOutcomeFinished, "2026-11-01T00:00:00Z")},
			want: contract.ReviewMetricsRetention{Finished: 1, LastRetainedAt: "2026-11-01T00:00:00Z"}, cycles: 2},
		{name: "last is the latest instant, not the latest string", rows: []contract.CycleOutcome{
			row("a", contract.CycleOutcomeFinished, "2026-11-01T18:00:00+09:00"),
			row("b", contract.CycleOutcomeAbandoned, "2026-11-01T10:00:00Z"),
		}, want: contract.ReviewMetricsRetention{Finished: 1, Abandoned: 1, LastRetainedAt: "2026-11-01T10:00:00Z"}, cycles: 3},
		{name: "an offset last is reported in utc", rows: []contract.CycleOutcome{
			row("a", contract.CycleOutcomeFinished, "2026-11-01T09:00:00.5+09:00"),
			row("b", contract.CycleOutcomeAbandoned, "2026-10-31T23:00:00Z"),
		}, want: contract.ReviewMetricsRetention{Finished: 1, Abandoned: 1, LastRetainedAt: "2026-11-01T00:00:00.5Z"}, cycles: 3},
		{name: "unparseable time counts but is not a last candidate", rows: []contract.CycleOutcome{
			row("a", contract.CycleOutcomeFinished, "not-a-time"),
			row("b", contract.CycleOutcomeAbandoned, "2026-11-01T10:00:00Z"),
			row("c", contract.CycleOutcomeFinished, ""),
		}, want: contract.ReviewMetricsRetention{Finished: 2, Abandoned: 1, LastRetainedAt: "2026-11-01T10:00:00Z"}, cycles: 4},
		{name: "a row whose cycle is still live is counted", rows: []contract.CycleOutcome{
			row("live", contract.CycleOutcomeFinished, "2026-11-02T00:00:00Z"),
		}, want: contract.ReviewMetricsRetention{Finished: 1, LastRetainedAt: "2026-11-02T00:00:00Z"}, cycles: 1},
		{name: "latest row first, earlier row after", rows: []contract.CycleOutcome{
			row("a", contract.CycleOutcomeFinished, "2026-11-03T00:00:00Z"),
			row("b", contract.CycleOutcomeAbandoned, "2026-11-01T00:00:00Z"),
		}, want: contract.ReviewMetricsRetention{Finished: 1, Abandoned: 1, LastRetainedAt: "2026-11-03T00:00:00Z"}, cycles: 3},
		{name: "a pruned row is counted and is a last candidate", rows: []contract.CycleOutcome{
			row("a", contract.CycleOutcomeFinished, "2026-11-01T00:00:00Z"),
			row("b", contract.CycleOutcomePruned, "2026-11-04T00:00:00Z"),
		}, want: contract.ReviewMetricsRetention{Finished: 1, Pruned: 1, LastRetainedAt: "2026-11-04T00:00:00Z"}, cycles: 3},
		{name: "unknown outcome is neither counted nor a last candidate", rows: []contract.CycleOutcome{
			row("a", contract.CycleOutcomeFinished, "2026-11-01T00:00:00Z"),
			row("b", "bogus", "2026-11-05T00:00:00Z"),
		}, want: contract.ReviewMetricsRetention{Finished: 1, LastRetainedAt: "2026-11-01T00:00:00Z"}, cycles: 3},
		{name: "an unreadable row leaves the counts unchanged", rows: []contract.CycleOutcome{
			row("a", contract.CycleOutcomeAbandoned, "2026-11-01T00:00:00Z"),
		}, unreadable: []string{"broken"}, want: contract.ReviewMetricsRetention{Abandoned: 1, LastRetainedAt: "2026-11-01T00:00:00Z"}, cycles: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			reader := MetricsReader{
				Now:          time.Now,
				ListCycleIDs: func(string, string) ([]string, []string, error) { return []string{"live"}, nil, nil },
				ReadRecord:   func(_, id string) (contract.Record, error) { return contract.Record{ID: id}, nil },
				ListOutcomes: func(_, repo string) ([]contract.CycleOutcome, []string, error) {
					calls++
					if repo != "/repo" {
						t.Errorf("ListOutcomes repo=%q", repo)
					}
					return tc.rows, tc.unreadable, nil
				},
			}
			got, err := reader.Read("state", "", "/repo")
			if err != nil {
				t.Fatal(err)
			}
			if got.ReadErrors != len(tc.unreadable) {
				t.Fatalf("read_errors=%d want %d", got.ReadErrors, len(tc.unreadable))
			}
			if got.Retention == nil || !reflect.DeepEqual(*got.Retention, tc.want) {
				t.Fatalf("retention=%+v want %+v", got.Retention, tc.want)
			}
			if calls != 1 {
				t.Fatalf("ListOutcomes called %d times, want 1", calls)
			}
			if len(got.Cycles) != tc.cycles {
				t.Fatalf("cycles=%d want %d (retention must not change the cycles list)", len(got.Cycles), tc.cycles)
			}
			if tc.wantRaw != "" {
				raw, err := json.Marshal(got)
				if err != nil || !strings.Contains(string(raw), tc.wantRaw) {
					t.Fatalf("json=%s err=%v want substring %s", raw, err, tc.wantRaw)
				}
			}
		})
	}

	t.Run("single id carries no retention", func(t *testing.T) {
		calls := 0
		reader := MetricsReader{
			Now:        time.Now,
			ReadRecord: func(_, id string) (contract.Record, error) { return contract.Record{ID: id}, nil },
			ListOutcomes: func(string, string) ([]contract.CycleOutcome, []string, error) {
				calls++
				return nil, nil, nil
			},
		}
		got, err := reader.Read("state", "one", "")
		if err != nil || got.Retention != nil || calls != 0 {
			t.Fatalf("got=%+v err=%v calls=%d", got.Retention, err, calls)
		}
	})
	t.Run("single id answered from a retained row carries no retention", func(t *testing.T) {
		reader := MetricsReader{
			Now: time.Now,
			ReadRecord: func(string, string) (contract.Record, error) {
				return contract.Record{}, fs.ErrNotExist
			},
			ListOutcomes: func(string, string) ([]contract.CycleOutcome, []string, error) {
				return []contract.CycleOutcome{row("gone", contract.CycleOutcomeAbandoned, "2026-11-01T00:00:00Z")}, nil, nil
			},
		}
		got, err := reader.Read("state", "gone", "")
		if err != nil || len(got.Cycles) != 1 || got.Cycles[0].ID != "gone" || got.Retention != nil {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
	t.Run("outcome listing failure fails the read", func(t *testing.T) {
		listErr := errors.New("outcome store unavailable")
		reader := MetricsReader{
			Now:          time.Now,
			ListCycleIDs: func(string, string) ([]string, []string, error) { return nil, nil, nil },
			ReadRecord:   func(string, string) (contract.Record, error) { return contract.Record{}, nil },
			ListOutcomes: func(string, string) ([]contract.CycleOutcome, []string, error) { return nil, nil, listErr },
		}
		got, err := reader.Read("state", "", "/repo")
		if !errors.Is(err, listErr) || got.OK || got.Retention != nil {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
	t.Run("no outcome lister carries no retention", func(t *testing.T) {
		reader := MetricsReader{
			Now:          time.Now,
			ListCycleIDs: func(string, string) ([]string, []string, error) { return []string{"live"}, nil, nil },
			ReadRecord:   func(_, id string) (contract.Record, error) { return contract.Record{ID: id}, nil },
		}
		got, err := reader.Read("state", "", "/repo")
		if err != nil || !got.OK || got.Retention != nil {
			t.Fatalf("got=%+v err=%v", got, err)
		}
		raw, _ := json.Marshal(got)
		if strings.Contains(string(raw), "retention") {
			t.Fatalf("nil retention must be omitted: %s", raw)
		}
	})
}
