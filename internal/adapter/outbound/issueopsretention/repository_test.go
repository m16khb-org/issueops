package issueopsretention

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

func TestRepositoryReadsListsAndDeletesRecordWithStagedArtifact(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-retention01"
	record := issueopscontract.Record{
		OK:            true,
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id,
		Phase:         issueopscontract.PhaseDone,
		UpdatedAt:     "2026-06-01T00:00:00Z",
	}
	data, err := issueopsrecord.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{
		{Bucket: "issueops_v1", ID: id, Data: data},
		{Bucket: artifactStageBucket, ID: id, Data: []byte("staged")},
	}); err != nil {
		t.Fatal(err)
	}

	repository := Repository{}
	ids, err := repository.ListIDs(context.Background(), stateRoot)
	if err != nil || len(ids) != 1 || ids[0] != id {
		t.Fatalf("list returned %+v, %v", ids, err)
	}
	got, err := repository.ReadUnchecked(context.Background(), stateRoot, id)
	if err != nil || !got.OK || got.ID != id {
		t.Fatalf("read returned %+v, %v", got, err)
	}
	if err := repository.DeleteIfUnchanged(context.Background(), stateRoot, id, got, issueopsdomain.CycleOutcomeForRecord(got, "pruned", "2026-11-12T00:00:00Z")); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := database.Get("issueops_v1", id); err != nil || ok {
		t.Fatalf("record survived delete: ok=%v err=%v", ok, err)
	}
	if _, ok, err := database.Get(artifactStageBucket, id); err != nil || ok {
		t.Fatalf("artifact survived delete: ok=%v err=%v", ok, err)
	}
	if _, err := repository.ReadUnchecked(context.Background(), stateRoot, id); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("deleted record read error = %v", err)
	}
}

const retainedAt = "2026-11-12T00:00:00Z"

