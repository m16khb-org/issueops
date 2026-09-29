package issueopsapp

import (
	"issueops/internal/adapter/doctor"
	statestore "issueops/internal/adapter/outbound/state"
	contract "issueops/internal/contract/looprun"
	"path/filepath"
)

func configureDoctorLoopGate() {
	doctor.RepoGateSummaryFor = func(repo string) (contract.RepoGateSummary, []string) {
		return newLoopReader().RepoGateSummaryFor(repo)
	}
	doctor.LoopStateRoot = func() string { return filepath.Join(statestore.StateDir(), "loop") }
}
