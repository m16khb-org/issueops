package looprun

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	loopruncontract "issueops/internal/contract/looprun"
	"issueops/internal/port"
)

// fakeDB is an in-memory StateDatabase that records what the Store asks of it.
type fakeDB struct {
	rows      map[string][]byte
	getErr    error
	applyErr  error
	applied   []port.RecordMutation
	spanCalls int
}

func (db *fakeDB) Get(bucket, id string) ([]byte, bool, error) {
	if db.getErr != nil {
		return nil, false, db.getErr
	}
	data, ok := db.rows[bucket+"/"+id]
	return data, ok, nil
}

func (db *fakeDB) Apply(_ context.Context, mutations []port.RecordMutation) error {
	if db.applyErr != nil {
		return db.applyErr
	}
	db.applied = append(db.applied, mutations...)
	return nil
}

func (db *fakeDB) WithSpan(ctx context.Context, fn func(context.Context) error) error {
	db.spanCalls++
	return fn(ctx)
}

func storeOver(db *fakeDB, openErr error) Store {
	return Store{Directory: "dir", OpenDatabase: func(string) (StateDatabase, error) {
		if openErr != nil {
			return nil, openErr
		}
		return db, nil
	}}
}

func TestNormalizeLoopIDRejectsMalformedIDs(t *testing.T) {
	for _, tc := range []struct{ id, want string }{
		{"", "loop_id is required"},
		{"   ", "loop_id is required"},
		{"abc", "invalid loop id"},
		{"loop-../x", "invalid loop id"},
		{"loop-a/b", "invalid loop id"},
		{`loop-a\b`, "invalid loop id"},
	} {
		if _, err := normalizeLoopID(tc.id); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("normalizeLoopID(%q) err=%v, want %q", tc.id, err, tc.want)
		}
	}
	if id, err := normalizeLoopID("  loop-abc123 "); err != nil || id != "loop-abc123" {
		t.Fatalf("trimmed id = %q err=%v", id, err)
	}
}

func TestStoreReadReportsEachFailureDistinctly(t *testing.T) {
	good := `{"id":"loop-a","schema_version":1}`
	sentinel := errors.New("disk exploded")

	if _, err := storeOver(&fakeDB{}, nil).Read("bad"); err == nil || !strings.Contains(err.Error(), "invalid loop id") {
		t.Fatalf("invalid id err=%v", err)
	}
	if loop, err := storeOver(nil, sentinel).Read("loop-a"); !errors.Is(err, sentinel) || loop.OK || loop.ID != "loop-a" {
		t.Fatalf("open failure loop=%+v err=%v", loop, err)
	}
	if _, err := storeOver(&fakeDB{getErr: sentinel}, nil).Read("loop-a"); !errors.Is(err, sentinel) {
		t.Fatalf("get failure err=%v", err)
	}
	if loop, err := storeOver(&fakeDB{}, nil).Read("loop-a"); !errors.Is(err, fs.ErrNotExist) || loop.OK {
		t.Fatalf("absent loop=%+v err=%v", loop, err)
	}
	if _, err := storeOver(&fakeDB{rows: map[string][]byte{"loop/loop-a": []byte(`{"id":"loop-other","schema_version":1}`)}}, nil).Read("loop-a"); err == nil || !strings.Contains(err.Error(), `loop id mismatch: record has "loop-other"`) {
		t.Fatalf("id mismatch err=%v", err)
	}
	if _, err := storeOver(&fakeDB{rows: map[string][]byte{"loop/loop-a": []byte("{")}}, nil).Read("loop-a"); err == nil {
		t.Fatal("corrupt JSON must fail")
	}
	loop, err := storeOver(&fakeDB{rows: map[string][]byte{"loop/loop-a": []byte(good)}}, nil).Read("loop-a")
	if err != nil || !loop.OK || loop.ID != "loop-a" {
		t.Fatalf("good loop=%+v err=%v", loop, err)
	}
}

