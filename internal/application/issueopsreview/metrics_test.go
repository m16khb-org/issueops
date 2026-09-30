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
		ReadRecord: func(root, id string) (contract.IssueOpsRecord, error) {
			events = append(events, "read:"+root+":"+id)
			if id == "corrupt" {
				return contract.IssueOpsRecord{}, errors.New("invalid state")
			}
			return contract.IssueOpsRecord{ID: id}, nil
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
	service := MetricsReader{Now: func() time.Time { calls++; return time.Time{} }, ReadRecord: func(root, id string) (contract.IssueOpsRecord, error) {
		calls++
		if root != "state" || id != "one" {
			t.Fatalf("lookup root=%s id=%s", root, id)
		}
		return contract.IssueOpsRecord{}, failure
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
