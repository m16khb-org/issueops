package issueopsapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	auditadapter "issueops/internal/adapter/audit"
	issueopsadapter "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestHandoffDeliveryRequestShowRecoveryStatesAndIdentity(t *testing.T) {
	requestID := "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name        string
		observation port.OrcaRequestObservation
		wantReplay  bool
		wantErr     bool
	}{
		{name: "completed", observation: port.OrcaRequestObservation{RuntimeID: "runtime-1", RequestID: requestID, Status: "completed", Method: "orchestration.dispatch"}, wantReplay: true},
		{name: "pending", observation: port.OrcaRequestObservation{RuntimeID: "runtime-1", RequestID: requestID, Status: "pending", Method: "orchestration.dispatch"}, wantReplay: true},
		{name: "absent", observation: port.OrcaRequestObservation{RuntimeID: "runtime-1", RequestID: requestID, Status: "absent", Method: "orchestration.dispatch"}, wantErr: true},
		{name: "request mismatch", observation: port.OrcaRequestObservation{RuntimeID: "runtime-1", RequestID: "other", Status: "completed", Method: "orchestration.dispatch"}, wantErr: true},
		{name: "method mismatch", observation: port.OrcaRequestObservation{RuntimeID: "runtime-1", RequestID: requestID, Status: "completed", Method: "orchestration.taskCreate"}, wantErr: true},
		{name: "runtime mismatch", observation: port.OrcaRequestObservation{RuntimeID: "runtime-other", RequestID: requestID, Status: "completed", Method: "orchestration.dispatch"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			request := handoffDeliveryRequestFixture("codex")
			request.RetryRequestID = requestID
			identity := handoffDeliveryIdentityFixture()
			if err := observeHandoffDeliveryFailure(stateRoot, request, identity, &port.OrcaError{Code: "response_lost", Invoked: true, OrchestrationRequestID: requestID, CallPhase: "orca_dispatch"}, time.Now); err != nil {
				t.Fatal(err)
			}
			fake := &handoffDeliveryProvisionerFake{requestObservation: test.observation}
			got, err := newHandoffDeliveryProvisioner(stateRoot, fake, time.Now).InspectIntent(context.Background(), request)
			if (err != nil) != test.wantErr {
				t.Fatalf("InspectIntent err=%v wantErr=%v inventory=%+v", err, test.wantErr, got)
			}
			if err == nil && got.ExactReplay != test.wantReplay {
				t.Fatalf("exact replay=%v want=%v inventory=%+v", got.ExactReplay, test.wantReplay, got)
			}
		})
	}
}

func TestHandoffDeliveryRecoveryRejectsRetryIDWithDifferentPayload(t *testing.T) {
	stateRoot := t.TempDir()
	requestID := "11111111-1111-4111-8111-111111111111"
	original := handoffDeliveryRequestFixture("codex")
	identity := handoffDeliveryIdentityFixture()
	if err := observeHandoffDeliveryFailure(stateRoot, original, identity, &port.OrcaError{Code: "response_lost", Invoked: true, OrchestrationRequestID: requestID, CallPhase: "orca_dispatch"}, time.Now); err != nil {
		t.Fatal(err)
	}
	different := original
	different.Launch = &port.ExecutionOrcaLaunchRequest{
		Prompt: "different", PromptPath: original.Launch.PromptPath, PromptSHA256: strings.Repeat("e", 64),
		ContextPacketPath: original.Launch.ContextPacketPath, ContextPacketSHA256: original.Launch.ContextPacketSHA256,
	}
	different.RetryRequestID = requestID
	fake := &handoffDeliveryProvisionerFake{requestObservation: port.OrcaRequestObservation{RuntimeID: "runtime-1", RequestID: requestID, Status: "completed", Method: "orchestration.dispatch"}}
	if _, err := newHandoffDeliveryProvisioner(stateRoot, fake, time.Now).InspectIntent(context.Background(), different); err == nil {
		t.Fatal("retry ID was accepted for a different prompt payload")
	}
}

