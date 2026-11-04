package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

var leaseHolderBucket = fmt.Sprintf("lease_holder_v%d", issueops.SchemaVersion)

// ListLeaseHolderIndexes는 state를 생성하거나 복구하지 않고 reverse index를
// 읽는다. 모든 row는 content에서 파생한 key와 대조해 검증한다.
func ListLeaseHolderIndexes(stateRoot string) ([]issueops.LeaseHolderIndex, error) {
	rows, err := sqlstore.GetAllExisting(stateRoot, leaseHolderBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []issueops.LeaseHolderIndex{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]issueops.LeaseHolderIndex, 0, len(rows))
	for _, row := range rows {
		index, err := decodeLeaseHolderIndexRow(row)
		if err != nil {
			return nil, err
		}
		result = append(result, index)
	}
	return result, nil
}

func decodeLeaseHolderIndexRow(row port.RecordRow) (issueops.LeaseHolderIndex, error) {
	var index issueops.LeaseHolderIndex
	if err := json.Unmarshal(row.Data, &index); err != nil {
		return index, fmt.Errorf("decode active lease-holder index %s: %w", row.ID, err)
	}
	if !leaseHolderIndexFieldsValid(index) {
		return index, fmt.Errorf("active lease-holder index %s is invalid", row.ID)
	}
	if _, err := normalizeIssueOpsID(index.LifecycleID); err != nil {
		return index, fmt.Errorf("active lease-holder index %s lifecycle identity is invalid: %w", row.ID, err)
	}
	actor := issueops.NativeActor{Host: index.Host, SessionID: index.SessionID, AgentID: index.AgentID}
	if expected := leaseHolderIndexKey(actor); row.ID != expected {
		return index, fmt.Errorf("active lease-holder index %s key does not match its actor identity", row.ID)
	}
	index.Key = row.ID
	return index, nil
}

func leaseHolderIndexFieldsValid(index issueops.LeaseHolderIndex) bool {
	return index.SchemaVersion == issueops.SchemaVersion && index.Generation != 0 && strings.TrimSpace(index.LifecycleID) != "" &&
		leaseHolderHostKnown(index.Host) && strings.TrimSpace(index.SessionID) != ""
}

func leaseHolderHostKnown(host string) bool {
	switch host {
	case "codex", "claude", "omo", "omp", "opencode":
		return true
	}
	return false
}

func persistExecutionTransition(ctx context.Context, stateRoot string, record issueops.Record, previousHolder *issueops.NativeActor) (issueops.Record, error) {
	return persistExecutionTransitionWithMutations(ctx, stateRoot, record, previousHolder, nil)
}

func persistExecutionTransitionWithMutations(ctx context.Context, stateRoot string, record issueops.Record, previousHolder *issueops.NativeActor, extra []port.RecordMutation) (issueops.Record, error) {
	if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
		return issueops.Record{OK: false, ID: record.ID}, err
	}
	encoded, data, err := encodeIssueOpsRecord(record)
	if err != nil {
		return encoded, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.Record{OK: false, ID: record.ID}, err
	}
	raw, found, err := mutableIssueOpsRaw(db, encoded.ID)
	if err != nil {
		return issueops.Record{OK: false, ID: encoded.ID}, err
	}
	expected := []port.ExpectedRecord{}
	if found {
		expected = append(expected, port.ExpectedRecord{Bucket: issueOpsBucket, ID: encoded.ID, Data: raw})
	}
	mutations := []port.RecordMutation{{Bucket: issueOpsBucket, ID: encoded.ID, Data: data, RequireAbsent: !found}}
	if previousHolder != nil {
		mutation, err := leaseIndexDeleteMutation(db, encoded.ID, *previousHolder)
		if err != nil {
			return issueops.Record{OK: false, ID: encoded.ID}, err
		}
		if mutation != nil {
			mutations = append(mutations, *mutation)
		}
	}
	indexMutation, err := activeLeaseIndexMutation(db, encoded)
	if err != nil {
		return issueops.Record{OK: false, ID: encoded.ID}, err
	}
	if indexMutation != nil {
		mutations = append(mutations, *indexMutation)
	}
	mutations = append(mutations, extra...)
	if err := db.CompareAndApply(ctx, expected, mutations); err != nil {
		return issueops.Record{OK: false, ID: encoded.ID}, err
	}
	return encoded, nil
}

