package issueopsrecord

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestDeleteIfUnchangedRejectsRecordDrift(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-delete-cas"
	store := Store{}
	data, err := Encode(issueopscontract.Record{
		OK:            true,
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id,
		Repo:          t.TempDir(),
		Branch:        "100-before",
		Phase:         issueopscontract.PhaseDone,
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, id, data); err != nil {
		t.Fatal(err)
	}
	expected, err := store.Read(context.Background(), stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(context.Background(), stateRoot, id, func(record issueopscontract.Record) (issueopscontract.Record, bool, error) {
		record.Branch = "100-after"
		return record, true, nil
	}); err != nil {
		t.Fatal(err)
	}

	err = store.DeleteIfUnchanged(context.Background(), stateRoot, id, expected, nil, "artifact_stage_v1")

	if _, ok := errors.AsType[*sqlstore.RawCASError](err); !ok {
		t.Fatalf("error = %v, want RawCASError", err)
	}
	current, readErr := store.Read(context.Background(), stateRoot, id)
	if readErr != nil || current.Branch != "100-after" {
		t.Fatalf("current record = %+v, err = %v", current, readErr)
	}
}

func TestDeleteIfUnchangedSerializesNewRelatedState(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-delete-related"
	store := Store{}
	expected := issueopscontract.Record{
		OK:            true,
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id,
		Repo:          t.TempDir(),
		Branch:        "101-related",
		Phase:         issueopscontract.PhaseDone,
	}
	data, err := Encode(expected)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, id, data); err != nil {
		t.Fatal(err)
	}

	updateStarted := make(chan struct{})
	updateTxEntered := make(chan struct{})
	releaseUpdate := make(chan struct{})
	updateDone := make(chan error, 1)
	go func() {
		_, updateErr := store.UpdateRelated(
			context.Background(),
			stateRoot,
			id,
			"artifact_stage_v1",
			func(_ issueopscontract.Record, _ []byte, _ bool) ([]byte, bool, error) {
				// 이 콜백은 이미 관련 트랜잭션(직렬화 게이트 + BEGIN IMMEDIATE)
				// 안에서 실행 중이다. delete가 먼저 커밋되고 이 Put이 되살리는
				// 순서 반전을 막으려면, delete는 이 트랜잭션이 커밋된 뒤에만
				// 게이트를 통과해야 한다. 게이트가 이미 releaseUpdate 대기로
				// 블록된 이 트랜잭션을 잡고 있으므로, delete 파생은 release 후에
				// 시작해야 순서가 보장된다.
				close(updateTxEntered)
				<-releaseUpdate
				return []byte(`{"plan":"new"}`), false, nil
			},
		)
		updateDone <- updateErr
	}()
	<-updateTxEntered
	close(updateStarted)

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- store.DeleteIfUnchanged(
			context.Background(),
			stateRoot,
			id,
			expected,
			nil,
			"artifact_stage_v1",
		)
	}()
	close(releaseUpdate)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for name, result := range map[string]<-chan error{
		"related update":   updateDone,
		"retention delete": deleteDone,
	} {
		select {
		case operationErr := <-result:
			if operationErr != nil {
				t.Fatalf("%s: %v", name, operationErr)
			}
		case <-ctx.Done():
			t.Fatalf("%s did not finish: %v", name, ctx.Err())
		}
	}
	if _, found, getErr := database.Get(bucket, id); getErr != nil || found {
		t.Fatalf("record survived serialized delete: found=%v err=%v", found, getErr)
	}
	if _, found, getErr := database.Get("artifact_stage_v1", id); getErr != nil || found {
		t.Fatalf("related state survived serialized delete: found=%v err=%v", found, getErr)
	}
}

