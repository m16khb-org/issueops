package issueops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
)

func TestReadCycleOutcome(t *testing.T) {
	t.Run("missing store answers false without creating state", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "state")
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
		outcome, found, err := ReadCycleOutcome(root, "io-gone")
		if err != nil || found || !reflect.DeepEqual(outcome, model.CycleOutcome{}) {
			t.Fatalf("outcome=%+v found=%v err=%v", outcome, found, err)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatalf("a read created state files: %v err=%v", entries, err)
		}
	})

	root := t.TempDir()
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	timeline := model.CycleTimeline{
		EventsTotal: 1, EventsShown: 1,
		Events: []model.TimelineEvent{{At: "2026-09-08T00:00:00Z", Kind: model.TimelineKindRegressed, Phase: "plan"}},
	}
	want := model.CycleOutcome{
		SchemaVersion: 1, ID: "io-ok", Repo: "/repo", Outcome: "abandoned", RetainedAt: "2026-09-09T00:00:00Z",
		Metrics:  model.ReviewMetricsCycle{ID: "io-ok", RegressCount: 1},
		Timeline: &timeline,
	}
	put := func(id string, value any) {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Put(cycleOutcomeBucket, id, data); err != nil {
			t.Fatal(err)
		}
	}
	put("io-ok", want)
	if err := db.Put(cycleOutcomeBucket, "io-broken", []byte(`{"schema_version":1,`)); err != nil {
		t.Fatal(err)
	}
	put("io-schema2", model.CycleOutcome{SchemaVersion: 2, ID: "io-schema2"})
	put("io-mismatch", model.CycleOutcome{SchemaVersion: 1, ID: "io-other"})

	t.Run("missing row answers false", func(t *testing.T) {
		outcome, found, err := ReadCycleOutcome(root, "io-absent")
		if err != nil || found || !reflect.DeepEqual(outcome, model.CycleOutcome{}) {
			t.Fatalf("outcome=%+v found=%v err=%v", outcome, found, err)
		}
	})
	t.Run("healthy row round-trips", func(t *testing.T) {
		got, found, err := ReadCycleOutcome(root, "io-ok")
		if err != nil || !found || !reflect.DeepEqual(got, want) {
			t.Fatalf("got=%+v found=%v err=%v, want %+v", got, found, err, want)
		}
	})
	for _, id := range []string{"io-broken", "io-schema2", "io-mismatch"} {
		t.Run("unreadable row "+id+" is an error", func(t *testing.T) {
			got, found, err := ReadCycleOutcome(root, id)
			if err == nil || found || !reflect.DeepEqual(got, model.CycleOutcome{}) {
				t.Fatalf("got=%+v found=%v err=%v, want an error", got, found, err)
			}
		})
	}
}