func TestStoreReadExistingUsesReadOnlyGetterAndNeverOpensDatabase(t *testing.T) {
	sentinel := errors.New("scan failed")
	var gotDir, gotBucket, gotID string
	store := Store{
		Directory: "state-dir",
		OpenDatabase: func(string) (StateDatabase, error) {
			t.Fatal("ReadExisting must not open (and thereby create/repair) the database")
			return nil, nil
		},
		GetExisting: func(dir, bucket, id string) ([]byte, bool, error) {
			gotDir, gotBucket, gotID = dir, bucket, id
			switch id {
			case "loop-missing":
				return nil, false, nil
			case "loop-err":
				return nil, false, sentinel
			case "loop-mismatch":
				return []byte(`{"id":"loop-x","schema_version":1}`), true, nil
			case "loop-oldschema":
				return []byte(`{"id":"loop-oldschema","schema_version":7}`), true, nil
			}
			return []byte(`{"id":"loop-ok","schema_version":1}`), true, nil
		},
	}
	loop, err := store.ReadExisting(" loop-ok ")
	if err != nil || !loop.OK || loop.ID != "loop-ok" || gotDir != "state-dir" || gotBucket != "loop" || gotID != "loop-ok" {
		t.Fatalf("loop=%+v err=%v dir=%q bucket=%q id=%q", loop, err, gotDir, gotBucket, gotID)
	}
	if _, err := store.ReadExisting(""); err == nil || !strings.Contains(err.Error(), "loop_id is required") {
		t.Fatalf("empty id err=%v", err)
	}
	if _, err := store.ReadExisting("loop-err"); !errors.Is(err, sentinel) {
		t.Fatalf("getter error err=%v", err)
	}
	if loop, err := store.ReadExisting("loop-missing"); !errors.Is(err, fs.ErrNotExist) || loop.OK || loop.ID != "loop-missing" {
		t.Fatalf("missing loop=%+v err=%v", loop, err)
	}
	if _, err := store.ReadExisting("loop-mismatch"); err == nil || !strings.Contains(err.Error(), "loop id mismatch") {
		t.Fatalf("mismatch err=%v", err)
	}
	if _, err := store.ReadExisting("loop-oldschema"); err == nil || !strings.Contains(err.Error(), "unsupported loop schema_version 7; current is 1") {
		t.Fatalf("schema err=%v", err)
	}
}

func TestStoreWriteValidatesBeforeTouchingTheDatabase(t *testing.T) {
	db := &fakeDB{}
	store := storeOver(db, nil)
	ctx := t.Context()

	if loop, err := store.Write(ctx, loopruncontract.LoopRun{ID: "nope", SchemaVersion: 1, OK: true}); err == nil || loop.OK || !strings.Contains(err.Error(), "invalid loop id") {
		t.Fatalf("bad id loop=%+v err=%v", loop, err)
	}
	if loop, err := store.Write(ctx, loopruncontract.LoopRun{ID: "loop-a", SchemaVersion: 2, OK: true}); err == nil || loop.OK || !strings.Contains(err.Error(), "unsupported loop schema_version 2") {
		t.Fatalf("bad schema loop=%+v err=%v", loop, err)
	}
	openErr := errors.New("cannot open")
	if loop, err := storeOver(db, openErr).Write(ctx, loopruncontract.LoopRun{ID: "loop-a", SchemaVersion: 1}); !errors.Is(err, openErr) || loop.OK {
		t.Fatalf("open failure loop=%+v err=%v", loop, err)
	}
	if len(db.applied) != 0 {
		t.Fatalf("rejected writes must not mutate state, applied=%+v", db.applied)
	}
}

func TestStoreWriteReportsApplyFailureAndStoresOKTrueRecord(t *testing.T) {
	applyErr := errors.New("commit failed")
	failing := storeOver(&fakeDB{applyErr: applyErr}, nil)
	if loop, err := failing.Write(t.Context(), loopruncontract.LoopRun{ID: "loop-a", SchemaVersion: 1}); !errors.Is(err, applyErr) || loop.OK {
		t.Fatalf("apply failure loop=%+v err=%v", loop, err)
	}

	db := &fakeDB{}
	written, err := storeOver(db, nil).Write(t.Context(), loopruncontract.LoopRun{ID: "loop-a", SchemaVersion: 1, Goal: "g"})
	if err != nil || !written.OK {
		t.Fatalf("written=%+v err=%v", written, err)
	}
	if len(db.applied) != 1 || db.applied[0].Bucket != "loop" || db.applied[0].ID != "loop-a" {
		t.Fatalf("applied = %+v", db.applied)
	}
	// Read always forces OK=true on decode, so inspect the stored bytes directly.
	var stored loopruncontract.LoopRun
	if err := json.Unmarshal(db.applied[0].Data, &stored); err != nil {
		t.Fatal(err)
	}
	if !stored.OK || stored.Goal != "g" || stored.ID != "loop-a" {
		t.Fatalf("stored record = %+v", stored)
	}
}