// DeleteIfUnchanged는 related update span이 열려 있는 동안 커밋해서는 안 된다.
// span이 직렬화 게이트를 쥔 채 콜백에서 블록돼 있으면 delete는 그 span이 끝날
// 때까지 기다려야 한다. 기다리지 않으면 delete가 먼저 커밋되고 span의 Put이
// related row를 되살려, 레코드는 사라졌는데 related state만 남는 고아가 된다.
func TestDeleteIfUnchangedWaitsForOpenRelatedUpdateSpan(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-delete-span"
	store := Store{}
	expected := issueopscontract.Record{
		OK:            true,
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id,
		Repo:          t.TempDir(),
		Branch:        "102-span",
		Phase:         issueopscontract.PhaseDone,
	}
	data, err := Encode(expected)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, id, data); err != nil {
		t.Fatal(err)
	}

	spanEntered := make(chan struct{})
	releaseSpan := make(chan struct{})
	updateDone := make(chan error, 1)
	go func() {
		_, updateErr := store.UpdateRelated(
			context.Background(),
			stateRoot,
			id,
			"artifact_stage_v1",
			func(_ issueopscontract.Record, _ []byte, _ bool) ([]byte, bool, error) {
				close(spanEntered)
				<-releaseSpan
				return []byte(`{"plan":"new"}`), false, nil
			},
		)
		updateDone <- updateErr
	}()
	<-spanEntered

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- store.DeleteIfUnchanged(
			context.Background(),
			stateRoot,
			id,
			expected,
			nil,
			"artifact_stage_v1",
		)
	}()

	select {
	case deleteErr := <-deleteDone:
		t.Fatalf("delete committed while the related update span was still open: %v", deleteErr)
	case <-time.After(500 * time.Millisecond):
	}

	close(releaseSpan)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for name, result := range map[string]<-chan error{
		"related update":   updateDone,
		"retention delete": deleteDone,
	} {
		select {
		case operationErr := <-result:
			if operationErr != nil {
				t.Fatalf("%s: %v", name, operationErr)
			}
		case <-ctx.Done():
			t.Fatalf("%s did not finish: %v", name, ctx.Err())
		}
	}
	if _, found, getErr := database.Get(bucket, id); getErr != nil || found {
		t.Fatalf("record survived the delete: found=%v err=%v", found, getErr)
	}
	if _, found, getErr := database.Get("artifact_stage_v1", id); getErr != nil || found {
		t.Fatalf("related state survived the delete: found=%v err=%v", found, getErr)
	}
}

func TestStoreReadsUpdatesRelatedDataAndDeletesAtomically(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-record01"
	data, err := json.Marshal(issueopscontract.Record{
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id,
		Phase:         issueopscontract.PhaseProblem,
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, id, data); err != nil {
		t.Fatal(err)
	}

	store := Store{}
	record, err := store.Update(
		context.Background(),
		stateRoot,
		id,
		func(record issueopscontract.Record) (issueopscontract.Record, bool, error) {
			record.Branch = "record-store"
			return record, true, nil
		},
	)
	if err != nil || record.Branch != "record-store" {
		t.Fatalf("update returned %+v, %v", record, err)
	}
	if _, err := store.UpdateRelated(
		context.Background(),
		stateRoot,
		id,
		"related_v1",
		func(_ issueopscontract.Record, _ []byte, _ bool) ([]byte, bool, error) {
			return []byte("related"), false, nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), stateRoot, id, "related_v1"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := database.Get(bucket, id); err != nil || found {
		t.Fatalf("record survived delete: found=%v err=%v", found, err)
	}
	if _, found, err := database.Get("related_v1", id); err != nil || found {
		t.Fatalf("related data survived delete: found=%v err=%v", found, err)
	}
}

func TestStoreScansValidAndInvalidRowsInOneInventory(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	valid := issueopscontract.Record{
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            "io-valid",
		Phase:         issueopscontract.PhaseProblem,
	}
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, valid.ID, encoded); err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, "io-invalid", []byte(`{"schema_version":1,"id":"io-invalid","phase":"unknown"}`)); err != nil {
		t.Fatal(err)
	}

	records, diagnostics, err := (Store{}).Scan(context.Background(), stateRoot)

	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ID != valid.ID {
		t.Fatalf("records = %+v", records)
	}
	if len(diagnostics) != 1 ||
		diagnostics[0].ID != "io-invalid" ||
		diagnostics[0].Code != "invalid_state" {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

// Probe: `status --id io-69864` (a truncated id) said only "file does not
// exist"; the error names the stored ids it is a prefix of.
func TestReadNotFoundSuggestsIDsWithThePrefix(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, "io-6986473fe689", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	_, err = Store{}.Read(context.Background(), stateRoot, "io-69864")
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "did you mean io-6986473fe689?") {
		t.Fatalf("err = %v", err)
	}
	_, err = Store{}.Read(context.Background(), stateRoot, "io-ffff")
	if !errors.Is(err, fs.ErrNotExist) || strings.Contains(err.Error(), "did you mean") {
		t.Fatalf("err = %v", err)
	}
}

// QA: after abandon, `status --id` said only "file does not exist"; a cycle
// whose record cleanup removed names its kept outcome instead.
func TestReadNotFoundNamesTheRetainedOutcome(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	outcome := []byte(`{"schema_version":1,"id":"io-gone","outcome":"abandoned","retained_at":"2026-10-26T15:00:17Z"}`)
	if err := database.Put(cycleOutcomeBucket, "io-gone", outcome); err != nil {
		t.Fatal(err)
	}
	_, err = Store{}.Read(context.Background(), stateRoot, "io-gone")
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "cleanup removed it (outcome abandoned at 2026-10-26T15:00:17Z)") {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteIfUnchangedAppliesWritesOnlyWithTheDelete(t *testing.T) {
	store := Store{}
	writes := []port.RecordMutation{{Bucket: cycleOutcomeBucket, ID: "io-writes", Data: []byte(`{"x":1}`)}}

	t.Run("success stores the writes with the delete", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
		putRecord(t, stateRoot, baseRecord("io-writes"))
		expected, err := store.Read(context.Background(), stateRoot, "io-writes")
		if err != nil {
			t.Fatal(err)
		}

		if err := store.DeleteIfUnchanged(context.Background(), stateRoot, "io-writes", expected, writes); err != nil {
			t.Fatal(err)
		}

		database, err := sqlstore.Open(stateRoot)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok, err := database.Get(bucket, "io-writes"); err != nil || ok {
			t.Fatalf("record survived delete: ok=%v err=%v", ok, err)
		}
		got, ok, err := database.Get(cycleOutcomeBucket, "io-writes")
		if err != nil || !ok || string(got) != `{"x":1}` {
			t.Fatalf("written row = %q, ok=%v, err=%v", got, ok, err)
		}
	})

	t.Run("drift applies neither the delete nor the writes", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
		putRecord(t, stateRoot, baseRecord("io-writes"))
		expected, err := store.Read(context.Background(), stateRoot, "io-writes")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Update(context.Background(), stateRoot, "io-writes", func(record issueopscontract.Record) (issueopscontract.Record, bool, error) {
			record.Branch = "100-after"
			return record, true, nil
		}); err != nil {
			t.Fatal(err)
		}

		err = store.DeleteIfUnchanged(context.Background(), stateRoot, "io-writes", expected, writes)

		if _, ok := errors.AsType[*sqlstore.RawCASError](err); !ok {
			t.Fatalf("error = %v, want RawCASError", err)
		}
		current, readErr := store.Read(context.Background(), stateRoot, "io-writes")
		if readErr != nil || current.Branch != "100-after" {
			t.Fatalf("current record = %+v, err = %v", current, readErr)
		}
		database, err := sqlstore.Open(stateRoot)
		if err != nil {
			t.Fatal(err)
		}
		if got, ok, err := database.Get(cycleOutcomeBucket, "io-writes"); err != nil || ok {
			t.Fatalf("a write landed without the delete: %q ok=%v err=%v", got, ok, err)
		}
	})
}

