package basiccli

import (
	"context"
	"os"
	"path/filepath"
	"time"

	doctoradapter "issueops/internal/adapter/doctor"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/repopath"
	doctorapp "issueops/internal/application/doctor"
	statecontract "issueops/internal/contract/state"
	doctordomain "issueops/internal/domain/doctor"
	"issueops/internal/domain/operationalhealth"
)

func testDoctorService() doctorapp.Service {
	stateDir := statestore.StateDir()
	lifecycle := testLifecycleService()
	loops := testLoopReader()
	gateways := doctoradapter.Gateways{Probe: doctoradapter.ProbeGatewayHTTP, CountFDs: doctoradapter.CountGatewayFDsViaLsof}
	return doctorapp.Service{Effects: doctorapp.Effects{
		NormalizeRoot: repopath.NormalizeRoot, StateDir: func() string { return stateDir },
		StateDoctor: func() (statecontract.StateDoctorResult, error) { return statestore.Doctor(stateDir) }, ValidateLifecycle: lifecycle.Resolve,
		ProjectDocs: doctoradapter.ObserveProjectDocs, RuntimeState: doctoradapter.ObserveRuntimeState,
		LoopContracts: func(root string) doctordomain.LoopObservation {
			summary, warnings := loops.RepoGateSummaryFor(root)
			return doctordomain.LoopObservation{Active: summary.Active, Exhausted: summary.Exhausted, Warnings: warnings, StateRoot: filepath.Join(stateDir, "loop")}
		},
		PipeCapacity: doctoradapter.MeasurePipeCapacity, MCPGateways: gateways.Observe, NativeIntegrations: doctoradapter.ObserveNativeIntegrations, BinaryDrift: doctoradapter.ObserveBinaryDrift, Now: time.Now,
	}}
}

var testOperationalCollector = func(_ context.Context, repo string) operationalhealth.Snapshot {
	return healthyCLIOperationalSnapshot(repo)
}

func testDoctorCommand() Doctor {
	home, _ := os.UserHomeDir()
	return Doctor{Service: testDoctorService(), NormalizeRepoRoot: repopath.NormalizeRoot, IssueOpsRoot: testIssueOpsRoot(), Home: home, Version: "0.1.0", Now: time.Now, CheckDaemonStatus: testDaemonReader().Run, CollectOperationalHealth: testOperationalCollector}
}
func testRunDoctor(args []string) error { return testDoctorCommand().Run(args) }
