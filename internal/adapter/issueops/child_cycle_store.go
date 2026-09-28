package issueops

import (
	"context"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type ChildCycleStore struct{ CycleRecordStore }

func (s ChildCycleStore) SavePair(ctx context.Context, parent, child model.IssueOpsRecord) (model.IssueOpsRecord, model.IssueOpsRecord, error) {
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
	err = db.Apply(ctx, []port.RecordMutation{{Bucket: issueOpsBucket, ID: parent.ID, Data: parentData}, {Bucket: issueOpsBucket, ID: child.ID, Data: childData}})
	return parent, child, err
}
