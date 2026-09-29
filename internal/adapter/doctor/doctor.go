package doctor

import (
	doctorapp "issueops/internal/application/doctor"
	"time"
)

func HarnessDoctor(req HarnessDoctorRequest) (HarnessDoctorResult, error) {
	return (doctorapp.Service{Effects: doctorapp.Effects{
		NormalizeRoot:      NormalizeRepoRoot,
		StateDir:           StateDir,
		StateDoctor:        StateDoctor,
		ValidateLifecycle:  ValidateProjectLifecycleState,
		ProjectDocs:        observeProjectDocs,
		RuntimeState:       observeRuntimeState,
		LoopContracts:      observeLoopContracts,
		PipeCapacity:       measurePipeCapacity,
		MCPGateways:        observeMCPGateways,
		NativeIntegrations: observeNativeIntegrations,
		BinaryDrift:        observeBinaryDrift,
		Now:                time.Now,
	}}).Run(req)
}
