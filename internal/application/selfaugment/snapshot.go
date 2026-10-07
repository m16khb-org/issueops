package selfaugment

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/domain/failurecause"
	domain "issueops/internal/domain/selfaugment"
)

type SnapshotStore struct {
	ReadState    func(string) (statecontract.StateResult, error)
	NormalizeKey func(string) (string, error)
	WriteRecord  func(string, string, statecontract.RecordEnvelope) (string, error)
	Now          func() time.Time
}

func (store SnapshotStore) Read(key string) (contract.SelfAugmentStateSnapshot, error) {
	state, err := store.ReadState(key)
	if err != nil {
		return contract.SelfAugmentStateSnapshot{}, err
	}
	var snapshot contract.SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		return contract.SelfAugmentStateSnapshot{}, err
	}
	if err := domain.ValidateSummarySnapshot(key, snapshot); err != nil {
		return contract.SelfAugmentStateSnapshot{}, err
	}
	if err := validateSnapshotFailureCause(key, snapshot); err != nil {
		return contract.SelfAugmentStateSnapshot{}, err
	}
	return snapshot, nil
}

// validateSnapshotFailureCause는 저장된 failure cause가 Write가 기록하는
// 분류 결과와 정확히 같은지 확인한다. 읽기 경로는 값을 다시 계산해 덮어쓰지
// 않으므로, 분류 필드가 없거나 어긋난 레코드는 거부된다.
func validateSnapshotFailureCause(key string, snapshot contract.SelfAugmentStateSnapshot) error {
	want := failurecause.Classify(snapshot.Summary.FailedSteps > 0, snapshot.Summary.FailureCauseEvidence)
	if snapshot.Summary.FailureCause != want.Cause || snapshot.Summary.FailureCauseReason != want.Reason ||
		snapshot.Summary.FailureCauseEvidence == nil || !reflect.DeepEqual(snapshot.Summary.FailureCauseEvidence, want.Evidence) {
		return fmt.Errorf("state key %q has failure cause %q (%q), want %q (%q)", key,
			snapshot.Summary.FailureCause, snapshot.Summary.FailureCauseReason, want.Cause, want.Reason)
	}
	return nil
}

func NormalizeSnapshotFailureCause(snapshot *contract.SelfAugmentStateSnapshot) {
	classified := failurecause.Classify(snapshot.Summary.FailedSteps > 0, snapshot.Summary.FailureCauseEvidence)
	snapshot.Summary.FailureCause = classified.Cause
	snapshot.Summary.FailureCauseReason = classified.Reason
	snapshot.Summary.FailureCauseEvidence = classified.Evidence
}

func (store SnapshotStore) Write(dir, key string, snapshot contract.SelfAugmentStateSnapshot) error {
	NormalizeSnapshotFailureCause(&snapshot)
	key, err := store.NormalizeKey(key)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	record := statecontract.RecordEnvelope{
		SchemaVersion: statecontract.SchemaVersion,
		Key:           key,
		Content:       string(content),
		UpdatedAt:     store.Now().UTC().Format(time.RFC3339Nano),
		Bytes:         len(content),
	}
	// 잠금과 원자적 저장은 주입된 state writer가 수행한다.
	_, err = store.WriteRecord(dir, key, record)
	return err
}
