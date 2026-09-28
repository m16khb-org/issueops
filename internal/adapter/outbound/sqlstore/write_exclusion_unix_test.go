//go:build unix

package sqlstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/adapter/outbound/processlease"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/port"
)

func TestWriteExclusionBlocksEveryRecordMutationAndPreservesBytes(t *testing.T) {
	ctx := context.Background()
	original := []byte(`{"value":"original"}`)
	cases := []struct {
		name string
		run  func(*DB) error
	}{
		{"put", func(d *DB) error { return d.Put("records", "new", []byte("new")) }},
		{"delete", func(d *DB) error { return d.Delete("records", "owner") }},
		{"delete bucket", func(d *DB) error { return d.DeleteBucket("records") }},
		{"apply", func(d *DB) error {
			return d.Apply(ctx, []port.RecordMutation{{Bucket: "records", ID: "new", Data: []byte("new"), RequireAbsent: true}})
		}},
		{"compare apply", func(d *DB) error {
			return d.CompareAndApply(ctx, []port.ExpectedRecord{{Bucket: "records", ID: "owner", Data: original}}, []port.RecordMutation{{Bucket: "records", ID: "owner", Delete: true}})
		}},
		{"compare build", func(d *DB) error {
			return d.CompareAndApplyFunc(ctx, []port.ExpectedRecord{{Bucket: "records", ID: "owner", Data: original}}, func() ([]port.RecordMutation, error) {
				return []port.RecordMutation{{Bucket: "records", ID: "owner", Delete: true}}, nil
			})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			database, err := Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := database.Put("records", "owner", original); err != nil {
				t.Fatal(err)
			}
			exclusion, err := database.ExcludeWrites(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer exclusion.Close()
			if other, err := database.ExcludeWrites(ctx); !errors.Is(err, processlease.ErrBusy) {
				if other != nil {
					other.Close()
				}
				t.Fatalf("competing exclusion acquired: %v", err)
			}
			if err := tc.run(database); !errors.Is(err, processlease.ErrBusy) {
				t.Errorf("mutation was not excluded: %v", err)
			}
			rows, err := database.GetAll("records")
			if err != nil || len(rows) != 1 || rows[0].ID != "owner" || string(rows[0].Data) != string(original) {
				t.Errorf("rows changed: %+v err=%v", rows, err)
			}
			if err := exclusion.Close(); err != nil {
				t.Fatal(err)
			}
			if err := database.Put("records", "new", []byte("new")); err != nil {
				t.Fatalf("writer did not recover: %v", err)
			}
		})
	}
}

func TestWriteExclusionCoversDistinctDatabaseHandles(t *testing.T) {
	root := t.TempDir()
	first, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newDB(root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.data.Close()
	defer second.span.Close()
	guard, err := first.ExcludeWrites(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	if err := second.Put("b", "x", []byte("v")); !errors.Is(err, processlease.ErrBusy) {
		t.Fatalf("second handle bypassed exclusion: %v", err)
	}
}

func TestWriteExclusionDoesNotWaitForSpanHeldByARefusedWriter(t *testing.T) {
	database, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = database.WithSpan(context.Background(), func(ctx context.Context) error {
		guard, err := database.ExcludeWrites(ctx)
		if err != nil {
			return err
		}
		defer guard.Close()
		if err := database.Put("b", "owner", []byte("v")); !errors.Is(err, processlease.ErrBusy) {
			t.Fatalf("span writer was not refused: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put("b", "owner", []byte("v")); err != nil {
		t.Fatalf("span or exclusion leaked: %v", err)
	}
}

func TestWriteLeaseFilenameMatchesStateDoctorContract(t *testing.T) {
	root := t.TempDir()
	database, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := database.ExcludeWrites(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	info, err := os.Stat(filepath.Join(root, statecontract.RecordWriteLeaseFile))
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		t.Fatalf("canonical record lock file missing or unsafe: %v %v", info, err)
	}
}