func TestDeleteIfUnchangedRollsBackWhenAWriteFailsInCommit(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-rollback"
	store := Store{}
	putRecord(t, stateRoot, baseRecord(id))
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{
		{Bucket: "artifact_stage_v1", ID: id, Data: []byte("staged")},
		{Bucket: cycleOutcomeBucket, ID: id, Data: []byte(`{"old":true}`)},
	}); err != nil {
		t.Fatal(err)
	}
	expected, err := store.Read(context.Background(), stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotStoreRows(t, stateRoot)

	err = store.DeleteIfUnchanged(
		context.Background(),
		stateRoot,
		id,
		expected,
		[]port.RecordMutation{{Bucket: cycleOutcomeBucket, ID: id, Data: []byte(`{"new":true}`), RequireAbsent: true}},
		"artifact_stage_v1",
	)

	if err == nil || !strings.Contains(err.Error(), "precondition failed") {
		t.Fatalf("error = %v, want a precondition failure", err)
	}
	if after := snapshotStoreRows(t, stateRoot); !slices.Equal(before, after) {
		t.Fatalf("rows changed although the commit failed:\nbefore %v\nafter  %v", before, after)
	}
}

// snapshotStoreRows returns a sorted (bucket/id sha256(data)) line for every row
// of the state DB, read through a read-only connection so the snapshot itself
// never changes state.
func snapshotStoreRows(t *testing.T, stateRoot string) []string {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(stateRoot, "issueops.db"))+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	rows, err := database.Query(`SELECT bucket, id, data FROM records ORDER BY bucket, id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	lines := []string{}
	for rows.Next() {
		var rowBucket, rowID string
		var data []byte
		if err := rows.Scan(&rowBucket, &rowID, &data); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		lines = append(lines, rowBucket+"/"+rowID+" "+hex.EncodeToString(sum[:]))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

// The retained outcome bucket name is persisted: changing it hides every row
// already stored. It must stay the versioned name readers derive from the
// outcome schema version.
func TestCycleOutcomeBucketKeepsItsPersistedName(t *testing.T) {
	if port.CycleOutcomeBucket != "issueops_cycle_outcome_v1" {
		t.Fatalf("CycleOutcomeBucket = %q, want issueops_cycle_outcome_v1", port.CycleOutcomeBucket)
	}
	if want := fmt.Sprintf("issueops_cycle_outcome_v%d", issueopscontract.CycleOutcomeSchemaVersion); port.CycleOutcomeBucket != want {
		t.Fatalf("CycleOutcomeBucket = %q, want %q from the outcome schema version", port.CycleOutcomeBucket, want)
	}
}
