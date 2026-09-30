package issueopsapp

import (
	"os"
	"path/filepath"
	"time"

	"issueops/cmd/issueops/basiccli"
	adapter "issueops/internal/adapter/doctor"

	"issueops/internal/adapter/operationalhealth"
	"issueops/internal/adapter/orca"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/doctor"
	statecontract "issueops/internal/contract/state"
	domain "issueops/internal/domain/doctor"
)

func newDoctorService() app.Service {
	stateDir := statestore.StateDir()
	lifecycle := newProjectLifecycleService()
	loops := newLoopReader()
	gateways := adapter.Gateways{Probe: adapter.ProbeGatewayHTTP, CountFDs: adapter.CountGatewayFDsViaLsof}
	return app.Service{Effects: app.Effects{
		NormalizeRoot: newRepoRootResolver("."), StateDir: func() string { return stateDir },
		StateDoctor:       func() (statecontract.StateDoctorResult, error) { return statestore.Doctor(stateDir) },
		ValidateLifecycle: lifecycle.Resolve,
		ProjectDocs:       adapter.ObserveProjectDocs, RuntimeState: adapter.ObserveRuntimeState,
		LoopContracts: func(root string) domain.LoopObservation {
			summary, warnings := loops.RepoGateSummaryFor(root)
			return domain.LoopObservation{Active: summary.Active, Exhausted: summary.Exhausted, Warnings: warnings, StateRoot: filepath.Join(stateDir, "loop")}
		},
		PipeCapacity: adapter.MeasurePipeCapacity, MCPGateways: gateways.Observe,
		NativeIntegrations: adapter.ObserveNativeIntegrations, BinaryDrift: adapter.ObserveBinaryDrift, Now: time.Now,
	}}
}

func newDoctorCommand() basiccli.Doctor {
	home, _ := os.UserHomeDir()
	collector := newOperationalHealthCollector(issueOpsStateRoot(), operationalhealth.ExecGitRunner{}, orca.New())
	return basiccli.Doctor{Service: newDoctorService(), NormalizeRepoRoot: newRepoRootResolver("."),
		IssueOpsRoot: issueOpsRoot(), Home: home, Version: version, Now: time.Now,
		CollectOperationalHealth: collector.Collect, CheckDaemonStatus: newDaemonReader().Run}
}
