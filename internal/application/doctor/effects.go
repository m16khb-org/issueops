package doctor

import (
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	statecontract "issueops/internal/contract/state"
	doctordomain "issueops/internal/domain/doctor"
)

type Effects struct {
	NormalizeRoot      func(string) (string, error)
	StateDir           func() string
	StateDoctor        func() (statecontract.StateDoctorResult, error)
	ValidateLifecycle  func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error)
	ProjectDocs        func(string) doctordomain.ProjectDocsObservation
	RuntimeState       func(string) doctordomain.RuntimeStateObservation
	LoopContracts      func(string) doctordomain.LoopObservation
	PipeCapacity       func() (int, error)
	MCPGateways        func(string) doctordomain.GatewayObservation
	NativeIntegrations func(string) doctordomain.NativeObservation
	BinaryDrift        func(string) doctordomain.BinaryObservation
	Now                func() time.Time
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
