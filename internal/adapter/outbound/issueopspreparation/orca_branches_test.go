package issueopspreparation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	preparationcontract "issueops/internal/contract/issueopspreparation"
	"issueops/internal/port"
)

type orcaScriptedProvider struct {
	probeResult   port.ExecutionOrcaProbeResult
	probeErr      error
	inspectResult port.ExecutionOrcaIntentInventory
	inspectErr    error
	invoked       bool
	invokeResult  port.ExecutionOrcaIntentReceipt
}

func (fake *orcaScriptedProvider) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	return fake.probeResult, fake.probeErr
}

func (fake *orcaScriptedProvider) InspectIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	return fake.inspectResult, fake.inspectErr
}

func (fake *orcaScriptedProvider) InvokeIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	fake.invoked = true
	return fake.invokeResult, nil
}

func TestOrcaProbeWrapsProviderErrorAndSkipsBranchCheckWhenNotReady(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("socket closed")
	failed, err := NewOrcaGateway(OrcaDependencies{Provisioner: &orcaScriptedProvider{probeResult: port.ExecutionOrcaProbeResult{Available: true, Code: "partial"}, probeErr: boom}}).Probe(ctx, preparationcontract.ProbeRequest{})
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "Orca probe failed") || !failed.Available || failed.Ready || failed.Code != "partial" {
		t.Fatalf("result=%+v err=%v", failed, err)
	}

	checked := false
	deps := func(result port.ExecutionOrcaProbeResult) OrcaDependencies {
		return OrcaDependencies{
			Provisioner: &orcaScriptedProvider{probeResult: result},
			ValidateProbe: func(context.Context, preparationcontract.ProbeRequest) (string, error) {
				checked = true
				return "", nil
			},
		}
	}
	for name, result := range map[string]port.ExecutionOrcaProbeResult{
		"unavailable": {Code: "orca_missing"},
		"not ready":   {Available: true, Code: "orca_unready"},
	} {
		checked = false
		got, err := NewOrcaGateway(deps(result)).Probe(ctx, preparationcontract.ProbeRequest{})
		if err != nil || checked || got.Ready || got.Code != result.Code {
			t.Fatalf("%s: result=%+v checked=%v err=%v", name, got, checked, err)
		}
	}
}

