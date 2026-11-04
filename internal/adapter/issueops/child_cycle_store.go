package issueops

import (
	"context"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// ChildCycleStore persists parent and child cycle records together, on top of the
// single-record CycleRecordStore.
type ChildCycleStore struct{ CycleRecordStore }

// SavePair writes the parent and child records in one compare-and-apply transaction so
// both land or neither does. Records carrying a cleanup attempt are refused, and an
// existing row must still match the bytes read just before the write.
func (s ChildCycleStore) SavePair(ctx context.Context, parent, child model.Record) (model.Record, model.Record, error) {
	for _, record := range []model.Record{parent, child} {
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

// Scan lists every readable cycle record under the state root.
func (s ChildCycleStore) Scan() ([]model.Record, error) {
	return ScanReadableIssueOps(s.StateRoot)
}
