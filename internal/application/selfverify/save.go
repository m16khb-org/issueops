package selfverify

import (
	"time"

	selfaugmentcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	failurecause "issueops/internal/domain/failurecause"
	selfaugmentdomain "issueops/internal/domain/selfaugment"
)

type SaveSummaryDeps struct {
	Now      func() time.Time
	Encode   func(selfaugmentcontract.SelfAugmentStateSnapshot) ([]byte, error)
	Write    func(string, string) (statecontract.StateResult, error)
	StateDir func() string
}

func SaveSummary(result *selfaugmentcontract.SelfAugmentResult, key string, deps SaveSummaryDeps) error {
	if key == "" {
		key = "self-verify-latest"
	}
	snapshot := selfaugmentdomain.NewSelfVerificationSummarySnapshot(*result, deps.Now().UTC())
	// SnapshotStore.Write와 같은 분류 결과를 기록해야 읽기 검증을 통과한다.
	cause := failurecause.Classify(snapshot.Summary.FailedSteps > 0, snapshot.Summary.FailureCauseEvidence)
	snapshot.Summary.FailureCause, snapshot.Summary.FailureCauseReason, snapshot.Summary.FailureCauseEvidence = cause.Cause, cause.Reason, cause.Evidence
	b, err := deps.Encode(snapshot)
	if err != nil {
		result.StateCheckpoint = &selfaugmentcontract.SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return err
	}
	state, err := deps.Write(key, string(b))
	if err != nil {
		result.StateCheckpoint = &selfaugmentcontract.SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: deps.StateDir(), Error: err.Error()}
		return err
	}
	result.StateCheckpoint = &selfaugmentcontract.SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	return nil
}
