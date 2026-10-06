package issueops

import (
	"context"
	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	"issueops/internal/port"
	"time"
)

// deleteIssueOps removes the cycle record for id the way cleanup does, so
// tests can observe a deleted child or cycle; deleting an absent record is
// not an error.
func deleteIssueOps(ctx context.Context, stateRoot, id string) error {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	raw, found, err := mutableIssueOpsRaw(db, id)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	// 스테이징 artifact는 레코드와 수명을 같이한다 — 레코드 삭제(prune,
	// cleanup finish)가 스테이지 blob을 고아로 남기지 않는다(C4a-F1 ②).
	return db.CompareAndApply(ctx, []port.ExpectedRecord{{Bucket: issueOpsBucket, ID: id, Data: raw}}, []port.RecordMutation{
		{Bucket: artifactStageBucket, ID: id, Delete: true},
		{Bucket: issueOpsBucket, ID: id, Delete: true},
	})
}

func touchAndWriteIssueOps(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return writeIssueOps(ctx, stateRoot, record)
}
