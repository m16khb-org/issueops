package issueopsview

import (
	"reflect"
	"testing"

	issueopsviewcontract "issueops/internal/contract/issueopsview"
)

func TestRetainedTimeline(t *testing.T) {
	t.Run("nil timeline answers an empty timeline that names the gap", func(t *testing.T) {
		got := RetainedTimeline(issueopsviewcontract.CycleOutcome{ID: "io-old", Outcome: "finished"})
		want := issueopsviewcontract.CycleTimeline{
			Limits: []string{"retained_without_timeline"},
			Events: []issueopsviewcontract.TimelineEvent{},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("RetainedTimeline(nil) = %+v, want %+v", got, want)
		}
		if got.Events == nil {
			t.Fatal("events must be an empty slice, not nil, so JSON prints []")
		}
	})
	t.Run("retained timeline is returned unchanged", func(t *testing.T) {
		kept := issueopsviewcontract.CycleTimeline{
			EventsTotal: 2, EventsShown: 2, Undated: 1,
			Limits: []string{"lease_last_value_only"},
			Events: []issueopsviewcontract.TimelineEvent{
				{At: "2026-09-07T00:00:00Z", Kind: "cycle_started"},
				{At: "2026-09-08T00:00:00Z", Kind: "regressed", Phase: "plan"},
			},
		}
		got := RetainedTimeline(issueopsviewcontract.CycleOutcome{ID: "io-new", Timeline: &kept})
		if !reflect.DeepEqual(got, kept) {
			t.Fatalf("RetainedTimeline = %+v, want %+v", got, kept)
		}
	})
}
