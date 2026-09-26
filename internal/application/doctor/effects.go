package doctor

import (
	"strings"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	statecontract "issueops/internal/contract/state"
)

type Effects struct {
	NormalizeRoot              func(string) (string, error)
	StateDir                   func() string
	StateDoctor                func() (statecontract.StateDoctorResult, error)
	ValidateLifecycle          func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error)
	CheckProjectDocs           func(*HarnessDoctorResult, string)
	CheckRepoLocalRuntimeState func(*HarnessDoctorResult, string)
	CheckLoopContracts         func(*HarnessDoctorResult, string)
	CheckPipeCapacity          func(*HarnessDoctorResult)
	CheckMCPGateways           func(*HarnessDoctorResult, string)
	CheckNativeIntegrations    func(*HarnessDoctorResult, string)
	CheckBinaryDrift           func(*HarnessDoctorResult, string)
	Now                        func() time.Time
}

type Service struct{ Effects Effects }

func severityRank(severity string) int {
	switch severity {
	case "error":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
