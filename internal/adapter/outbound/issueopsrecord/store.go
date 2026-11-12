package issueopsrecord

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

var bucket = fmt.Sprintf("issueops_v%d", issueopscontract.SchemaVersion)

// Store reads and writes issueops cycle records in the sqlstore under stateRoot.
// Scope prefixes the operation names reported to Observer, which may be nil.
type Store struct {
	Scope    string
	Observer Observer
}

// ScanDiagnostic reports a stored row that Scan skipped: the row ID and a code
// (invalid_state when it failed to decode).
type ScanDiagnostic struct {
	ID   string
	Code string
}

type mutation func(
	issueopscontract.Record,
) (issueopscontract.Record, bool, error)

type relatedMutation func(
	issueopscontract.Record,
	[]byte,
	bool,
) ([]byte, bool, error)

func (Store) Read(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopscontract.Record, error) {
	if err := ctx.Err(); err != nil {
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	id, err := normalizeID(id)
	if err != nil {
		return issueopscontract.Record{OK: false}, err
	}
	data, found, err := sqlstore.GetExisting(stateRoot, bucket, id)
	if err != nil {
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	if !found {
		return issueopscontract.Record{OK: false, ID: id}, notFound(ctx, stateRoot, id)
	}
	return Decode(id, data)
}

// notFound suggests, for a truncated or mistyped id, the stored ids it is a
// prefix of. Probe: `status --id io-69864` said only "file does not exist".
func notFound(ctx context.Context, stateRoot, id string) error {
	err := fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
	row, found, getErr := sqlstore.GetExisting(stateRoot, cycleOutcomeBucket, id)
	if retired := retiredOutcome(id, row, found, getErr); retired != "" {
		return fmt.Errorf("%w; %s", err, retired)
	}
	ids, listErr := (Store{}).ListIDs(ctx, stateRoot)
	if listErr != nil {
		return err
	}
	var matches []string
	for _, candidate := range ids {
		if strings.HasPrefix(candidate, id) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) > 0 && len(matches) <= 3 {
		return fmt.Errorf("%w; did you mean %s?", err, strings.Join(matches, ", "))
	}
	return err
}

// cycleOutcomeBucket is the bucket cleanup and prune write a deleted cycle's
// outcome to.
const cycleOutcomeBucket = port.CycleOutcomeBucket

// retiredOutcome names how a deleted cycle ended. QA: after abandon, `status
// --id` said only "file does not exist", which reads like a failure rather
// than the confirmation the agent was looking for.
// A row that does not decode strictly yields no note rather than a guess.
func retiredOutcome(id string, data []byte, ok bool, err error) string {
	if err != nil || !ok {
		return ""
	}
	outcome, err := issueopscontract.DecodeCycleOutcome(id, data)
	if err != nil {
		return ""
	}
	return issueopsdomain.RetiredCycleNote(outcome)
}

// ListIDs returns the IDs of all stored records. A state root without a store
// yields an empty list.
func (Store) ListIDs(ctx context.Context, stateRoot string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := sqlstore.ListExisting(stateRoot, bucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	return ids, err
}

// Scan decodes every stored record, returning them with a diagnostic for each row
// that could not be decoded. It holds all records in memory; use ScanEach to stream.
func (Store) Scan(
	ctx context.Context,
	stateRoot string,
) ([]issueopscontract.Record, []ScanDiagnostic, error) {
	records := []issueopscontract.Record{}
	diagnostics, err := (Store{}).ScanEach(ctx, stateRoot, func(record issueopscontract.Record) error {
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return records, diagnostics, nil
}

// ScanEach decodes every stored record and passes each to visit, stopping at the
// first error visit returns. Rows that fail to decode are skipped and reported
// in the returned diagnostics. A state root without a store yields no diagnostics.
func (Store) ScanEach(
	ctx context.Context,
	stateRoot string,
	visit func(issueopscontract.Record) error,
) ([]ScanDiagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	diagnostics := make([]ScanDiagnostic, 0)
	visited := false
	err := sqlstore.WalkExisting(ctx, stateRoot, bucket, func(row port.RecordRow) error {
		visited = true
		if err := ctx.Err(); err != nil {
			return err
		}
		record, decodeErr := Decode(row.ID, row.Data)
		if decodeErr != nil {
			diagnostics = append(diagnostics, ScanDiagnostic{ID: row.ID, Code: "invalid_state"})
			return nil
		}
		return visit(record)
	})
	if !visited && errors.Is(err, fs.ErrNotExist) {
		return []ScanDiagnostic{}, nil
	}
	if err != nil {
		return nil, err
	}
	return diagnostics, nil
}

// Update reads record id under the store lock, applies mutate and writes the
// result. mutate reports whether it changed anything; an unchanged record is not
// written. A record with a cleanup attempt in progress, or an abandon apply in
// progress, is refused. The returned record has OK set only on success.
func (store Store) Update(
	ctx context.Context,
	stateRoot string,
	id string,
	mutate mutation,
) (issueopscontract.Record, error) {
	id, database, err := open(ctx, stateRoot, id)
	if err != nil {
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	result := issueopscontract.Record{OK: false, ID: id}
	err = database.WithSpan(store.ObserveSpans(ctx, "update"), func(spanContext context.Context) error {
		record, err := readLocked(spanContext, database, id)
		if err != nil {
			return err
		}
		var changed bool
		result, changed, err = mutate(record)
		if err != nil || !changed {
			return err
		}
		if err := issueopsdomain.RequireNoCleanupAttempt(result.CleanupAttempt); err != nil {
			return err
		}
		data, err := Encode(result)
		if err != nil {
			return err
		}
		return database.Apply(spanContext, []port.RecordMutation{{Bucket: bucket, ID: id, Data: data}})
	})
	if err != nil {
		result.OK = false
		return result, err
	}
	result.OK = true
	return result, nil
}

// UpdateRelated reads record id and the row of relatedBucket with the same id
// under the store lock, then applies mutate to them and writes or deletes the
// related row according to mutate's result. The record itself is never written.
func (store Store) UpdateRelated(
	ctx context.Context,
	stateRoot string,
	id string,
	relatedBucket string,
	mutate relatedMutation,
) (issueopscontract.Record, error) {
	id, database, err := open(ctx, stateRoot, id)
	if err != nil {
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	result := issueopscontract.Record{OK: false, ID: id}
	err = database.WithSpan(store.ObserveSpans(ctx, "related_update"), func(spanContext context.Context) error {
		record, err := readLocked(spanContext, database, id)
		if err != nil {
			return err
		}
		data, found, err := database.Get(relatedBucket, id)
		if err != nil {
			return err
		}
		data, remove, err := mutate(record, data, found)
		if err != nil {
			return err
		}
		mutation := port.RecordMutation{Bucket: relatedBucket, ID: id, Data: data}
		if remove {
			mutation = port.RecordMutation{Bucket: relatedBucket, ID: id, Delete: true}
		}
		err = database.Apply(spanContext, []port.RecordMutation{mutation})
		if err == nil {
			result = record
		}
		return err
	})
	if err != nil {
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	result.OK = true
	return result, nil
}

// ReadRelated returns the raw row stored under id in relatedBucket and whether it exists.
func (Store) ReadRelated(
	ctx context.Context,
	stateRoot string,
	id string,
	relatedBucket string,
) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	id, err := normalizeID(id)
	if err != nil {
		return nil, false, err
	}
	return sqlstore.GetExisting(stateRoot, relatedBucket, id)
}

// Delete removes record id and its rows in relatedBuckets in one atomic commit,
// inside the same serialization gate as Update so an open update cannot recreate
// a related row afterwards. A missing record is not an error; a record with a
// cleanup attempt in progress is refused.
func (store Store) Delete(
	ctx context.Context,
	stateRoot string,
	id string,
	relatedBuckets ...string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := normalizeID(id)
	if err != nil {
		return err
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	mutations := make([]port.RecordMutation, 0, len(relatedBuckets)+1)
	for _, relatedBucket := range relatedBuckets {
		mutations = append(mutations, port.RecordMutation{
			Bucket: relatedBucket,
			ID:     id,
			Delete: true,
		})
	}
	mutations = append(mutations, port.RecordMutation{Bucket: bucket, ID: id, Delete: true})
	// 삭제는 update/related-update span과 같은 직렬화 게이트를 지나야 한다.
	// 게이트 밖에서 커밋하면 열려 있는 span이 그 뒤에 related row를 되살려,
	// 레코드는 사라졌는데 related state만 남는 고아가 생긴다.
	return database.WithSpan(store.ObserveSpans(ctx, "delete"), func(spanContext context.Context) error {
		raw, found, err := database.Get(bucket, id)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		record, err := Decode(id, raw)
		if err != nil {
			return err
		}
		if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
			return err
		}
		return database.CompareAndApply(spanContext, []port.ExpectedRecord{{Bucket: bucket, ID: id, Data: raw}}, mutations)
	})
}

// DeleteIfUnchanged removes record id and its rows in relatedBuckets only when the
// stored record still equals expected, so a concurrent change aborts the delete.
// It runs inside the same serialization gate as Update and refuses an expected
// record with a cleanup attempt in progress. The writes are applied in the same
// commit as the delete and never without it; a write that fails in the commit
// rolls the delete back.
func (store Store) DeleteIfUnchanged(
	ctx context.Context,
	stateRoot string,
	id string,
	expected issueopscontract.Record,
	writes []port.RecordMutation,
	relatedBuckets ...string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := normalizeID(id)
	if err != nil {
		return err
	}
	if err := issueopsdomain.RequireNoCleanupAttempt(expected.CleanupAttempt); err != nil {
		return err
	}
	expectedData, err := Encode(expected)
	if err != nil {
		return err
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	mutations := make([]port.RecordMutation, 0, len(relatedBuckets)+len(writes)+1)
	for _, relatedBucket := range relatedBuckets {
		mutations = append(mutations, port.RecordMutation{Bucket: relatedBucket, ID: id, Delete: true})
	}
	mutations = append(mutations, writes...)
	mutations = append(mutations, port.RecordMutation{Bucket: bucket, ID: id, Delete: true})
	// DeleteIfUnchanged도 같은 게이트를 지난다. CAS는 레코드 drift만 막고
	// 열려 있는 span의 related Put과는 순서를 맺지 못한다.
	return database.WithSpan(store.ObserveSpans(ctx, "retention_delete"), func(spanContext context.Context) error {
		return database.CompareAndApply(
			spanContext,
			[]port.ExpectedRecord{{Bucket: bucket, ID: id, Data: expectedData}},
			mutations,
		)
	})
}

func open(ctx context.Context, stateRoot, id string) (string, *sqlstore.DB, error) {
	if err := ctx.Err(); err != nil {
		return id, nil, err
	}
	id, err := normalizeID(id)
	if err != nil {
		return id, nil, err
	}
	database, err := sqlstore.Open(stateRoot)
	return id, database, err
}

func readLocked(
	ctx context.Context,
	database *sqlstore.DB,
	id string,
) (issueopscontract.Record, error) {
	if err := ctx.Err(); err != nil {
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	data, found, err := database.Get(bucket, id)
	if err != nil {
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	if !found {
		err := fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
		row, ok, getErr := database.Get(cycleOutcomeBucket, id)
		if retired := retiredOutcome(id, row, ok, getErr); retired != "" {
			err = fmt.Errorf("%w; %s", err, retired)
		}
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	record, err := Decode(id, data)
	if err != nil {
		return record, err
	}
	if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	if record.CleanupAbandonFailure != nil &&
		record.CleanupAbandonFailure.Step == "applying" {
		return issueopscontract.Record{OK: false, ID: id}, fmt.Errorf(
			"cleanup abandon apply is in progress",
		)
	}
	return record, nil
}
