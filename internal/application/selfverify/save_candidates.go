package selfverify

import (
	"time"

	selfaugmentapp "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
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
	checkpoint, err := selfaugmentapp.SaveStateSnapshot(key, snapshot, deps.Encode, deps.Write, deps.StateDir)
	result.StateCheckpoint = checkpoint
	return err
}
