package issueopsrecord

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestReadSelectedStrictlyReadsOnlySelectedRow(t *testing.T) {
	root := t.TempDir()
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := Encode(issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            "io-valid", Repo: "/repo", Phase: issueopscontract.IssueOpsPhaseProblem,
	})
	if err != nil {
		t.Fatal(err)
	}
	for id, data := range map[string][]byte{
		"io-valid":   valid,
		"io-corrupt": []byte(`{"id":"io-corrupt","schema_version":1,"unknown":true}`),
	} {
		if err := db.Put(bucket, id, data); err != nil {
			t.Fatal(err)
		}
	}
	for _, scenario := range []struct {
		id      string
		invalid bool
	}{
		{"io-valid", false}, {"io-corrupt", true}, {"io-missing", true}, {"not-an-id", true}, {"io-../bad", true},
	} {
		t.Run(scenario.id, func(t *testing.T) {
			record, err := (Store{}).ReadSelected(t.Context(), root, scenario.id)
			if err != nil || record.ID != scenario.id || record.Invalid != scenario.invalid {
				t.Fatalf("selected record = %+v, err = %v", record, err)
			}
		})
	}
}

func TestReadSelectedPreservesCancellationAndStoreErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (Store{}).ReadSelected(ctx, t.TempDir(), "io-selected"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	// A state root that is a regular file fails on Unix with ENOTDIR. Windows
	// reports the same path as not existing (ERROR_PATH_NOT_FOUND), which
	// ReadSelected correctly treats as an absent record.
	if runtime.GOOS != "windows" {
		root := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(root, []byte("file"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := (Store{}).ReadSelected(t.Context(), root, "io-selected"); err == nil {
			t.Fatal("a genuine store error must not become an invalid selection")
		}
	}
	// A data database path that is a directory exists but cannot be opened.
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "issueops.db"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{}).ReadSelected(t.Context(), root, "io-selected"); err == nil {
		t.Fatal("an unopenable data database must not become an invalid selection")
	}
}