// activeLeaseIndexMutation은 활성 lease 홀더 인덱스를 새로 써야 할 때 그 변경을 돌려준다.
// 활성 lease가 아니거나 같은 lifecycle/generation 인덱스가 이미 있으면 nil이다.
func activeLeaseIndexMutation(db *sqlstore.DB, encoded issueops.Record) (*port.RecordMutation, error) {
	if encoded.Execution == nil || encoded.Execution.Lease.Status != issueops.LeaseStatusActive {
		return nil, nil
	}
	holder := encoded.Execution.Lease.Holder
	if holder == nil {
		return nil, fmt.Errorf("active execution lease is missing its holder")
	}
	alreadyCurrent, err := requireLeaseIndexAvailable(db, encoded.ID, encoded.Execution.Lease.Generation, *holder)
	if err != nil {
		return nil, err
	}
	index := issueops.LeaseHolderIndex{
		SchemaVersion: issueops.SchemaVersion,
		LifecycleID:   encoded.ID,
		Generation:    encoded.Execution.Lease.Generation,
		Host:          holder.Host,
		SessionID:     holder.SessionID,
		AgentID:       holder.AgentID,
	}
	indexData, err := json.Marshal(index)
	if err != nil {
		return nil, err
	}
	if alreadyCurrent {
		return nil, nil
	}
	return &port.RecordMutation{
		Bucket: leaseHolderBucket, ID: leaseHolderIndexKey(*holder), Data: indexData, RequireAbsent: true,
	}, nil
}

func requireLeaseIndexAvailable(db *sqlstore.DB, lifecycleID string, generation uint64, actor issueops.NativeActor) (bool, error) {
	data, ok, err := db.Get(leaseHolderBucket, leaseHolderIndexKey(actor))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	var existing issueops.LeaseHolderIndex
	if err := json.Unmarshal(data, &existing); err != nil {
		return false, fmt.Errorf("decode active lease-holder index: %w", err)
	}
	if existing.SchemaVersion != issueops.SchemaVersion {
		return false, fmt.Errorf("unsupported lease-holder index schema %d", existing.SchemaVersion)
	}
	if existing.LifecycleID == lifecycleID && existing.Generation == generation {
		return true, nil
	}
	return false, fmt.Errorf("native session already holds active lease %s generation %d; a session holds one lease at a time: release it first (issueops execution release --id %s --generation %d $ACTOR_FLAGS --json), then retry", existing.LifecycleID, existing.Generation, existing.LifecycleID, existing.Generation)
}

func leaseIndexDeleteMutation(db *sqlstore.DB, lifecycleID string, actor issueops.NativeActor) (*port.RecordMutation, error) {
	key := leaseHolderIndexKey(actor)
	data, ok, err := db.Get(leaseHolderBucket, key)
	if err != nil || !ok {
		return nil, err
	}
	var existing issueops.LeaseHolderIndex
	if err := json.Unmarshal(data, &existing); err != nil {
		return nil, fmt.Errorf("decode active lease-holder index: %w", err)
	}
	if existing.LifecycleID != lifecycleID {
		return nil, fmt.Errorf("refusing to delete another lifecycle's lease-holder index")
	}
	return &port.RecordMutation{Bucket: leaseHolderBucket, ID: key, Delete: true}, nil
}

func leaseHolderIndexKey(actor issueops.NativeActor) string {
	identity := strings.ToLower(strings.TrimSpace(actor.Host)) + "\x00" + strings.TrimSpace(actor.SessionID) + "\x00" + strings.TrimSpace(actor.AgentID)
	sum := sha256.Sum256([]byte(identity))
	return "lease-holder-" + hex.EncodeToString(sum[:])
}