func TestHandoffDeliveryOmoDurablePromptReceiptRecoversCandidateWithoutExternalWork(t *testing.T) {
	stateRoot := t.TempDir()
	request := handoffDeliveryRequestFixture("omo")
	identity := handoffDeliveryIdentityFixture()
	dispatchID := "11111111-1111-4111-8111-111111111111"
	promptID := "22222222-2222-4222-8222-222222222222"
	receipt := port.ExecutionOrcaIntentReceipt{
		TerminalPTYID: "pty-1", TerminalHandle: "term-1", TaskID: "task-1", DispatchID: "dispatch-1", RequestID: dispatchID,
		PromptReceipt: &port.OrcaPromptReceipt{RequestID: promptID, Stages: []string{"input_accepted", "turn_started"}, Provider: "omo", ProcessIncarnation: "process-1"},
	}
	if err := observeHandoffDeliveryCompleted(stateRoot, request, identity, "dispatch", receipt, time.Now); err != nil {
		t.Fatal(err)
	}
	if err := observeHandoffDeliveryCompleted(stateRoot, request, identity, "prompt", receipt, time.Now); err != nil {
		t.Fatal(err)
	}
	fake := &handoffDeliveryProvisionerFake{receipt: receipt, requestObservation: port.OrcaRequestObservation{RuntimeID: "runtime-1", RequestID: dispatchID, Status: "completed", Method: "orchestration.dispatch"}}
	inventory, err := newHandoffDeliveryProvisioner(stateRoot, fake, time.Now).InspectIntent(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Candidates) != 1 || inventory.Candidates[0].PromptReceipt == nil || inventory.Candidates[0].PromptReceipt.RequestID != promptID || fake.invokeCalls != 0 {
		t.Fatalf("durable prompt recovery inventory=%+v external calls=%d", inventory, fake.invokeCalls)
	}
}

