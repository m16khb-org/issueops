package issueops

import (
	"context"
	"reflect"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

// 타임라인이 생기기 전에 쓴 v1 row도 읽혀야 하고, finish 삭제 경로가 남기는 새 row는
// 타임라인을 들고 있어야 한다. schema_version은 그대로 1이다.
func TestListCycleOutcomesReadsRowsWithoutTimeline(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	legacy := `{"schema_version":1,"id":"io-legacy","repo":"/repo","outcome":"finished","retained_at":"2026-09-29T00:00:02Z","metrics":{"id":"io-legacy","regress_count":0,"devils_advocate_rounds":0}}`
	if err := db.Put(cycleOutcomeBucket, "io-legacy", []byte(legacy)); err != nil {
		t.Fatal(err)
	}

	outcomes, unreadable, err := ListCycleOutcomes(root)
	if err != nil || len(unreadable) != 0 || len(outcomes) != 1 {
		t.Fatalf("legacy row: outcomes=%+v unreadable=%v err=%v", outcomes, unreadable, err)
	}
	if outcomes[0].ID != "io-legacy" || outcomes[0].Timeline != nil {
		t.Fatalf("legacy row must read with a nil timeline: %+v", outcomes[0])
	}

	record, err := writeIssueOps(ctx, root, model.Record{
		SchemaVersion: 1, ID: "io-timeline-finish", Phase: model.PhaseDone,
		RegressEvents: []model.RegressEvent{{Reason: "stop", FromPhase: model.PhasePlan, At: "2026-09-29T00:00:00Z"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	store := CleanupRecordStore{StateRoot: root}
	snapshot, err := store.Load(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	armed, err := store.Arm(ctx, snapshot, finishRecordsAttempt("a"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, armed, finishOutcome(armed)); err != nil {
		t.Fatal(err)
	}

	outcomes, unreadable, err = ListCycleOutcomes(root)
	if err != nil || len(unreadable) != 0 || len(outcomes) != 2 {
		t.Fatalf("after delete: outcomes=%+v unreadable=%v err=%v", outcomes, unreadable, err)
	}
	var finished *model.CycleOutcome
	for i := range outcomes {
		if outcomes[i].ID == record.ID {
			finished = &outcomes[i]
		}
	}
	if finished == nil || finished.Timeline == nil {
		t.Fatalf("finish path retained no timeline: %+v", outcomes)
	}
	if want := domain.BuildCycleTimeline(armed.Record); !reflect.DeepEqual(*finished.Timeline, want) {
		t.Fatalf("retained timeline = %+v, want %+v", *finished.Timeline, want)
	}
	if finished.Timeline.EventsTotal == 0 {
		t.Fatalf("retained timeline has no events: %+v", *finished.Timeline)
	}
	if finished.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", finished.SchemaVersion)
	}
}
