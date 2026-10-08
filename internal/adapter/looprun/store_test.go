package looprun

import (
	loopruncontract "issueops/internal/contract/looprun"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/port"
)

func TestReadLoopRefusesNonCurrentSchema(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	loop := startLoopForTest(t, "non-current-schema", 2)
	db, err := sqlstore.Open(testLoopStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{`"schema_version":99,`, `"schema_version":0,`, ``} {
		if err := db.Put(loopBucket, loop.ID, []byte(`{"ok":true,`+schema+`"id":"`+loop.ID+`"}`+"\n")); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadLoop(loop.ID); err == nil || !strings.Contains(err.Error(), "unsupported loop schema_version") {
			t.Fatalf("ReadLoop(%s) err=%v, want schema refusal", schema, err)
		}
	}
}

func TestRepoGateSummaryDoesNotRepairExistingLoopStore(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := Start(loopruncontract.StartLoopRequest{Repo: repo, Name: "read-only", Goal: "prove diagnostic reads stay read-only"}); err != nil {
		t.Fatal(err)
	}

	paths := []string{
		testLoopStateRoot(),
		filepath.Join(testLoopStateRoot(), "issueops.db"),
		filepath.Join(testLoopStateRoot(), "issueops.lock.db"),
	}
	for index, path := range paths {
		mode := os.FileMode(0o644)
		if index == 0 {
			mode = 0o755
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}

	summary, warnings := RepoGateSummaryFor(repo)
	if summary.Active != 1 || len(warnings) != 0 {
		t.Fatalf("loop summary=%#v warnings=%#v", summary, warnings)
	}
	for index, path := range paths {
		want := os.FileMode(0o644)
		if index == 0 {
			want = 0o755
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("loop diagnostic repaired %s mode to %o, want unchanged %o", path, got, want)
		}
	}
}

func TestReadAllExistingDecodesEveryLoopInOneScan(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	good := startLoopForTest(t, "scan-good", 2)
	db, err := sqlstore.Open(testLoopStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(loopBucket, "loop-broken", []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	if err := db.Put(loopBucket, "not-a-loop", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	scans := 0
	store := testLoopStore()
	store.GetAllExisting = func(dir, bucket string) ([]port.RecordRow, error) {
		scans++
		return sqlstore.GetAllExisting(dir, bucket)
	}
	observations, err := store.ReadAllExisting()
	if err != nil || scans != 1 || len(observations) != 3 {
		t.Fatalf("observations=%+v scans=%d err=%v", observations, scans, err)
	}
	byID := map[string]error{}
	for _, observation := range observations {
		byID[observation.ID] = observation.Error
		if observation.ID == good.ID && (!observation.Loop.OK || observation.Loop.ID != good.ID) {
			t.Fatalf("good loop = %+v", observation.Loop)
		}
	}
	if byID[good.ID] != nil || byID["loop-broken"] == nil || byID["not-a-loop"] == nil {
		t.Fatalf("per-row errors = %v", byID)
	}
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	if observations, err := testLoopStore().ReadAllExisting(); err != nil || len(observations) != 0 {
		t.Fatalf("missing store observations=%v err=%v", observations, err)
	}
}
