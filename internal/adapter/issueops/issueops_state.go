// Package issueops is the outbound adapter of the IssueOps cycle: it stores cycle records
// and execution leases in the SQLite state, and implements the git, process, provider-CLI and
// filesystem observations that the application layer consumes through the ports.
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
var issueOpsBucket = fmt.Sprintf("issueops_v%d", issueops.SchemaVersion)

// ReadIssueOpsExisting reads exactly one existing record without creating,
// repairing, migrating, or changing permissions on the state store.
func ReadIssueOpsExisting(stateRoot, id string) (issueops.Record, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return issueops.Record{OK: false}, err
	}
	b, ok, err := sqlstore.GetExisting(stateRoot, issueOpsBucket, id)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return issueops.Record{OK: false, ID: id}, err
	}
	// A missing store answers like a missing record: the caller asked for a
	// cycle, and the store path is an implementation detail.
	if err != nil || !ok {
		return issueops.Record{OK: false, ID: id}, recordNotFound(stateRoot, id)
	}
	return decodeIssueOpsRecord(id, b)
}

// recordNotFound names the cycle, the state it looked in, and the command that
// lists the existing ones. QA: an agent's shell lost ISSUEOPS_STATE_DIR and
// the bare "not found" did not say which state was read.
func recordNotFound(stateRoot, id string) error {
	if data, ok, err := sqlstore.GetExisting(stateRoot, cycleOutcomeBucket, id); err == nil && ok {
		var outcome issueops.CycleOutcome
		if json.Unmarshal(data, &outcome) == nil {
			if note := issueopsdomain.RetiredCycleNote(outcome); note != "" {
				return fmt.Errorf("issueops record %s: %s: %w", id, note, fs.ErrNotExist)
			}
		}
	}
	return fmt.Errorf("issueops record %s not found in state %s (ISSUEOPS_STATE_DIR selects the state); list cycles with `issueops list --json`: %w", id, stateRoot, fs.ErrNotExist)
}

// ReadIssueOps loads and validates the cycle record for id. A missing record wraps
// fs.ErrNotExist; a row that fails to decode or validate returns an Invalid record with
// the invalid-state error.
func ReadIssueOps(stateRoot, id string) (issueops.Record, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return issueops.Record{OK: false}, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.Record{OK: false, ID: id}, err
	}
	b, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		return issueops.Record{OK: false, ID: id}, err
	}
	if !ok {
		return issueops.Record{OK: false, ID: id}, recordNotFound(stateRoot, id)
	}
	return decodeIssueOpsRecord(id, b)
}

func decodeIssueOpsRecord(id string, b []byte) (issueops.Record, error) {
	invalid := issueops.Record{OK: false, ID: id, Invalid: true, InvalidReason: statecontract.ErrInvalidState.Error()}
	var record issueops.Record
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return invalid, statecontract.ErrInvalidState
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return invalid, statecontract.ErrInvalidState
	}
	if record.SchemaVersion != issueops.SchemaVersion || record.ID != id {
		return invalid, statecontract.ErrInvalidState
	}
	if err := validateIssueOpsRecord(record); err != nil {
		return invalid, statecontract.ErrInvalidState
	}
	record.OK = true
	return record, nil
}

// VisitIssueOpsExisting visits every stored cycle in id order from one
// read-only scan without creating or repairing state. A row whose id or record
// is invalid reaches visit with its error. A missing store has no rows.
func VisitIssueOpsExisting(stateRoot string, visit func(id string, record issueops.Record, err error)) error {
	rows, err := sqlstore.GetAllExisting(stateRoot, issueOpsBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, row := range rows {
		id, err := normalizeIssueOpsID(row.ID)
		if err != nil {
			visit(row.ID, issueops.Record{OK: false}, err)
			continue
		}
		record, err := decodeIssueOpsRecord(id, row.Data)
		visit(id, record, err)
	}
	return nil
}

// ScanReadableIssueOps returns every cycle record under stateRoot that decodes and validates.
// Unreadable rows are skipped, and a missing state yields an empty list.
func ScanReadableIssueOps(stateRoot string) ([]issueops.Record, error) {
	records, _, err := scanIssueOpsRows(stateRoot)
	return records, err
}

func scanIssueOpsRows(stateRoot string) ([]issueops.Record, bool, error) {
	rows, err := sqlstore.GetAllExisting(stateRoot, issueOpsBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []issueops.Record{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	records := make([]issueops.Record, 0, len(rows))
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

func writeIssueOps(ctx context.Context, stateRoot string, record issueops.Record) (issueops.Record, error) {
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

func encodeIssueOpsRecord(record issueops.Record) (issueops.Record, []byte, error) {
	if _, err := normalizeIssueOpsID(record.ID); err != nil {
		record.OK = false
		return record, nil, err
	}
	if record.SchemaVersion != issueops.SchemaVersion {
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
		// Probe: `status --json` without --id said only "id is required".
		return "", fmt.Errorf("id is required (--id): %s", issueops.IDFormatHint)
	}
	if !strings.HasPrefix(id, "io-") {
		return "", fmt.Errorf("invalid issueops id %q: %s", id, issueops.IDFormatHint)
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	return id, nil
}

func validateIssueOpsRecord(record issueops.Record) error {
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
