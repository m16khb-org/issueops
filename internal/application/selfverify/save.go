package selfverify

import (
	"time"

	selfaugmentcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	failurecause "issueops/internal/domain/failurecause"
	selfaugmentdomain "issueops/internal/domain/selfaugment"
)

// SaveSummaryDeps injects the clock, snapshot encoder, state writer and state directory lookup used
// by SaveSummary.
type SaveSummaryDeps struct {
	Now      func() time.Time
	Encode   func(selfaugmentcontract.StateSnapshot) ([]byte, error)
	Write    func(string, string) (statecontract.Result, error)
	StateDir func() string
}

// SaveSummary writes the result summary to state as a snapshot under key (default
// self-verify-latest) and records the outcome in result.StateCheckpoint. It classifies the failure
// cause as the snapshot store does so the stored snapshot passes read validation.
func SaveSummary(result *selfaugmentcontract.Result, key string, deps SaveSummaryDeps) error {
	if key == "" {
		key = "self-verify-latest"
	}
	snapshot := selfaugmentdomain.NewSelfVerificationSummarySnapshot(*result, deps.Now().UTC())
	// SnapshotStore.Write와 같은 분류 결과를 기록해야 읽기 검증을 통과한다.
	cause := failurecause.Classify(snapshot.Summary.FailedSteps > 0, snapshot.Summary.FailureCauseEvidence)
	snapshot.Summary.FailureCause, snapshot.Summary.FailureCauseReason, snapshot.Summary.FailureCauseEvidence = cause.Cause, cause.Reason, cause.Evidence
	b, err := deps.Encode(snapshot)
	if err != nil {
		result.StateCheckpoint = &selfaugmentcontract.StateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return err
	}
	state, err := deps.Write(key, string(b))
	if err != nil {
		result.StateCheckpoint = &selfaugmentcontract.StateCheckpoint{OK: false, Key: key, StateDir: deps.StateDir(), Error: err.Error()}
		return err
	}
	result.StateCheckpoint = &selfaugmentcontract.StateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	return nil
}
