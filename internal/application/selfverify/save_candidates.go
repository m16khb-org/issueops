package selfverify

import (
	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
	"time"
)

type SaveCandidateExportDeps struct {
	Now      func() time.Time
	Encode   func(contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error)
	Write    func(string, string) (state.StateResult, error)
	StateDir func() string
}

func SaveCandidateExport(result *contract.SelfVerificationCandidateExportResult, key string, deps SaveCandidateExportDeps) error {
	if key == "" {
		key = "self-verify-candidates-latest"
	}
	snapshot := domain.NewCandidateExportSnapshot(*result, deps.Now().UTC())
	b, err := deps.Encode(snapshot)
	if err != nil {
		result.StateCheckpoint = &contract.SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return err
	}
	state, err := deps.Write(key, string(b))
	if err != nil {
		result.StateCheckpoint = &contract.SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: deps.StateDir(), Error: err.Error()}
		return err
	}
	result.StateCheckpoint = &contract.SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	return nil
}