// seedRetentionRecord stores a done record and a staged artifact row under id and
// returns the open state DB with the record as the retention reader sees it.
func seedRetentionRecord(t *testing.T, stateRoot, id string) (*sqlstore.DB, issueopscontract.Record) {
	t.Helper()
	data, err := issueopsrecord.Encode(issueopscontract.Record{
		OK:            true,
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id,
		Repo:          "/repo/retention",
		Phase:         issueopscontract.PhaseDone,
		CreatedAt:     "2026-05-01T00:00:00Z",
		UpdatedAt:     "2026-06-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{
		{Bucket: "issueops_v1", ID: id, Data: data},
		{Bucket: artifactStageBucket, ID: id, Data: []byte("staged")},
	}); err != nil {
		t.Fatal(err)
	}
	record, err := Repository{}.ReadUnchecked(context.Background(), stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	return database, record
}

// retentionRowSnapshot returns a sorted (bucket/id sha256(data)) line for every row
// of the state DB, read through a read-only connection so the snapshot itself
// never changes state.
func retentionRowSnapshot(t *testing.T, stateRoot string) []string {
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

// readRetainedOutcome reads the retained row of id through the shared record
// store and decodes it into the contract type.
func readRetainedOutcome(t *testing.T, stateRoot, id string) (issueopscontract.CycleOutcome, bool) {
	t.Helper()
	data, found, err := (issueopsrecord.Store{}).ReadRelated(context.Background(), stateRoot, id, port.CycleOutcomeBucket)
	if err != nil {
		t.Fatalf("read retained outcome %s: %v", id, err)
	}
	var outcome issueopscontract.CycleOutcome
	if found {
		if err := json.Unmarshal(data, &outcome); err != nil {
			t.Fatalf("decode retained outcome %s: %v", id, err)
		}
	}
	return outcome, found
}

func requireNoRetainedOutcome(t *testing.T, stateRoot, id string) {
	t.Helper()
	if got, ok := readRetainedOutcome(t, stateRoot, id); ok {
		t.Fatalf("retained outcome = %+v, want none", got)
	}
}

func TestRepositoryDeleteIfUnchangedRetainsOutcomeInSameCommit(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-retain-same"
	database, record := seedRetentionRecord(t, stateRoot, id)
	outcome := issueopsdomain.CycleOutcomeForRecord(record, "pruned", retainedAt)

	if err := (Repository{}).DeleteIfUnchanged(context.Background(), stateRoot, id, record, outcome); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := database.Get("issueops_v1", id); err != nil || ok {
		t.Fatalf("record survived delete: ok=%v err=%v", ok, err)
	}
	if _, ok, err := database.Get(artifactStageBucket, id); err != nil || ok {
		t.Fatalf("artifact survived delete: ok=%v err=%v", ok, err)
	}
	got, found := readRetainedOutcome(t, stateRoot, id)
	if !found {
		t.Fatal("no retained outcome after the delete")
	}
	if !reflect.DeepEqual(got, outcome) {
		t.Fatalf("retained outcome = %+v, want %+v", got, outcome)
	}
}

// A retained row is one per cycle ID. When a cycle that reused the ID of an
// earlier, already retained cycle is pruned, its outcome replaces the earlier
// row (upsert), as a second cleanup of the same ID always did.
func TestRepositoryDeleteIfUnchangedOverwritesAnEarlierOutcomeOfTheSameID(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-retain-reused"
	database, record := seedRetentionRecord(t, stateRoot, id)
	earlierRecord := record
	earlierRecord.CreatedAt = "2026-01-01T00:00:00Z"
	earlier, err := issueopscontract.EncodeCycleOutcome(id, issueopsdomain.CycleOutcomeForRecord(earlierRecord, "finished", "2026-02-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{{Bucket: port.CycleOutcomeBucket, ID: id, Data: earlier}}); err != nil {
		t.Fatal(err)
	}
	outcome := issueopsdomain.CycleOutcomeForRecord(record, "pruned", retainedAt)
	if reflect.DeepEqual(outcome.Timeline, issueopsdomain.CycleOutcomeForRecord(earlierRecord, "pruned", retainedAt).Timeline) {
		t.Fatal("the earlier and the new record share a timeline; the test cannot tell the rows apart")
	}

	if err := (Repository{}).DeleteIfUnchanged(context.Background(), stateRoot, id, record, outcome); err != nil {
		t.Fatal(err)
	}

	got, found := readRetainedOutcome(t, stateRoot, id)
	if !found || got.Outcome != "pruned" || !reflect.DeepEqual(got, outcome) {
		t.Fatalf("retained outcome = %+v (found=%v), want the pruned outcome of the new record %+v", got, found, outcome)
	}
}

func TestRepositoryDeleteIfUnchangedLeavesRecordAndOutcomeOnDrift(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-retain-drift"
	database, record := seedRetentionRecord(t, stateRoot, id)
	outcome := issueopsdomain.CycleOutcomeForRecord(record, "pruned", retainedAt)
	if _, err := (issueopsrecord.Store{}).Update(context.Background(), stateRoot, id, func(current issueopscontract.Record) (issueopscontract.Record, bool, error) {
		current.Branch = "100-after"
		return current, true, nil
	}); err != nil {
		t.Fatal(err)
	}

	err := (Repository{}).DeleteIfUnchanged(context.Background(), stateRoot, id, record, outcome)

	if _, ok := errors.AsType[*sqlstore.RawCASError](err); !ok {
		t.Fatalf("error = %v, want RawCASError", err)
	}
	current, readErr := Repository{}.ReadUnchecked(context.Background(), stateRoot, id)
	if readErr != nil || current.Branch != "100-after" {
		t.Fatalf("current record = %+v, err = %v", current, readErr)
	}
	requireNoRetainedOutcome(t, stateRoot, id)
	if _, ok, err := database.Get(artifactStageBucket, id); err != nil || !ok {
		t.Fatalf("staged artifact was removed by a failed delete: ok=%v err=%v", ok, err)
	}
}

func TestRepositoryDeleteIfUnchangedKeepsRecordWhenOutcomeWriteFailsInCommit(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-retain-trigger"
	_, record := seedRetentionRecord(t, stateRoot, id)
	outcome := issueopsdomain.CycleOutcomeForRecord(record, "pruned", retainedAt)
	raw, err := sql.Open("sqlite", filepath.Join(stateRoot, "issueops.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`CREATE TRIGGER fail_outcome BEFORE INSERT ON records WHEN NEW.bucket = 'issueops_cycle_outcome_v1' BEGIN SELECT RAISE(ABORT, 'injected outcome write failure'); END`); err != nil {
		t.Fatal(err)
	}
	before := retentionRowSnapshot(t, stateRoot)

	err = (Repository{}).DeleteIfUnchanged(context.Background(), stateRoot, id, record, outcome)

	if err == nil || !strings.Contains(err.Error(), "injected outcome write failure") {
		t.Fatalf("error = %v, want the injected outcome write failure", err)
	}
	if after := retentionRowSnapshot(t, stateRoot); !slices.Equal(before, after) {
		t.Fatalf("rows changed although the commit failed:\nbefore %v\nafter  %v", before, after)
	}
	requireNoRetainedOutcome(t, stateRoot, id)
}

func TestRepositoryDeleteIfUnchangedKeepsRecordWhenOutcomeEncodeFails(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-retain-encode"
	database, record := seedRetentionRecord(t, stateRoot, id)
	outcome := issueopsdomain.CycleOutcomeForRecord(record, "pruned", retainedAt)
	outcome.ID = "io-someone-else"

	if err := (Repository{}).DeleteIfUnchanged(context.Background(), stateRoot, id, record, outcome); err == nil {
		t.Fatal("delete succeeded with an outcome of another cycle")
	}

	if _, ok, err := database.Get("issueops_v1", id); err != nil || !ok {
		t.Fatalf("record was removed: ok=%v err=%v", ok, err)
	}
	if _, ok, err := database.Get(artifactStageBucket, id); err != nil || !ok {
		t.Fatalf("staged artifact was removed: ok=%v err=%v", ok, err)
	}
	requireNoRetainedOutcome(t, stateRoot, id)
	requireNoRetainedOutcome(t, stateRoot, outcome.ID)
}
