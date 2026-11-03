package basiccli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	doctoradapter "issueops/internal/adapter/doctor"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/repopath"
	doctorapp "issueops/internal/application/doctor"
	statecontract "issueops/internal/contract/state"
	doctordomain "issueops/internal/domain/doctor"
	"issueops/internal/domain/operationalhealth"
)

// TestMain gives the package a scratch HOME. Doctor probes every loopback MCP
// gateway in ~/.claude.json and counts its FDs with lsof; against a developer's
// real HOME that made the package spend seconds of CPU per doctor call and tie
// results to the machine.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "issueops-basiccli-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Setenv("HOME", home); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

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
	return Doctor{Service: testDoctorService(), NormalizeRepoRoot: repopath.NormalizeRoot, IssueOpsRoot: testIssueOpsRoot(), Home: home, Version: "0.1.0", Now: time.Now, CollectOperationalHealth: testOperationalCollector}
}
func testRunDoctor(args []string) error { return testDoctorCommand().Run(args) }
