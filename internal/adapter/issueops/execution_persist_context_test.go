package issueops

import (
	"context"
	"errors"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
)

type spanPersist struct {
	name    string
	lock    func(context.Context, string, func(context.Context) error) error
	persist func(context.Context, issueops.IssueOpsRecord) error
}

func spanPersisters(stateRoot string) []spanPersist {
	replacement := ReplacementRecords{StateRoot: stateRoot}
	cycle := CycleRecordStore{StateRoot: stateRoot}
	return []spanPersist{
		{name: "replacement", lock: replacement.WithinLock, persist: func(ctx context.Context, record issueops.IssueOpsRecord) error {
			_, err := replacement.Persist(ctx, record, nil)
			return err
		}},
		{name: "cycle", lock: cycle.WithinLock, persist: func(ctx context.Context, record issueops.IssueOpsRecord) error {
			_, err := cycle.Save(ctx, record)
			return err
		}},
	}
}

func TestSpanPersistRefusesCommitAfterRequestCancellation(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "901-persist-precommit")
	for _, store := range spanPersisters(stateRoot) {
		t.Run(store.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			changed := fixture.record
			changed.UpdatedAt = "2026-10-02T00:00:00.000000001Z"
			err := store.lock(ctx, changed.ID, func(spanCtx context.Context) error {
				cancel()
				return store.persist(spanCtx, changed)
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("persist after cancellation err = %v, want context.Canceled", err)
			}
			stored, err := ReadIssueOps(stateRoot, changed.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.UpdatedAt == changed.UpdatedAt {
				t.Fatal("cancelled request still committed its record")
			}
		})
	}
}

func TestSpanPersistKeepsCommittedWriteWhenCancelledAfterCommit(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "902-persist-postcommit")
	for index, store := range spanPersisters(stateRoot) {
		t.Run(store.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			var observed sqlstore.SpanObservation
			ctx = sqlstore.WithSpanObserver(ctx, func(observation sqlstore.SpanObservation) { observed = observation })
			current, err := ReadIssueOps(stateRoot, fixture.record.ID)
			if err != nil {
				t.Fatal(err)
			}
			changed := current
			changed.UpdatedAt = []string{"2026-10-02T00:00:01Z", "2026-10-02T00:00:02Z"}[index]
			if err := store.lock(ctx, changed.ID, func(spanCtx context.Context) error {
				if err := store.persist(spanCtx, changed); err != nil {
					return err
				}
				cancel()
				return nil
			}); err != nil {
				t.Fatalf("span with post-commit cancellation: %v", err)
			}
			stored, err := ReadIssueOps(stateRoot, changed.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.UpdatedAt != changed.UpdatedAt {
				t.Fatalf("post-commit cancellation erased the write: stored=%s", stored.UpdatedAt)
			}
			if observed.CommitCount != 1 || observed.CommitCoverage != sqlstore.SpanCommitCoverageComplete {
				t.Fatalf("commit was not attributed to the request span: %+v", observed)
			}
		})
	}
}
