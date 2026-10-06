package doctor

import (
	doctorcontract "issueops/internal/contract/doctor"
	"path/filepath"
	"time"

	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/repopath"
	doctorapp "issueops/internal/application/doctor"
	statecontract "issueops/internal/contract/state"
	doctordomain "issueops/internal/domain/doctor"
)

func testDoctorService() doctorapp.Service {
	stateDir := statestore.StateDir()
	lifecycle := testLifecycleService()
	loops := testLoopReader()
	gateways := Gateways{Probe: probeMCPGateway, CountFDs: countMCPGatewayFDs}
	return doctorapp.Service{Effects: doctorapp.Effects{
		NormalizeRoot: repopath.NormalizeRoot, StateDir: func() string { return stateDir },
		StateDoctor: func() (statecontract.StateDoctorResult, error) { return statestore.Doctor(stateDir) }, ValidateLifecycle: lifecycle.Resolve,
		ProjectDocs: ObserveProjectDocs, RuntimeState: ObserveRuntimeState,
		LoopContracts: func(root string) doctordomain.LoopObservation {
			summary, warnings := loops.RepoGateSummaryFor(root)
			return doctordomain.LoopObservation{Active: summary.Active, Exhausted: summary.Exhausted, Warnings: warnings, StateRoot: filepath.Join(stateDir, "loop")}
		},
		PipeCapacity: measurePipeCapacity, MCPGateways: gateways.Observe, NativeIntegrations: ObserveNativeIntegrations, BinaryDrift: ObserveBinaryDrift, Now: time.Now,
	}}
}

var measurePipeCapacity = MeasurePipeCapacity
var probeMCPGateway = ProbeGatewayHTTP
var countMCPGatewayFDs = CountGatewayFDsViaLsof

func HarnessDoctor(req doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error) {
	return testDoctorService().Run(req)
}