func TestHandoffDeliveryFreshOmoDelegatesWithoutDurableDispatchInspection(t *testing.T) {
	stateRoot := t.TempDir()
	request := handoffDeliveryRequestFixture("omo")
	fake := &handoffDeliveryFreshOmoFake{}

	inventory, err := newHandoffDeliveryProvisioner(stateRoot, fake, time.Now).InspectIntent(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !inventory.AuthoritativeZero || fake.dispatchInspections != 0 || fake.inventoryInspections != 1 {
		t.Fatalf("fresh Omo inventory=%+v dispatch inspections=%d inventory inspections=%d", inventory, fake.dispatchInspections, fake.inventoryInspections)
	}
}

func TestHandoffDeliveryOmoCrashBetweenCallsReplaysDispatchIDAndStartsPromptOnce(t *testing.T) {
	stateRoot := t.TempDir()
	request := handoffDeliveryRequestFixture("omo")
	identity := handoffDeliveryIdentityFixture()
	dispatchID := "11111111-1111-4111-8111-111111111111"
	dispatchReceipt := port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-1", TerminalHandle: "term-1", TaskID: "task-1", DispatchID: "dispatch-1", RequestID: dispatchID}
	if err := observeHandoffDeliveryCompleted(stateRoot, request, identity, "dispatch", dispatchReceipt, time.Now); err != nil {
		t.Fatal(err)
	}
	fake := &handoffDeliveryProvisionerFake{
		receipt:            dispatchReceipt,
		requestObservation: port.OrcaRequestObservation{RuntimeID: "runtime-1", RequestID: dispatchID, Status: "completed", Method: "orchestration.dispatch"},
	}
	observed := newHandoffDeliveryProvisioner(stateRoot, fake, time.Now)
	inventory, err := observed.InspectIntent(context.Background(), request)
	if err != nil || !inventory.ExactReplay {
		t.Fatalf("crash recovery inventory=%+v err=%v", inventory, err)
	}
	promptID := "22222222-2222-4222-8222-222222222222"
	fake.receipt.PromptReceipt = &port.OrcaPromptReceipt{RequestID: promptID, Stages: []string{"input_accepted"}, Provider: "omo", ProcessIncarnation: "process-1"}
	if _, err := observed.InvokeIntent(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if fake.invokeCalls != 1 || fake.lastInvokeRequest.RetryRequestID != dispatchID || fake.lastInvokeRequest.PromptRetryRequestID != "" {
		t.Fatalf("recovery invocation calls=%d dispatch retry=%q prompt retry=%q", fake.invokeCalls, fake.lastInvokeRequest.RetryRequestID, fake.lastInvokeRequest.PromptRetryRequestID)
	}
}

func TestHandoffDeliveryOmoPromptResponseLossReplaysBothExactIDs(t *testing.T) {
	stateRoot := t.TempDir()
	request := handoffDeliveryRequestFixture("omo")
	identity := handoffDeliveryIdentityFixture()
	dispatchID := "11111111-1111-4111-8111-111111111111"
	promptID := "22222222-2222-4222-8222-222222222222"
	if err := observeHandoffDeliveryFailure(stateRoot, request, identity, &port.OrcaError{Code: "response_lost", Invoked: true, DispatchRequestID: dispatchID, OrchestrationRequestID: promptID, CallPhase: "terminal_send"}, time.Now); err != nil {
		t.Fatal(err)
	}
	fake := &handoffDeliveryProvisionerFake{
		receipt:            port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-1", TerminalHandle: "term-1", TaskID: "task-1", DispatchID: "dispatch-1", RequestID: dispatchID, PromptReceipt: &port.OrcaPromptReceipt{RequestID: promptID, Stages: []string{"input_accepted"}, Provider: "omo", ProcessIncarnation: "process-1"}},
		requestObservation: port.OrcaRequestObservation{RuntimeID: "runtime-1", RequestID: dispatchID, Status: "completed", Method: "orchestration.dispatch"},
	}
	observed := newHandoffDeliveryProvisioner(stateRoot, fake, time.Now)
	inventory, err := observed.InspectIntent(context.Background(), request)
	if err != nil || !inventory.ExactReplay {
		t.Fatalf("prompt response-loss inventory=%+v err=%v", inventory, err)
	}
	if _, err := observed.InvokeIntent(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if fake.lastInvokeRequest.RetryRequestID != dispatchID || fake.lastInvokeRequest.PromptRetryRequestID != promptID {
		t.Fatalf("exact replay IDs dispatch=%q prompt=%q", fake.lastInvokeRequest.RetryRequestID, fake.lastInvokeRequest.PromptRetryRequestID)
	}
}

func TestHandoffDeliveryOmoAcceptedPromptCrashWithoutDurableIDDoesNotSendAgain(t *testing.T) {
	stateRoot := t.TempDir()
	request := handoffDeliveryRequestFixture("omo")
	dispatchID := "11111111-1111-4111-8111-111111111111"
	fake := &handoffDeliveryPromptCrashFake{
		handoffDeliveryProvisionerFake: handoffDeliveryProvisionerFake{
			receipt: port.ExecutionOrcaIntentReceipt{
				TerminalPTYID: "pty-1", TerminalHandle: "term-1", TaskID: "task-1",
				DispatchID: "dispatch-1", RequestID: dispatchID,
			},
			requestObservation: port.OrcaRequestObservation{
				RuntimeID: "runtime-1", RequestID: dispatchID, Status: "completed", Method: "orchestration.dispatch",
			},
		},
	}
	observed := newHandoffDeliveryProvisioner(stateRoot, fake, time.Now)

	if _, err := observed.InvokeIntent(context.Background(), request); err == nil {
		t.Fatal("injected process crash after accepted terminal send was lost")
	}
	if fake.promptCalls != 1 {
		t.Fatalf("initial prompt calls=%d want=1", fake.promptCalls)
	}

	inventory, err := observed.InspectIntent(context.Background(), request)
	if err == nil && inventory.ExactReplay {
		_, _ = observed.InvokeIntent(context.Background(), request)
	}
	if err == nil {
		t.Fatalf("staged prompt without a durable UUID was classified for recovery: %+v", inventory)
	}
	if fake.promptCalls != 1 {
		t.Fatalf("ambiguous prompt was sent again: calls=%d", fake.promptCalls)
	}
}

func TestHandoffDeliveryDispatchCandidateRequiresExactRequestAndTerminalIdentity(t *testing.T) {
	const dispatchID = "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name    string
		host    string
		receipt port.ExecutionOrcaIntentReceipt
	}{
		{
			name: "codex different request UUID", host: "codex",
			receipt: port.ExecutionOrcaIntentReceipt{TaskID: "task-1", DispatchID: "dispatch-1", RequestID: "99999999-9999-4999-8999-999999999999", TerminalPTYID: "pty-1", TerminalHandle: "term-1"},
		},
		{
			name: "claude different assignee", host: "claude",
			receipt: port.ExecutionOrcaIntentReceipt{TaskID: "task-1", DispatchID: "dispatch-1", RequestID: dispatchID, TerminalPTYID: "pty-1", TerminalHandle: "term-other"},
		},
		{
			name: "omo dual launcher other terminal", host: "omo",
			receipt: port.ExecutionOrcaIntentReceipt{TaskID: "task-1", DispatchID: "dispatch-other", RequestID: dispatchID, TerminalPTYID: "pty-other", TerminalHandle: "term-other"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			request := handoffDeliveryRequestFixture(test.host)
			identity := handoffDeliveryIdentityFixture()
			if err := observeHandoffDeliveryFailure(stateRoot, request, identity, &port.OrcaError{
				Code: "response_lost", Invoked: true, OrchestrationRequestID: dispatchID, CallPhase: "orca_dispatch",
			}, time.Now); err != nil {
				t.Fatal(err)
			}
			fake := &handoffDeliveryProvisionerFake{
				receipt: test.receipt,
				requestObservation: port.OrcaRequestObservation{
					RuntimeID: "runtime-1", RequestID: dispatchID, Status: "completed", Method: "orchestration.dispatch",
				},
			}
			inventory, err := newHandoffDeliveryProvisioner(stateRoot, fake, time.Now).InspectIntent(context.Background(), request)
			if err == nil {
				t.Fatalf("mismatched dispatch candidate was adopted or replayed: %+v", inventory)
			}
			if fake.invokeCalls != 0 {
				t.Fatalf("identity mismatch performed external work: %d", fake.invokeCalls)
			}
		})
	}
}

func TestHandoffDeliveryProductionProvisionerUsesOneTimestampBeforeInvoke(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateRoot)
	clock := advancingHandoffClock(time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC))
	provisioner := &handoffDeliveryProvisionerFake{
		receipt: port.ExecutionOrcaIntentReceipt{
			TerminalPTYID: "pty-1", TerminalHandle: "term-1", TaskID: "task-1",
			DispatchID: "dispatch-1", RequestID: "11111111-1111-4111-8111-111111111111",
		},
	}
	observed := newHandoffDeliveryProvisioner(stateRoot, provisioner, clock)

	_, err := observed.InvokeIntent(context.Background(), handoffDeliveryRequestFixture("codex"))
	if err != nil {
		t.Fatalf("production observed invoke: %v", err)
	}
	if provisioner.invokeCalls != 1 {
		t.Fatalf("external invoke calls=%d, want 1", provisioner.invokeCalls)
	}
	observations, err := auditadapter.ReadHandoffDeliveryAuditObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) < 1 {
		t.Fatal("production observer did not persist the pre-call frame")
	}
	staged := observations[0]
	if staged.CallStaged.ObservedAt != staged.UpdatedAt {
		t.Fatalf("pre-call event used more than one timestamp: observed_at=%q updated_at=%q", staged.CallStaged.ObservedAt, staged.UpdatedAt)
	}
	if staged.CallStaged.Evidence != "external_call_staged" || staged.Ambiguous.Status != "not_observed" || staged.Ambiguous.Evidence != "" || staged.Ambiguous.ObservedAt != "" {
		t.Fatalf("normal pre-call was mislabeled as a crash: staged=%+v ambiguous=%+v", staged.CallStaged, staged.Ambiguous)
	}
}

