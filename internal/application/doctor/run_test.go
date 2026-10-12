package doctor

import (
	"errors"
	"reflect"
	"testing"
	"time"

	doctorcontract "issueops/internal/contract/doctor"
	lifecyclecontract "issueops/internal/contract/lifecycle"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	statecontract "issueops/internal/contract/state"
	doctordomain "issueops/internal/domain/doctor"
	"issueops/internal/domain/operationalhealth"
)

func baseService(stateDoctor func() (statecontract.StateDoctorResult, error)) Service {
	return Service{Effects: Effects{
		NormalizeRoot: func(root string) (string, error) { return root, nil },
		StateDir:      func() string { return "/state" },
		StateDoctor:   stateDoctor,
		ValidateLifecycle: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
			return lifecyclecontract.ProjectLifecycleStatePlan{Exists: true, NamespaceValid: true}, nil
		},
		ProjectDocs:        func(string) doctordomain.ProjectDocsObservation { return doctordomain.ProjectDocsObservation{} },
		RuntimeState:       func(string) doctordomain.RuntimeStateObservation { return doctordomain.RuntimeStateObservation{} },
		LoopContracts:      func(string) doctordomain.LoopObservation { return doctordomain.LoopObservation{} },
		PipeCapacity:       func() (int, error) { return 0, nil },
		MCPGateways:        func(string) doctordomain.GatewayObservation { return doctordomain.GatewayObservation{} },
		NativeIntegrations: func(string) doctordomain.NativeObservation { return doctordomain.NativeObservation{} },
		BinaryDrift:        func(string) doctordomain.BinaryObservation { return doctordomain.BinaryObservation{} },
		Now:                func() time.Time { return time.Date(2026, 9, 29, 1, 2, 3, 0, time.UTC) },
	}}
}

