package issueopsrecord

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

var bucket = fmt.Sprintf("issueops_v%d", issueopscontract.IssueOpsSchemaVersion)

type Store struct {
	Scope    string
	Observer Observer
}

type ScanDiagnostic struct {
	ID   string
	Code string
}

type Mutation func(
	issueopscontract.IssueOpsRecord,
) (issueopscontract.IssueOpsRecord, bool, error)

type RelatedMutation func(
	issueopscontract.IssueOpsRecord,
	[]byte,
	bool,
) ([]byte, bool, error)

func Bucket() string {
	return bucket
}

func (Store) Read(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopscontract.IssueOpsRecord, error) {
	if err := ctx.Err(); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	id, err := NormalizeID(id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	data, found, err := sqlstore.GetExisting(stateRoot, bucket, id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	if !found {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf(
			"issueops record %s: %w",
			id,
			fs.ErrNotExist,
		)
	}
	return Decode(id, data)
}

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

func (Store) Scan(
	ctx context.Context,
	stateRoot string,
) ([]issueopscontract.IssueOpsRecord, []ScanDiagnostic, error) {
	records := []issueopscontract.IssueOpsRecord{}
	diagnostics, err := (Store{}).ScanEach(ctx, stateRoot, func(record issueopscontract.IssueOpsRecord) error {
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return records, diagnostics, nil
}

func (Store) ScanEach(
	ctx context.Context,
	stateRoot string,
	visit func(issueopscontract.IssueOpsRecord) error,
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

func (store Store) Update(
	ctx context.Context,
	stateRoot string,
	id string,
	mutate Mutation,
) (issueopscontract.IssueOpsRecord, error) {
	id, database, err := open(ctx, stateRoot, id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	result := issueopscontract.IssueOpsRecord{OK: false, ID: id}
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

func (store Store) UpdateRelated(
	ctx context.Context,
	stateRoot string,
	id string,
	relatedBucket string,
	mutate RelatedMutation,
) (issueopscontract.IssueOpsRecord, error) {
	id, database, err := open(ctx, stateRoot, id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	result := issueopscontract.IssueOpsRecord{OK: false, ID: id}
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
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	result.OK = true
	return result, nil
}

func (Store) ReadRelated(
	ctx context.Context,
	stateRoot string,
	id string,
	relatedBucket string,
) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	id, err := NormalizeID(id)
	if err != nil {
		return nil, false, err
	}
	return sqlstore.GetExisting(stateRoot, relatedBucket, id)
}

func (store Store) Delete(
	ctx context.Context,
	stateRoot string,
	id string,
	relatedBuckets ...string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := NormalizeID(id)
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

func (store Store) DeleteIfUnchanged(
	ctx context.Context,
	stateRoot string,
	id string,
	expected issueopscontract.IssueOpsRecord,
	relatedBuckets ...string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := NormalizeID(id)
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
	mutations := make([]port.RecordMutation, 0, len(relatedBuckets)+1)
	for _, relatedBucket := range relatedBuckets {
		mutations = append(mutations, port.RecordMutation{Bucket: relatedBucket, ID: id, Delete: true})
	}
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
	id, err := NormalizeID(id)
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
) (issueopscontract.IssueOpsRecord, error) {
	if err := ctx.Err(); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	data, found, err := database.Get(bucket, id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	if !found {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf(
			"issueops record %s: %w",
			id,
			fs.ErrNotExist,
		)
	}
	record, err := Decode(id, data)
	if err != nil {
		return record, err
	}
	if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	if record.CleanupAbandonFailure != nil &&
		record.CleanupAbandonFailure.Step == "applying" {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf(
			"cleanup abandon apply is in progress",
		)
	}
	return record, nil
}