func TestHandoffDeliveryPreflightRejectionIsNotRecordedAsCrash(t *testing.T) {
	stateRoot := t.TempDir()
	fake := &handoffDeliveryProvisionerFake{invokeErr: &port.OrcaError{Code: "intent_preflight_rejected", Invoked: false}}
	_, err := newHandoffDeliveryProvisioner(stateRoot, fake, time.Now).InvokeIntent(context.Background(), handoffDeliveryRequestFixture("codex"))
	if err == nil {
		t.Fatal("preflight rejection was lost")
	}
	observations, readErr := auditadapter.ReadHandoffDeliveryAuditObservationsAt(stateRoot)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(observations) != 1 || observations[0].CallStaged.Status != "observed" || observations[0].Ambiguous.Status != "not_observed" {
		t.Fatalf("preflight rejection was mislabeled as crash: %+v", observations)
	}
}

func TestManualHandoffDeliveryRejectsClaimForgeryAndUsesIsolatedLineage(t *testing.T) {
	stateRoot := t.TempDir()
	request := handoffDeliveryRequestFixture("codex")
	identity := handoffDeliveryIdentityFixture()
	eventNow := handoffDeliveryEventClock(time.Now)
	observation, err := handoffDeliveryObservation(stateRoot, request, identity, port.ExecutionOrcaIntentReceipt{}, "", "prompt", eventNow)
	if err != nil {
		t.Fatal(err)
	}
	observation.AttemptID = handoffDeliveryManualLineagePrefix + request.Workspace.LifecycleID + ":1:orca:attempt-1"
	observation.LineageID = handoffDeliveryManualLineagePrefix + observation.LineageID
	observation.CallStaged = handoffDeliveryObserved(eventNow, "external_call_staged")
	record := issueopscontract.IssueOpsRecord{
		ID:        request.Workspace.LifecycleID,
		Execution: &issueopscontract.Execution{Mode: issueopscontract.ExecutionModeDirect, Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased}},
	}
	if err := validateManualHandoffDeliveryObservation(record, observation); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*issueopscontract.IssueOpsHandoffDeliveryObservation){
		func(value *issueopscontract.IssueOpsHandoffDeliveryObservation) {
			value.OwnerActor = &issueopscontract.NativeActor{Host: "codex", SessionID: "forged"}
		},
		func(value *issueopscontract.IssueOpsHandoffDeliveryObservation) {
			value.OwnerClaimed = handoffDeliveryObserved(eventNow, "issueops_claim")
		},
		func(value *issueopscontract.IssueOpsHandoffDeliveryObservation) {
			value.OwnerClaim = issueopscontract.IssueOpsHandoffDeliveryOwnerClaim{Claimed: true, Generation: 1, ClaimedAt: observation.UpdatedAt}
		},
	} {
		forged := observation
		mutate(&forged)
		if err := validateManualHandoffDeliveryObservation(record, forged); err == nil {
			t.Fatal("public manual observation accepted forged owner claim fields")
		}
	}
	if _, err := auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation); err != nil {
		t.Fatal(err)
	}
	standardLineage := strings.TrimPrefix(observation.LineageID, handoffDeliveryManualLineagePrefix)
	folded, _, err := auditadapter.FoldHandoffDeliveryAuditObservationsForAt(stateRoot, observation.LifecycleID, standardLineage)
	if err != nil || len(folded) != 0 {
		t.Fatalf("manual observation entered execution recovery lineage: folded=%+v err=%v", folded, err)
	}
}