func TestRunReportsNormalizeRootFailureWithStateDir(t *testing.T) {
	service := baseService(nil)
	service.Effects.NormalizeRoot = func(string) (string, error) { return "", errors.New("bad root") }
	result, err := service.Run(doctorcontract.HarnessDoctorRequest{RepoRoot: "x"})
	if err == nil || err.Error() != "bad root" || result.OK || result.Kind != "harness_doctor" || result.StateDir != "/state" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestRunTurnsStateDoctorFailureIntoErrorIssue(t *testing.T) {
	service := baseService(func() (statecontract.StateDoctorResult, error) {
		return statecontract.StateDoctorResult{}, errors.New("denied")
	})
	result, err := service.Run(doctorcontract.HarnessDoctorRequest{RepoRoot: "/repo", StaticOnly: true})
	if err != nil || !result.OK || result.Healthy {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var found *doctorcontract.HarnessDoctorIssue
	for i := range result.Issues {
		if result.Issues[i].Code == "state_doctor_error" {
			found = &result.Issues[i]
		}
	}
	if found == nil || found.Severity != "error" || found.Path != "/state" || found.Fix == nil {
		t.Fatalf("issues=%+v", result.Issues)
	}
	for _, check := range result.Checks {
		if check.Name == "state_store" {
			t.Fatalf("state_store check recorded despite failure: %+v", result.Checks)
		}
	}
	if result.GeneratedAt != "2026-09-29T01:02:03Z" {
		t.Fatalf("generated_at=%q", result.GeneratedAt)
	}
}

func TestRunSortsIssuesBySeverityThenCodeThenPath(t *testing.T) {
	service := baseService(func() (statecontract.StateDoctorResult, error) {
		return statecontract.StateDoctorResult{Healthy: false, StateDir: "/state", Issues: []statecontract.StateDoctorIssue{
			{Code: "zeta", Severity: "info", Path: "/a", Message: "m"},
			{Code: "beta", Severity: "warning", Path: "/z", Message: "m"},
			{Code: "beta", Severity: "warning", Path: "/b", Message: "m"},
			{Code: "alpha", Severity: "error", Path: "/a", Message: "m"},
		}}, nil
	})
	result, err := service.Run(doctorcontract.HarnessDoctorRequest{RepoRoot: "/repo", StaticOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, issue := range result.Issues {
		got = append(got, issue.Severity+":"+issue.Code+":"+issue.Path)
	}
	want := []string{"error:state_alpha:/a", "warning:state_beta:/b", "warning:state_beta:/z", "info:state_zeta:/a"}
	// Other (non-state) issues may exist; compare only the state ones in order.
	filtered := []string{}
	for _, entry := range got {
		for _, w := range want {
			if entry == w {
				filtered = append(filtered, entry)
			}
		}
	}
	if !reflect.DeepEqual(filtered, want) || result.Healthy {
		t.Fatalf("got=%v healthy=%t", got, result.Healthy)
	}
}

func TestRunProjectsUnexpectedStateArtifactsAsResidueWithoutMutatingCallerSnapshot(t *testing.T) {
	service := baseService(func() (statecontract.StateDoctorResult, error) {
		return statecontract.StateDoctorResult{Healthy: true, StateDir: "/state", Issues: []statecontract.StateDoctorIssue{
			{Code: "unexpected_file", Severity: "warning", Path: "/state/stray", Message: "stray"},
		}}, nil
	})
	artifacts := make([]operationalhealthcontract.StateArtifact, 1, 4)
	artifacts[0] = operationalhealthcontract.StateArtifact{Path: "/state/seed", Code: "unexpected_file"}
	snapshot := operationalhealth.Snapshot{StateArtifacts: artifacts}
	result, err := service.Run(doctorcontract.HarnessDoctorRequest{RepoRoot: "/repo", StaticOnly: true, OperationalSnapshot: &snapshot})
	if err != nil {
		t.Fatal(err)
	}
	residue := false
	for _, issue := range result.Issues {
		if issue.Code == "state_unexpected_file" {
			t.Fatalf("unexpected artifact reported as raw state issue: %+v", issue)
		}
		if issue.Code == operationalhealthcontract.FindingStateArtifactResidue && issue.Path == "/state/stray" {
			residue = true
		}
	}
	if !residue {
		t.Fatalf("residue issue for /state/stray missing: %+v", result.Issues)
	}
	if len(snapshot.StateArtifacts) != 1 || !reflect.DeepEqual(artifacts[:4][1], operationalhealthcontract.StateArtifact{}) {
		t.Fatalf("caller snapshot mutated: %+v backing=%+v", snapshot.StateArtifacts, artifacts[:4])
	}
}

func TestRunReportsLifecycleStateOnlyWhenValidationSucceeds(t *testing.T) {
	plan := lifecyclecontract.ProjectLifecycleStatePlan{Exists: true, NamespaceValid: true, RepoID: "repo-1"}
	ok := baseService(func() (statecontract.StateDoctorResult, error) {
		return statecontract.StateDoctorResult{Healthy: true}, nil
	})
	ok.Effects.ValidateLifecycle = func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return plan, nil }
	result, err := ok.Run(doctorcontract.HarnessDoctorRequest{RepoRoot: "/repo", StaticOnly: true})
	if err != nil || result.LifecycleState.RepoID != "repo-1" {
		t.Fatalf("lifecycle=%+v err=%v", result.LifecycleState, err)
	}

	failing := baseService(func() (statecontract.StateDoctorResult, error) {
		return statecontract.StateDoctorResult{Healthy: true}, nil
	})
	failing.Effects.ValidateLifecycle = func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
		return plan, errors.New("lifecycle invalid")
	}
	result, err = failing.Run(doctorcontract.HarnessDoctorRequest{RepoRoot: "/repo", StaticOnly: true})
	if err != nil || !reflect.DeepEqual(result.LifecycleState, lifecyclecontract.ProjectLifecycleStatePlan{}) {
		t.Fatalf("lifecycle must stay empty on validation error: %+v err=%v", result.LifecycleState, err)
	}
}

func TestRunPrefixesOperationalFindingSummaryWithResourceIdentity(t *testing.T) {
	service := baseService(func() (statecontract.StateDoctorResult, error) {
		return statecontract.StateDoctorResult{Healthy: true, StateDir: "/state"}, nil
	})
	snapshot := operationalhealth.Snapshot{
		StateArtifacts: []operationalhealthcontract.StateArtifact{{Path: "/state/stray", Code: "unexpected_file"}},
		InventoryProblems: []operationalhealthcontract.InventoryProblem{
			{Code: "probe_failed", Detail: "boom"},
		},
	}
	result, err := service.Run(doctorcontract.HarnessDoctorRequest{
		RepoRoot: "/repo", StaticOnly: true, OperationalSnapshot: &snapshot,
		OperationalOptions: operationalhealth.Options{Now: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatal(err)
	}
	summaries := map[string]string{}
	inventory := map[string]bool{}
	for _, issue := range result.Issues {
		summaries[issue.Code+"|"+issue.Path] = issue.Summary
		if issue.Code == operationalhealthcontract.FindingInventoryUnknown {
			inventory[issue.Summary] = true
		}
	}
	if got, want := summaries[operationalhealthcontract.FindingStateArtifactResidue+"|/state/stray"], "state_artifact /state/stray: state directory contains an unexpected runtime or recovery artifact"; got != want {
		t.Fatalf("residue summary=%q want %q", got, want)
	}
	if want := "resource probe_failed: boom"; !inventory[want] {
		t.Fatalf("blank resource kind must read %q, got %v", want, inventory)
	}
}
