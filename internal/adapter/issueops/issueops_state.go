package issueops

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// 레코드는 schema 버전별 물리 namespace에만 읽고 쓴다.
// namespace 이름은 issueops.IssueOpsSchemaVersion에서 파생된다.
var issueOpsBucket = fmt.Sprintf("issueops_v%d", issueops.IssueOpsSchemaVersion)

// ReadIssueOpsExisting reads exactly one existing record without creating,
// repairing, migrating, or changing permissions on the state store.
func ReadIssueOpsExisting(stateRoot, id string) (issueops.IssueOpsRecord, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	b, ok, err := sqlstore.GetExisting(stateRoot, issueOpsBucket, id)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: id}, err
	}
	if !ok {
		return issueops.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
	}
	return decodeIssueOpsRecord(id, b)
}

func ReadIssueOps(stateRoot, id string) (issueops.IssueOpsRecord, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: id}, err
	}
	b, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: id}, err
	}
	if !ok {
		return issueops.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
	}
	return decodeIssueOpsRecord(id, b)
}

func decodeIssueOpsRecord(id string, b []byte) (issueops.IssueOpsRecord, error) {
	invalid := issueops.IssueOpsRecord{OK: false, ID: id, Invalid: true, InvalidReason: statecontract.ErrInvalidState.Error()}
	var record issueops.IssueOpsRecord
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return invalid, statecontract.ErrInvalidState
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return invalid, statecontract.ErrInvalidState
	}
	if record.SchemaVersion != issueops.IssueOpsSchemaVersion || record.ID != id {
		return invalid, statecontract.ErrInvalidState
	}
	if err := validateIssueOpsRecord(record); err != nil {
		return invalid, statecontract.ErrInvalidState
	}
	record.OK = true
	return record, nil
}

// ListIssueOpsIDs returns every cycle id stored under stateRoot in ascending
// order.
func ListIssueOpsIDs(stateRoot string) ([]string, error) {
	ids, err := sqlstore.ListExisting(stateRoot, issueOpsBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func ScanReadableIssueOps(stateRoot string) ([]issueops.IssueOpsRecord, error) {
	records, _, err := scanIssueOpsRows(stateRoot)
	return records, err
}

func scanIssueOpsRows(stateRoot string) ([]issueops.IssueOpsRecord, bool, error) {
	rows, err := sqlstore.GetAllExisting(stateRoot, issueOpsBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []issueops.IssueOpsRecord{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	records := make([]issueops.IssueOpsRecord, 0, len(rows))
	invalid := false
	for _, row := range rows {
		record, decodeErr := decodeIssueOpsRecord(row.ID, row.Data)
		if decodeErr != nil {
			invalid = true
			continue
		}
		records = append(records, record)
	}
	return records, invalid, nil
}

func newIssueOpsID(repo, branch string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(branch)))
	return "io-" + hex.EncodeToString(sum[:])[:12]
}

func newIndependentIssueOpsID(repo string) (string, error) {
	var nonce [16]byte
	if _, err := cryptorand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generate independent issueops id: %w", err)
	}
	seed := append([]byte(strings.TrimSpace(repo)+"\x00"), nonce[:]...)
	sum := sha256.Sum256(seed)
	return "io-" + hex.EncodeToString(sum[:])[:12], nil
}

func writeIssueOps(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
	record, b, err := encodeIssueOpsRecord(record)
	if err != nil {
		return record, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		record.OK = false
		return record, err
	}
	if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
		record.OK = false
		return record, err
	}
	raw, found, err := mutableIssueOpsRaw(db, record.ID)
	if err != nil {
		record.OK = false
		return record, err
	}
	mutation := port.RecordMutation{Bucket: issueOpsBucket, ID: record.ID, Data: b}
	if found {
		err = db.CompareAndApply(ctx, []port.ExpectedRecord{{Bucket: issueOpsBucket, ID: record.ID, Data: raw}}, []port.RecordMutation{mutation})
	} else {
		mutation.RequireAbsent = true
		err = db.Apply(ctx, []port.RecordMutation{mutation})
	}
	if err != nil {
		record.OK = false
		return record, err
	}
	return record, nil
}

func encodeIssueOpsRecord(record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, []byte, error) {
	if _, err := normalizeIssueOpsID(record.ID); err != nil {
		record.OK = false
		return record, nil, err
	}
	if record.SchemaVersion != issueops.IssueOpsSchemaVersion {
		record.OK = false
		return record, nil, statecontract.ErrInvalidState
	}
	if err := validateIssueOpsRecord(record); err != nil {
		record.OK = false
		return record, nil, err
	}
	record.OK = true
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		record.OK = false
		return record, nil, err
	}
	return record, b, nil
}

func normalizeIssueOpsID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	if !strings.HasPrefix(id, "io-") {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	return id, nil
}

func validateIssueOpsRecord(record issueops.IssueOpsRecord) error {
	if err := issueops.ValidateRecord(record); err != nil {
		return statecontract.ErrInvalidState
	}
	if err := issueopsdomain.ValidateRecordInvariants(record); err != nil {
		return statecontract.ErrInvalidState
	}
	return nil
}

// mutableIssueOpsRaw binds an ordinary writer to the current unfenced bytes.
// The caller must use these bytes in its CAS, or RequireAbsent for creation.
func mutableIssueOpsRaw(db *sqlstore.DB, id string) ([]byte, bool, error) {
	raw, found, err := db.Get(issueOpsBucket, id)
	if err != nil || !found {
		return raw, found, err
	}
	record, err := decodeIssueOpsRecord(id, raw)
	if err != nil {
		return nil, false, err
	}
	if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
		return nil, false, err
	}
	return raw, true, nil
}
