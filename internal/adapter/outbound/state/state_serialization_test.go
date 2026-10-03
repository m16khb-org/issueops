package state

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	stateapp "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/domain/statepath"
	stateport "issueops/internal/port/state"

	"modernc.org/sqlite"
)

func TestStateDeletionHoldsWriterSpan(t *testing.T) {
	for _, action := range []string{"delete", "prune", "dry-prune"} {
		t.Run(action, func(t *testing.T) {
			dir := t.TempDir()
			t.Cleanup(func() {
				if err := sqlstore.CloseRoot(dir); err != nil {
					t.Error(err)
				}
			})
			store, err := openStateStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			competitor, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "issueops.lock.db")+"?_pragma=busy_timeout(0)&_txlock=immediate")
			if err != nil {
				t.Fatal(err)
			}
			defer competitor.Close()
			probes := 0
			probe := func() {
				probes++
				tx, err := competitor.BeginTx(t.Context(), nil)
				if err == nil {
					if err := tx.Rollback(); err != nil {
						t.Fatal(err)
					}
				}
				var contention *sqlite.Error
				held := errors.As(err, &contention) && (contention.Code()&0xff == 5 || contention.Code()&0xff == 6)
				if err != nil && !held {
					t.Fatalf("unexpected contender error: %v", err)
				}
				if held != (action != "dry-prune") {
					t.Fatalf("%s writer lock held=%v", action, held)
				}
			}
			observed := &observedStateStore{Store: store}
			now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
			service := stateapp.NewService(stateapp.Dependencies{
				StateDir: func() string { return dir }, StatePath: statepath.Path,
				OpenStore:       func(string) (stateport.Store, error) { return observed, nil },
				ExistingRecords: ExistingRecords{}, Now: func() time.Time { return now },
			})
			record := statecontract.RecordEnvelope{
				SchemaVersion: statecontract.SchemaVersion, Key: "old", Content: "x", Bytes: 1,
				UpdatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339Nano),
			}
			if _, err := service.WriteRecord(t.Context(), dir, "old", record); err != nil {
				t.Fatal(err)
			}
			observed.onList, observed.onDelete = probe, probe

			if action == "delete" {
				err = service.Delete(t.Context(), "old")
			} else {
				_, err = service.Prune(t.Context(), time.Hour, action == "prune")
			}
			if err != nil || probes == 0 {
				t.Fatalf("err=%v probes=%d", err, probes)
			}
			observed.onList, observed.onDelete = nil, nil
			if action == "dry-prune" {
				if _, err := service.Read("old"); err != nil {
					t.Fatalf("dry-run removed record: %v", err)
				}
			} else if _, err := service.Read("old"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("record survived deletion: %v", err)
			}
			if _, err := service.Write(t.Context(), "old", "fresh"); err != nil {
				t.Fatalf("writer could not recreate after span release: %v", err)
			}
		})
	}
}

type observedStateStore struct {
	stateport.Store
	onList   func()
	onDelete func()
}

func (s *observedStateStore) List(bucket string) ([]string, error) {
	if s.onList != nil {
		s.onList()
	}
	return s.Store.List(bucket)
}

func (s *observedStateStore) Mutate(ctx context.Context, mutations []stateport.Mutation) error {
	for _, mutation := range mutations {
		if mutation.Delete && s.onDelete != nil {
			s.onDelete()
		}
	}
	return s.Store.Mutate(ctx, mutations)
}

func TestCanceledStateWriteDoesNotCreateRoot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := StateWrite(ctx, "key", "body"); !errors.Is(err, context.Canceled) {
		t.Fatalf("write error=%v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("canceled request created storage: %v", err)
	}
}
