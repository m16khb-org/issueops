package selfaugment

import (
	"time"

	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
)

type SavePlanDeps struct {
	Now      func() time.Time
	Encode   func(contract.SelfAugmentPlanStateSnapshot) ([]byte, error)
	Write    func(string, string) (state.StateResult, error)
	StateDir func() string
}

func SavePlan(result *contract.SelfAugmentPlanResult, key string, deps SavePlanDeps) error {
	if key == "" {
		key = "self-augment-latest"
	}
	snapshot := domain.NewPlanSnapshot(*result, deps.Now().UTC())
	checkpoint, err := SaveStateSnapshot(key, snapshot, deps.Encode, deps.Write, deps.StateDir)
	result.StateCheckpoint = checkpoint
	return err
}

// SaveStateSnapshot encodes snapshot, writes it under key, and returns the
// checkpoint the self-augment and self-verify results report. An encode
// failure has no state directory yet; a write failure names the directory.
func SaveStateSnapshot[S any](key string, snapshot S, encode func(S) ([]byte, error), write func(string, string) (state.StateResult, error), stateDir func() string) (*contract.SelfAugmentStateCheckpoint, error) {
	b, err := encode(snapshot)
	if err != nil {
		return &contract.SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}, err
	}
	written, err := write(key, string(b))
	if err != nil {
		return &contract.SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: stateDir(), Error: err.Error()}, err
	}
	return &contract.SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      written.Record.Key,
		StateDir: written.StateDir,
		Path:     written.Path,
		Bytes:    written.Record.Bytes,
	}, nil
}
