package issueops

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/port"
)

func TestNormalizeIssueOpsIDRejectsUnsafeIDs(t *testing.T) {
	for _, bad := range []string{"", "   ", "abc", "io-../x", "io-a/b", `io-a\b`, "../io-x"} {
		if got, err := normalizeIssueOpsID(bad); err == nil {
			t.Errorf("normalizeIssueOpsID(%q) = %q, want error", bad, got)
		}
	}
	got, err := normalizeIssueOpsID("  io-abc123 ")
	if err != nil || got != "io-abc123" {
		t.Fatalf("normalizeIssueOpsID trimmed = %q, %v", got, err)
	}
}

func TestEncodeIssueOpsRecordRejectsInvalidRecords(t *testing.T) {
	cases := map[string]issueops.IssueOpsRecord{
		"bad id":         {ID: "nope", SchemaVersion: issueops.IssueOpsSchemaVersion, OK: true},
		"wrong schema":   {ID: "io-abc", SchemaVersion: issueops.IssueOpsSchemaVersion + 1, OK: true},
		"fails validate": {ID: "io-abc", SchemaVersion: issueops.IssueOpsSchemaVersion},
	}
	for name, record := range cases {
		t.Run(name, func(t *testing.T) {
			got, data, err := encodeIssueOpsRecord(record)
			if err == nil || data != nil || got.OK {
				t.Fatalf("encode = ok:%v data:%d err:%v, want rejection", got.OK, len(data), err)
			}
		})
	}
}

func TestVisitIssueOpsExistingMissingStoreVisitsNothing(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	visited := 0
	if err := VisitIssueOpsExisting(root, func(string, issueops.IssueOpsRecord, error) { visited++ }); err != nil {
		t.Fatal(err)
	}
	if visited != 0 {
		t.Fatalf("visited %d rows in a missing store", visited)
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("visit created the store: %v", err)
	}
}

func TestVisitIssueOpsExistingReportsInvalidRows(t *testing.T) {
	root := t.TempDir()
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Apply(context.Background(), []port.RecordMutation{
		{Bucket: issueOpsBucket, ID: "io-garbage", Data: []byte("{not json"), RequireAbsent: true},
		{Bucket: issueOpsBucket, ID: "bad-id", Data: []byte("{}"), RequireAbsent: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]error{}
	if err := VisitIssueOpsExisting(root, func(id string, record issueops.IssueOpsRecord, err error) {
		if record.OK {
			t.Errorf("row %s reported OK", id)
		}
		got[id] = err
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("visited rows = %v", got)
	}
	if !errors.Is(got["io-garbage"], statecontract.ErrInvalidState) {
		t.Fatalf("garbage row err = %v, want ErrInvalidState", got["io-garbage"])
	}
	if got["bad-id"] == nil {
		t.Fatal("row with non-issueops id was not reported as an error")
	}
	records, err := ScanReadableIssueOps(root)
	if err != nil || len(records) != 0 {
		t.Fatalf("ScanReadableIssueOps = %v, %v; want invalid rows skipped", records, err)
	}
}

func TestCleanupRecordStoreLoadRejections(t *testing.T) {
	root := t.TempDir()
	store := CleanupRecordStore{StateRoot: root}
	ctx := context.Background()
	if _, err := store.Load(ctx, "bad"); err == nil {
		t.Fatal("Load accepted a non-issueops id")
	}
	if _, err := store.Load(ctx, "io-missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load missing store err = %v, want ErrNotExist", err)
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(ctx, []port.RecordMutation{{Bucket: issueOpsBucket, ID: "io-garbage", Data: []byte("x"), RequireAbsent: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(ctx, "io-missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load missing row err = %v, want ErrNotExist", err)
	}
	if _, err := store.Load(ctx, "io-garbage"); !errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("Load garbage err = %v, want ErrInvalidState", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.Load(cancelled, "io-garbage"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load cancelled err = %v, want Canceled", err)
	}
}

func TestMaterialFilesResolveAndRelative(t *testing.T) {
	var files MaterialFiles
	root := filepath.Join(string(filepath.Separator), "w", "root")
	if got := files.Resolve(root, "a/../b.md"); got != filepath.Join(root, "b.md") {
		t.Fatalf("relative Resolve = %q", got)
	}
	abs := filepath.Join(string(filepath.Separator), "elsewhere", "x.md")
	if got := files.Resolve(root, abs); got != abs {
		t.Fatalf("absolute Resolve = %q, want unchanged", got)
	}
	rel, err := files.Relative(root, filepath.Join(root, "docs", "x.md"))
	if err != nil || rel != "docs/x.md" {
		t.Fatalf("Relative = %q, %v", rel, err)
	}
	if rel, _ := files.Relative(root, filepath.Join(root, "..", "out.md")); rel != "../out.md" {
		t.Fatalf("Relative outside root = %q", rel)
	}
}

func TestExecutionSyncBaseMergeMsgConflictCount(t *testing.T) {
	root := t.TempDir()
	msgPath := filepath.Join(root, "MERGE_MSG")
	deps := func(code int, out string) issueops.ExecutionSyncBaseDeps {
		return issueops.ExecutionSyncBaseDeps{Git: func(context.Context, string, ...string) (int, string) { return code, out }}
	}
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(msgPath, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	write("Merge branch 'x'\n\n# Conflicts:\n#\ta.go\n#\tb.go\n\nother\n#\tnot-counted\n")
	if got := executionSyncBaseMergeMsgConflictCount(ctx, root, deps(0, msgPath+"\n")); got != 2 {
		t.Fatalf("commented block count = %d, want 2", got)
	}
	write("Merge\n\nConflicts:\n\ta.go\n\tb.go\n\tc.go\n")
	if got := executionSyncBaseMergeMsgConflictCount(ctx, root, deps(0, "MERGE_MSG")); got != 3 {
		t.Fatalf("relative git path, plain block count = %d, want 3", got)
	}
	write("Merge branch 'x'\n\t\tstray\n")
	if got := executionSyncBaseMergeMsgConflictCount(ctx, root, deps(0, msgPath)); got != 0 {
		t.Fatalf("no Conflicts block count = %d, want 0", got)
	}
	if got := executionSyncBaseMergeMsgConflictCount(ctx, root, deps(1, msgPath)); got != 0 {
		t.Fatalf("git failure count = %d, want 0", got)
	}
	if got := executionSyncBaseMergeMsgConflictCount(ctx, root, deps(0, "  \n")); got != 0 {
		t.Fatalf("empty git path count = %d, want 0", got)
	}
	if got := executionSyncBaseMergeMsgConflictCount(ctx, root, deps(0, filepath.Join(root, "absent"))); got != 0 {
		t.Fatalf("missing file count = %d, want 0", got)
	}
}