func TestPublicManualHandoffProducerRejectsEveryOwnerClaimField(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateRoot)
	record := seedReleasedDirectHandoffRecord(t, stateRoot)
	request := handoffDeliveryRequestFixture("codex")
	request.Workspace.LifecycleID = record.ID
	identity := handoffDeliveryIdentityFixture()
	eventNow := handoffDeliveryEventClock(time.Now)
	observation, err := handoffDeliveryObservation(stateRoot, request, identity, port.ExecutionOrcaIntentReceipt{}, "", "prompt", eventNow)
	if err != nil {
		t.Fatal(err)
	}
	observation.AttemptID = handoffDeliveryManualLineagePrefix + record.ID + ":1:orca:attempt-1"
	observation.LineageID = handoffDeliveryManualLineagePrefix + observation.LineageID
	observation.CallStaged = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceExternalCallStaged)
	for name, mutate := range map[string]func(*issueopscontract.IssueOpsHandoffDeliveryObservation){
		"owner_actor": func(value *issueopscontract.IssueOpsHandoffDeliveryObservation) {
			value.OwnerActor = &issueopscontract.NativeActor{Host: "codex", SessionID: "forged"}
		},
		"owner_claimed": func(value *issueopscontract.IssueOpsHandoffDeliveryObservation) {
			value.OwnerClaimed = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim)
		},
		"owner_claim": func(value *issueopscontract.IssueOpsHandoffDeliveryObservation) {
			value.OwnerClaim = issueopscontract.IssueOpsHandoffDeliveryOwnerClaim{Claimed: true, Generation: 1, ClaimedAt: observation.UpdatedAt}
		},
	} {
		t.Run(name, func(t *testing.T) {
			forged := observation
			mutate(&forged)
			if _, err := auditManualHandoffDeliveryObservation(forged); err == nil || !strings.Contains(err.Error(), "cannot produce owner claim evidence") {
				t.Fatalf("public producer accepted forged %s: %v", name, err)
			}
		})
	}
	if _, err := auditManualHandoffDeliveryObservation(observation); err != nil {
		t.Fatalf("valid manual observation: %v", err)
	}
}

