package issueops

import (
	"context"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type ChildCycleStore struct{ CycleRecordStore }

func (s ChildCycleStore) SavePair(ctx context.Context, parent, child model.IssueOpsRecord) (model.IssueOpsRecord, model.IssueOpsRecord, error) {
	for _, record := range []model.IssueOpsRecord{parent, child} {
		if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
			return parent, child, err
		}
	}
	parent, parentData, err := encodeIssueOpsRecord(parent)
	if err != nil {
		return parent, child, err
	}
	child, childData, err := encodeIssueOpsRecord(child)
	if err != nil {
		return parent, child, err
	}
	db, err := sqlstore.Open(s.StateRoot)
	if err != nil {
		return parent, child, err
	}
	expected := []port.ExpectedRecord{}
	mutations := []port.RecordMutation{{Bucket: issueOpsBucket, ID: parent.ID, Data: parentData}, {Bucket: issueOpsBucket, ID: child.ID, Data: childData}}
	for i := range mutations {
		raw, found, err := mutableIssueOpsRaw(db, mutations[i].ID)
		if err != nil {
			return parent, child, err
		}
		if found {
			expected = append(expected, port.ExpectedRecord{Bucket: issueOpsBucket, ID: mutations[i].ID, Data: raw})
		} else {
			mutations[i].RequireAbsent = true
		}
	}
	err = db.CompareAndApply(ctx, expected, mutations)
	return parent, child, err
}

func (s ChildCycleStore) Scan() ([]model.IssueOpsRecord, error) {
	return ScanReadableIssueOps(s.StateRoot)
}