func TestReadAllExistingPropagatesScanErrorsExceptNotExist(t *testing.T) {
	sentinel := errors.New("scan broke")
	store := Store{GetAllExisting: func(string, string) ([]port.RecordRow, error) { return nil, sentinel }}
	if obs, err := store.ReadAllExisting(); !errors.Is(err, sentinel) || obs != nil {
		t.Fatalf("obs=%v err=%v", obs, err)
	}
	store.GetAllExisting = func(string, string) ([]port.RecordRow, error) { return nil, fs.ErrNotExist }
	if obs, err := store.ReadAllExisting(); err != nil || obs == nil || len(obs) != 0 {
		t.Fatalf("missing store must yield an empty non-nil list, obs=%#v err=%v", obs, err)
	}
}

func TestStoreWithLockValidatesIDThenRunsInsideSpan(t *testing.T) {
	db := &fakeDB{}
	store := storeOver(db, nil)
	ran := false
	if err := store.WithLock(t.Context(), "bad", func(context.Context) error { ran = true; return nil }); err == nil || !strings.Contains(err.Error(), "invalid loop id") || ran || db.spanCalls != 0 {
		t.Fatalf("invalid id err=%v ran=%v spans=%d", err, ran, db.spanCalls)
	}
	openErr := errors.New("no db")
	if err := storeOver(db, openErr).WithLock(t.Context(), "loop-a", func(context.Context) error { ran = true; return nil }); !errors.Is(err, openErr) || ran {
		t.Fatalf("open failure err=%v ran=%v", err, ran)
	}
	fnErr := errors.New("inner")
	if err := store.WithLock(t.Context(), "loop-a", func(context.Context) error { ran = true; return fnErr }); !errors.Is(err, fnErr) || !ran || db.spanCalls != 1 {
		t.Fatalf("err=%v ran=%v spans=%d", err, ran, db.spanCalls)
	}
}

func TestIdentityNormalizeRepo(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "base")
	identity := Identity{BaseDir: base}
	if _, err := identity.NormalizeRepo("  "); err == nil || !strings.Contains(err.Error(), "repo is required") {
		t.Fatalf("empty repo err=%v", err)
	}
	if got, err := identity.NormalizeRepo(" /a/b/../c "); err != nil || got != filepath.Join(string(filepath.Separator), "a", "c") {
		t.Fatalf("absolute repo = %q err=%v", got, err)
	}
	if got, err := identity.NormalizeRepo("rel/repo"); err != nil || got != filepath.Join(base, "rel", "repo") {
		t.Fatalf("relative repo = %q err=%v", got, err)
	}
	baseErr := errors.New("getwd failed")
	broken := Identity{BaseDir: base, BaseError: baseErr}
	if _, err := broken.NormalizeRepo("rel"); !errors.Is(err, baseErr) {
		t.Fatalf("relative repo with broken base err=%v", err)
	}
	// an absolute path does not need the base directory at all.
	if got, err := broken.NormalizeRepo("/abs"); err != nil || got != filepath.Join(string(filepath.Separator), "abs") {
		t.Fatalf("absolute with broken base = %q err=%v", got, err)
	}
}

func TestIdentityNewIDIsStableAcrossWhitespaceAndSensitiveToInputs(t *testing.T) {
	identity := Identity{}
	id := identity.NewID("/repo", "name")
	if !strings.HasPrefix(id, "loop-") || len(id) != len("loop-")+12 {
		t.Fatalf("id = %q", id)
	}
	if identity.NewID("  /repo ", " name\n") != id {
		t.Fatal("surrounding whitespace must not change the id")
	}
	if identity.NewID("/repo", "other") == id || identity.NewID("/repo2", "name") == id {
		t.Fatal("distinct repo/name pairs must yield distinct ids")
	}
	// the separator keeps ("ab","c") and ("a","bc") apart.
	if identity.NewID("ab", "c") == identity.NewID("a", "bc") {
		t.Fatal("repo/name boundary must be part of the hash")
	}
	if _, err := identity.NormalizeID(id); err != nil {
		t.Fatalf("generated id must be a valid loop id: %v", err)
	}
}

func TestClockNowIsUTCRFC3339Nano(t *testing.T) {
	zone := time.FixedZone("KST", 9*3600)
	clock := Clock{Time: func() time.Time { return time.Date(2026, 1, 2, 12, 0, 0, 123, zone) }}
	if got := clock.Now(); got != "2026-01-02T03:00:00.000000123Z" {
		t.Fatalf("Now() = %q", got)
	}
}