func seedReleasedDirectHandoffRecord(t *testing.T, stateRoot string) issueopscontract.IssueOpsRecord {
	t.Helper()
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(claimWiringGit(t, repo, "rev-parse", "HEAD"))
	record, err := issueopsadapter.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "11-manual-handoff"})
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(t.TempDir(), "manual-handoff")
	if err := os.MkdirAll(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	record.Phase = issueopsadapter.IssueOpsPhaseImplement
	record.WorktreePath = worktree
	record.IssueURL = "https://github.com/example/issueops/issues/11"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: repo, Root: worktree, Branch: record.Branch, BaseHead: baseHead,
			Driver: "git", LinkedAt: time.Now().UTC().Format(time.RFC3339Nano),
		},
		Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased},
	}
	written, err := issueopsadapter.WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return written
}

type handoffDeliveryProvisionerFake struct {
	receipt            port.ExecutionOrcaIntentReceipt
	requestObservation port.OrcaRequestObservation
	requestErr         error
	invokeErr          error
	invokeCalls        int
	lastInvokeRequest  port.ExecutionOrcaIntentRequest
}

type handoffDeliveryPromptCrashFake struct {
	handoffDeliveryProvisionerFake
	promptCalls int
}

type handoffDeliveryFreshOmoFake struct {
	handoffDeliveryProvisionerFake
	dispatchInspections  int
	inventoryInspections int
}

func (fake *handoffDeliveryFreshOmoFake) InspectDeliveryDispatch(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, bool, error) {
	fake.dispatchInspections++
	return port.ExecutionOrcaIntentReceipt{}, false, errors.New("fresh delivery has no durable dispatch request")
}

func (fake *handoffDeliveryFreshOmoFake) InspectIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	fake.inventoryInspections++
	return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
}