func TestOrcaProbeBranchPrecheckFailureDefaultsCodeAndRevokesReadiness(t *testing.T) {
	provider := &orcaScriptedProvider{probeResult: port.ExecutionOrcaProbeResult{Available: true, Ready: true, Code: "ready"}}
	boom := errors.New("branch check exploded")
	adapter := NewOrcaGateway(OrcaDependencies{
		Provisioner:   provider,
		ValidateProbe: func(context.Context, preparationcontract.ProbeRequest) (string, error) { return "", boom },
	})
	got, err := adapter.Probe(context.Background(), preparationcontract.ProbeRequest{})
	if !errors.Is(err, boom) || got.Ready || !got.Available || got.Code != "orca_branch_precheck_failed" {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	adapter = NewOrcaGateway(OrcaDependencies{Provisioner: provider})
	got, err = adapter.Probe(context.Background(), preparationcontract.ProbeRequest{})
	if err == nil || got.Ready || got.Code != "orca_branch_precheck_unavailable" {
		t.Fatalf("missing precheck: result=%+v err=%v", got, err)
	}
}

func TestOrcaGatewayFailsClosedWithoutProvisioner(t *testing.T) {
	request := preparationcontract.IntentRequest{}
	var nilAdapter *OrcaGatewayAdapter
	for name, adapter := range map[string]*OrcaGatewayAdapter{"nil adapter": nilAdapter, "nil provisioner": NewOrcaGateway(OrcaDependencies{})} {
		if got, err := adapter.Probe(context.Background(), preparationcontract.ProbeRequest{}); err == nil || got.Code != "orca_adapter_unavailable" {
			t.Fatalf("%s probe: %+v %v", name, got, err)
		}
		if _, err := adapter.Inspect(context.Background(), request); err == nil || !strings.Contains(err.Error(), "provisioner is unavailable") {
			t.Fatalf("%s inspect: %v", name, err)
		}
		if _, err := adapter.Invoke(context.Background(), request); err == nil || !strings.Contains(err.Error(), "provisioner is unavailable") {
			t.Fatalf("%s invoke: %v", name, err)
		}
	}
}

func TestOrcaInspectMapsInventoryFlagsAndPropagatesFailure(t *testing.T) {
	boom := errors.New("inspect failed")
	if _, err := NewOrcaGateway(OrcaDependencies{Provisioner: &orcaScriptedProvider{inspectErr: boom}}).Inspect(context.Background(), preparationcontract.IntentRequest{}); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	provider := &orcaScriptedProvider{inspectResult: port.ExecutionOrcaIntentInventory{
		AuthoritativeZero: true, ExactReplay: true,
		Candidates: []port.ExecutionOrcaIntentReceipt{{
			RunID: "run-1", RunBound: true, TerminalPTYID: "pty", TerminalHandle: "handle", TaskID: "task",
			Workspace: &port.ExecutionOrcaWorkspaceReceipt{
				Workspace: port.ExecutionWorkspaceReceipt{SourceRoot: "/repo", Root: "/wt", Branch: "b", BaseHead: "h", ParentWorktree: "/parent", Driver: "orca", Exists: true},
				RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt", WorktreeInstanceID: "inst",
			},
		}, {RunID: "run-2"}},
	}}
	inventory, err := NewOrcaGateway(OrcaDependencies{Provisioner: provider}).Inspect(context.Background(), preparationcontract.IntentRequest{})
	if err != nil || !inventory.AuthoritativeZero || !inventory.ExactReplay || len(inventory.Candidates) != 2 {
		t.Fatalf("inventory=%+v err=%v", inventory, err)
	}
	want := &preparationcontract.OrcaWorkspaceReceipt{
		Workspace: preparationcontract.WorkspaceReceipt{SourceRoot: "/repo", Root: "/wt", Branch: "b", BaseHead: "h", ParentWorktree: "/parent", Driver: "orca", Exists: true},
		RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt", WorktreeInstanceID: "inst",
	}
	first := inventory.Candidates[0]
	if !reflect.DeepEqual(first.Workspace, want) || !first.RunBound || first.TerminalHandle != "handle" || first.TerminalPTYID != "pty" || first.TaskID != "task" {
		t.Fatalf("candidate=%+v", first)
	}
	if inventory.Candidates[1].Workspace != nil || inventory.Candidates[1].PromptReceipt != nil {
		t.Fatalf("absent receipts must stay absent: %+v", inventory.Candidates[1])
	}
}

func TestOrcaInvokeRequiresHydratorForSealedLaunchAndSurfacesHydrationFailure(t *testing.T) {
	provider := &orcaScriptedProvider{}
	request := adapterIntentRequest()
	if _, err := NewOrcaGateway(OrcaDependencies{Provisioner: provider}).Invoke(context.Background(), request); err == nil || !strings.Contains(err.Error(), "launch hydrator is unavailable") {
		t.Fatalf("missing hydrator: %v", err)
	}
	boom := errors.New("prompt file changed")
	adapter := NewOrcaGateway(OrcaDependencies{
		Provisioner: provider,
		HydrateLaunch: func(context.Context, preparationcontract.IntentRequest) (preparationcontract.IntentRequest, error) {
			return preparationcontract.IntentRequest{}, boom
		},
	})
	if _, err := adapter.Invoke(context.Background(), request); !errors.Is(err, boom) {
		t.Fatalf("hydration error: %v", err)
	}
	if provider.invoked {
		t.Fatal("provider invoked although the sealed launch could not be hydrated")
	}
	request.Launch = nil
	if _, err := NewOrcaGateway(OrcaDependencies{Provisioner: provider}).Invoke(context.Background(), request); err != nil || !provider.invoked {
		t.Fatalf("stages without a launch need no hydrator: invoked=%v err=%v", provider.invoked, err)
	}
}

func TestOrcaIntentRequestMapsPreparedWorkspaceAndLaunch(t *testing.T) {
	request := adapterIntentRequest()
	request.Prepared.Workspace.ParentWorktree = "/parent"
	request.Prepared.Workspace.Exists = true
	request.RoleAgentArgs = []string{"--role", "owner"}
	got := OrcaIntentRequest(request)
	if got.Prepared == nil || got.Prepared.Workspace.ParentWorktree != "/parent" || !got.Prepared.Workspace.Exists ||
		got.Prepared.WorktreeInstanceID != "instance" || got.Prepared.RuntimeID != "runtime" {
		t.Fatalf("prepared=%+v", got.Prepared)
	}
	if got.Launch == nil || got.Launch.ContextPacketSHA256 != "packet-sha" || got.Launch.PromptSHA256 != "prompt-sha" ||
		!reflect.DeepEqual(got.RoleAgentArgs, request.RoleAgentArgs) || !got.RunBound || got.TaskID != "task" || got.Stage != port.ExecutionOrcaIntentStage(request.Stage) {
		t.Fatalf("request=%+v", got)
	}
	request.Prepared, request.Launch = nil, nil
	bare := OrcaIntentRequest(request)
	if bare.Prepared != nil || bare.Launch != nil {
		t.Fatalf("absent evidence must stay absent: %+v", bare)
	}
}
