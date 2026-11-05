package selfverify

import (
	"time"

	selfaugmentapp "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
)

// SaveCandidateExportDeps injects the clock, snapshot encoder, state writer and state directory
// lookup used by SaveCandidateExport.
type SaveCandidateExportDeps struct {
	Now      func() time.Time
	Encode   func(contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error)
	Write    func(string, string) (state.Result, error)
	StateDir func() string
}

// SaveCandidateExport writes the candidate export to state under key (default
// self-verify-candidates-latest) and records the checkpoint on result.
func SaveCandidateExport(result *contract.SelfVerificationCandidateExportResult, key string, deps SaveCandidateExportDeps) error {
	if key == "" {
		key = "self-verify-candidates-latest"
	}
	snapshot := domain.NewCandidateExportSnapshot(*result, deps.Now().UTC())
	checkpoint, err := selfaugmentapp.SaveStateSnapshot(key, snapshot, deps.Encode, deps.Write, deps.StateDir)
	result.StateCheckpoint = checkpoint
	return err
}