func (fake *handoffDeliveryPromptCrashFake) InvokeIntentObserved(_ context.Context, request port.ExecutionOrcaIntentRequest, observe func(port.ExecutionOrcaCallObservation) error) (port.ExecutionOrcaIntentReceipt, error) {
	fake.invokeCalls++
	fake.lastInvokeRequest = request
	dispatchReceipt := fake.receipt
	if err := observe(port.ExecutionOrcaCallObservation{CallKind: "dispatch", Phase: port.ExecutionOrcaCallStaged, Receipt: dispatchReceipt}); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if err := observe(port.ExecutionOrcaCallObservation{CallKind: "dispatch", Phase: port.ExecutionOrcaCallCompleted, Receipt: dispatchReceipt}); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if err := observe(port.ExecutionOrcaCallObservation{CallKind: "prompt", Phase: port.ExecutionOrcaCallStaged, Receipt: dispatchReceipt}); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	fake.promptCalls++
	return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{
		Code: "process_crash", Invoked: true, DispatchRequestID: dispatchReceipt.RequestID, CallPhase: "terminal_send",
	}
}

func (*handoffDeliveryProvisionerFake) InspectDeliveryIdentity(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaDeliveryIdentity, error) {
	return handoffDeliveryIdentityFixture(), nil
}

func (fake *handoffDeliveryProvisionerFake) InspectDeliveryDispatch(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, bool, error) {
	return fake.receipt, fake.receipt.DispatchID != "", nil
}

func (fake *handoffDeliveryProvisionerFake) ObserveRequest(context.Context, string) (port.OrcaRequestObservation, error) {
	if fake.requestErr != nil {
		return port.OrcaRequestObservation{}, fake.requestErr
	}
	return fake.requestObservation, nil
}

func handoffDeliveryIdentityFixture() port.ExecutionOrcaDeliveryIdentity {
	return port.ExecutionOrcaDeliveryIdentity{
		LauncherPath: "/usr/local/bin/orca", Version: "1.4.200", RuntimeID: "runtime-1",
		MachineID: "machine-1", TargetIdentity: "local", TerminalPTYID: "pty-1", TerminalHandle: "term-1",
	}
}

func (*handoffDeliveryProvisionerFake) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	return port.ExecutionOrcaProbeResult{Available: true, Ready: true}, nil
}

func (*handoffDeliveryProvisionerFake) InspectIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
}

func (fake *handoffDeliveryProvisionerFake) InvokeIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	fake.invokeCalls++
	fake.lastInvokeRequest = request
	if fake.invokeErr != nil {
		return port.ExecutionOrcaIntentReceipt{}, fake.invokeErr
	}
	return fake.receipt, nil
}

func handoffDeliveryRequestFixture(host string) port.ExecutionOrcaIntentRequest {
	return port.ExecutionOrcaIntentRequest{
		Stage:            port.ExecutionOrcaIntentDispatch,
		OperationID:      strings.Repeat("a", 32),
		SourceGeneration: 1,
		Workspace: port.ExecutionWorkspaceRequest{
			LifecycleID: "io-handoff-production", SourceRoot: "/repo", Root: "/repo.worktrees/task", Branch: "task", BaseHead: strings.Repeat("b", 40),
		},
		Probe: port.ExecutionOrcaProbeRequest{Repo: "/repo", Host: host, Model: "model", Marker: "marker"},
		Prepared: &port.ExecutionOrcaWorkspaceReceipt{
			Workspace: port.ExecutionWorkspaceReceipt{SourceRoot: "/repo", Root: "/repo.worktrees/task", Branch: "task", BaseHead: strings.Repeat("b", 40), Driver: "orca"},
			RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1",
		},
		Launch: &port.ExecutionOrcaLaunchRequest{
			Prompt: "handoff", PromptPath: "/repo.worktrees/task/owner.md", PromptSHA256: strings.Repeat("c", 64),
			ContextPacketPath: "/repo.worktrees/task/context.json", ContextPacketSHA256: strings.Repeat("d", 64),
		},
		TerminalPTYID: "pty-1", RunID: "run-1", RunBound: true, TaskID: "task-1",
	}
}

func advancingHandoffClock(start time.Time) func() time.Time {
	next := start
	return func() time.Time {
		value := next
		next = next.Add(time.Nanosecond)
		return value
	}
}
