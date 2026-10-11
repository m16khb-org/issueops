package handoffdelivery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	auditcontract "issueops/internal/contract/audit"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

const (
	testDispatchUUID = "11111111-1111-4111-8111-111111111111"
	testPromptUUID   = "22222222-2222-4222-8222-222222222222"
	testSHA          = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// memAudit는 Append한 관찰을 그대로 Read/ReadFor로 돌려주는 인메모리 감사 로그다.
type memAudit struct {
	observations []model.IssueOpsHandoffDeliveryObservation
	readErr      error
	appendErr    error
	// failAppendAfter>0이면 그 개수만큼 Append가 성공한 뒤부터 lateErr로 실패한다.
	failAppendAfter int
	lateErr         error
}

func (a *memAudit) Append(o model.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	if a.appendErr != nil {
		return auditcontract.HandoffDeliveryAuditRecord{}, a.appendErr
	}
	if a.failAppendAfter > 0 && len(a.observations) >= a.failAppendAfter {
		return auditcontract.HandoffDeliveryAuditRecord{}, a.lateErr
	}
	// 실제 감사 저장소가 Append 시점에 채우는 영수증을 흉내 낸다.
	o.Receipt = model.IssueOpsHandoffDeliveryReceipt{Location: "audit://test", Digest: testSHA}
	a.observations = append(a.observations, o)
	return auditcontract.HandoffDeliveryAuditRecord{Observation: o}, nil
}
func (a *memAudit) Read() ([]model.IssueOpsHandoffDeliveryObservation, error) {
	return append([]model.IssueOpsHandoffDeliveryObservation(nil), a.observations...), a.readErr
}
func (a *memAudit) ReadFor(lifecycle, lineage string) ([]model.IssueOpsHandoffDeliveryObservation, error) {
	if a.readErr != nil {
		return nil, a.readErr
	}
	var out []model.IssueOpsHandoffDeliveryObservation
	for _, o := range a.observations {
		if o.LifecycleID == lifecycle && o.LineageID == lineage {
			out = append(out, o)
		}
	}
	return out, nil
}

// bareNext는 delivery observer 인터페이스를 구현하지 않는 최소 provisioner다.
type bareNext struct{ inspects, invokes int }

func (*bareNext) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	return port.ExecutionOrcaProbeResult{Code: "probed"}, nil
}
func (n *bareNext) InspectIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	n.inspects++
	return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
}
func (n *bareNext) InvokeIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	n.invokes++
	return port.ExecutionOrcaIntentReceipt{RequestID: "bare"}, nil
}

// scriptedNext는 identity/요청 관찰/invoke 결과를 제어할 수 있는 observer provisioner다.
type scriptedNext struct {
	bareNext
	identity      port.ExecutionOrcaDeliveryIdentity
	identityErr   error
	invokeReceipt port.ExecutionOrcaIntentReceipt
	invokeErr     error
	observed      port.OrcaRequestObservation
	observeErr    error
	dispatch      port.ExecutionOrcaIntentReceipt
	dispatchOK    bool
	dispatchErr   error
	dispatchReqs  []port.ExecutionOrcaIntentRequest
	seen          []port.ExecutionOrcaIntentRequest
}

func (n *scriptedNext) InvokeIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	n.invokes++
	n.seen = append(n.seen, request)
	return n.invokeReceipt, n.invokeErr
}
func (n *scriptedNext) InspectDeliveryIdentity(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaDeliveryIdentity, error) {
	return n.identity, n.identityErr
}
func (n *scriptedNext) InspectDeliveryDispatch(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, bool, error) {
	n.dispatchReqs = append(n.dispatchReqs, request)
	return n.dispatch, n.dispatchOK, n.dispatchErr
}
func (n *scriptedNext) ObserveRequest(context.Context, string) (port.OrcaRequestObservation, error) {
	return n.observed, n.observeErr
}

// observedNext는 InvokeIntentObserved를 추가로 구현해 콜백을 직접 구동한다.
type observedNext struct {
	scriptedNext
	phases []port.ExecutionOrcaCallObservation
	err    error
}

func (n *observedNext) InvokeIntentObserved(_ context.Context, _ port.ExecutionOrcaIntentRequest, observe func(port.ExecutionOrcaCallObservation) error) (port.ExecutionOrcaIntentReceipt, error) {
	for _, phase := range n.phases {
		if err := observe(phase); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
	}
	return n.invokeReceipt, n.err
}

func fixedClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC) }
}

func deliveryRequest(host string) port.ExecutionOrcaIntentRequest {
	return port.ExecutionOrcaIntentRequest{
		Stage:            port.ExecutionOrcaIntentDispatch,
		OperationID:      "op-1",
		SourceGeneration: 1,
		Workspace:        port.ExecutionWorkspaceRequest{LifecycleID: "life-1"},
		Probe:            port.ExecutionOrcaProbeRequest{Host: host},
		Launch:           &port.ExecutionOrcaLaunchRequest{PromptSHA256: testSHA, ContextPacketSHA256: testSHA},
		TerminalPTYID:    "pty-1",
		TaskID:           "task-1",
	}
}

func testIdentity() port.ExecutionOrcaDeliveryIdentity {
	return port.ExecutionOrcaDeliveryIdentity{LauncherPath: "/usr/bin/orca", Version: "1.0", RuntimeID: "rt-1", MachineID: "m-1", TargetIdentity: "srv-1", TerminalPTYID: "pty-1", TerminalHandle: "pane-1"}
}

func newProvisioner(audit Audit, next port.ExecutionOrcaProvisioner) *Provisioner {
	return &Provisioner{Audit: audit, Next: next, Now: fixedClock()}
}

func TestNonDeliveryRequestsPassThroughUntouched(t *testing.T) {
	next := &bareNext{}
	audit := &memAudit{}
	p := newProvisioner(audit, next)
	request := deliveryRequest("codex")
	request.Launch = nil // launch가 없으면 delivery가 아니다.
	if probe, err := p.Probe(context.Background(), port.ExecutionOrcaProbeRequest{}); err != nil || probe.Code != "probed" {
		t.Fatalf("probe=%+v err=%v", probe, err)
	}
	if inv, err := p.InspectIntent(context.Background(), request); err != nil || !inv.AuthoritativeZero || next.inspects != 1 {
		t.Fatalf("inv=%+v err=%v", inv, err)
	}
	if receipt, err := p.InvokeIntent(context.Background(), request); err != nil || receipt.RequestID != "bare" || next.invokes != 1 {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if len(audit.observations) != 0 {
		t.Fatalf("pass-through wrote audit: %v", audit.observations)
	}
}

func TestDeliveryRejectsInvalidRetryIDsBeforeAnyEffect(t *testing.T) {
	for name, mutate := range map[string]func(*port.ExecutionOrcaIntentRequest){
		"dispatch": func(r *port.ExecutionOrcaIntentRequest) { r.RetryRequestID = "not-a-uuid" },
		"prompt":   func(r *port.ExecutionOrcaIntentRequest) { r.PromptRetryRequestID = "not-a-uuid" },
	} {
		t.Run(name, func(t *testing.T) {
			next := &scriptedNext{identity: testIdentity()}
			p := newProvisioner(&memAudit{}, next)
			request := deliveryRequest("codex")
			mutate(&request)
			if _, err := p.InvokeIntent(context.Background(), request); err == nil || !strings.Contains(err.Error(), name+" retry request ID is invalid") {
				t.Fatalf("invoke err=%v", err)
			}
			if _, err := p.InspectIntent(context.Background(), request); err == nil || !strings.Contains(err.Error(), name+" retry request ID is invalid") {
				t.Fatalf("inspect err=%v", err)
			}
			if next.invokes != 0 {
				t.Fatal("external invoked despite invalid retry id")
			}
		})
	}
}

func TestDeliveryRequiresIdentityObserverAndPropagatesAuditReadError(t *testing.T) {
	p := newProvisioner(&memAudit{}, &bareNext{})
	if _, err := p.InvokeIntent(context.Background(), deliveryRequest("codex")); err == nil || !strings.Contains(err.Error(), "identity observer is unavailable") {
		t.Fatalf("err=%v", err)
	}
	if _, err := p.InspectIntent(context.Background(), deliveryRequest("codex")); err == nil || !strings.Contains(err.Error(), "identity observer is unavailable") {
		t.Fatalf("err=%v", err)
	}
	readErr := errors.New("audit read")
	next := &scriptedNext{identity: testIdentity()}
	p = newProvisioner(&memAudit{readErr: readErr}, next)
	if _, err := p.InvokeIntent(context.Background(), deliveryRequest("codex")); !errors.Is(err, readErr) || next.invokes != 0 {
		t.Fatalf("err=%v invokes=%d", err, next.invokes)
	}
	identityErr := errors.New("identity")
	next = &scriptedNext{identityErr: identityErr}
	p = newProvisioner(&memAudit{}, next)
	if _, err := p.InvokeIntent(context.Background(), deliveryRequest("codex")); !errors.Is(err, identityErr) || next.invokes != 0 {
		t.Fatalf("err=%v", err)
	}
}

func TestInvokeRecordsStagedThenAcceptedDispatchForNonOmoHost(t *testing.T) {
	audit := &memAudit{}
	next := &scriptedNext{identity: testIdentity(), invokeReceipt: port.ExecutionOrcaIntentReceipt{RequestID: testDispatchUUID, TerminalPTYID: "pty-1", TerminalHandle: "pane-1"}}
	receipt, err := newProvisioner(audit, next).InvokeIntent(context.Background(), deliveryRequest("codex"))
	if err != nil || receipt.RequestID != testDispatchUUID {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if len(audit.observations) != 2 {
		t.Fatalf("observations=%d", len(audit.observations))
	}
	staged, done := audit.observations[0], audit.observations[1]
	if staged.CallStaged.Status != model.IssueOpsHandoffDeliveryStateObserved || staged.Request.DurableID != "" {
		t.Fatalf("staged=%+v", staged)
	}
	if done.InputAccepted.Status != model.IssueOpsHandoffDeliveryStateObserved || done.Request.DurableID != testDispatchUUID {
		t.Fatalf("done=%+v", done)
	}
	if done.Launcher.Name != model.IssueOpsHandoffDeliveryLauncherOrca || done.Launcher.RuntimeID != "rt-1" || done.Launcher.ServerID != "srv-1" {
		t.Fatalf("launcher=%+v", done.Launcher)
	}
	if done.Target.TerminalID != "pty-1" || done.Target.PaneID != "pane-1" || done.AttemptID != "op-1:dispatch:dispatch" {
		t.Fatalf("target=%+v attempt=%s", done.Target, done.AttemptID)
	}
}

func TestInvokeOmoDispatchIsAmbiguousUntilPromptReceipt(t *testing.T) {
	audit := &memAudit{}
	next := &scriptedNext{identity: testIdentity(), invokeReceipt: port.ExecutionOrcaIntentReceipt{
		RequestID: testDispatchUUID, TerminalPTYID: "pty-1", TerminalHandle: "pane-1",
		PromptReceipt: &port.OrcaPromptReceipt{RequestID: testPromptUUID, Stages: []string{"input_accepted", "turn_started"}, ProcessIncarnation: "proc-1", Generation: 3},
	}}
	if _, err := newProvisioner(audit, next).InvokeIntent(context.Background(), deliveryRequest("omo")); err != nil {
		t.Fatal(err)
	}
	if len(audit.observations) != 3 {
		t.Fatalf("observations=%d", len(audit.observations))
	}
	dispatch, prompt := audit.observations[1], audit.observations[2]
	if dispatch.Ambiguous.Evidence != model.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt || dispatch.InputAccepted.Status == model.IssueOpsHandoffDeliveryStateObserved {
		t.Fatalf("dispatch=%+v", dispatch)
	}
	if prompt.Request.DurableID != testPromptUUID || prompt.Target.ProcessIncarnation != "proc-1" || prompt.Target.PromptGeneration == nil || *prompt.Target.PromptGeneration != 3 {
		t.Fatalf("prompt=%+v", prompt)
	}
	if prompt.InputAccepted.Evidence != model.IssueOpsHandoffDeliveryEvidenceOmoSendAccepted || prompt.NativeTurnObserved.Evidence != model.IssueOpsHandoffDeliveryEvidenceNativeReceipt {
		t.Fatalf("prompt=%+v", prompt)
	}
	if prompt.AttemptID != "op-1:dispatch:prompt" {
		t.Fatalf("attempt=%s", prompt.AttemptID)
	}
}

func TestInvokeFailureRecordsLostResponseAndJoinsAuditError(t *testing.T) {
	cause := &port.OrcaError{Code: "dispatch_failed", Invoked: true, OrchestrationRequestID: testDispatchUUID}
	audit := &memAudit{}
	next := &scriptedNext{identity: testIdentity(), invokeErr: cause}
	_, err := newProvisioner(audit, next).InvokeIntent(context.Background(), deliveryRequest("codex"))
	if !errors.Is(err, cause) {
		t.Fatalf("err=%v", err)
	}
	last := audit.observations[len(audit.observations)-1]
	if last.Ambiguous.Evidence != model.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost || last.Request.DurableID != testDispatchUUID {
		t.Fatalf("last=%+v", last)
	}

	// staged 기록 뒤 실패 기록이 실패하면 원인과 감사 오류가 모두 보존된다.
	auditErr := errors.New("failure append unavailable")
	audit = &memAudit{failAppendAfter: 1, lateErr: auditErr}
	next = &scriptedNext{identity: testIdentity(), invokeErr: cause}
	_, err = newProvisioner(audit, next).InvokeIntent(context.Background(), deliveryRequest("codex"))
	if !errors.Is(err, cause) || !errors.Is(err, auditErr) || len(audit.observations) != 1 {
		t.Fatalf("err=%v observations=%d", err, len(audit.observations))
	}

	// 호출이 실행되지 않은 실패는 모호성 기록 없이 staged 기록만 남긴다.
	audit = &memAudit{}
	next = &scriptedNext{identity: testIdentity(), invokeErr: &port.OrcaError{Code: "unreachable"}}
	if _, err = newProvisioner(audit, next).InvokeIntent(context.Background(), deliveryRequest("codex")); err == nil {
		t.Fatal("want error")
	}
	if len(audit.observations) != 1 || audit.observations[0].Ambiguous.Status == model.IssueOpsHandoffDeliveryStateObserved {
		t.Fatalf("observations=%+v", audit.observations)
	}
}

func TestInvokeTerminalSendFailureRecordsPromptAndDispatchLineages(t *testing.T) {
	cause := &port.OrcaError{Code: "terminal_send_failed", Invoked: true, CallPhase: "terminal_send", OrchestrationRequestID: testPromptUUID, DispatchRequestID: testDispatchUUID}
	audit := &memAudit{}
	request := deliveryRequest("omo")
	if err := ObserveFailure(audit, request, testIdentity(), cause, fixedClock()); err != nil {
		t.Fatal(err)
	}
	if len(audit.observations) != 2 {
		t.Fatalf("observations=%d", len(audit.observations))
	}
	dispatch, prompt := audit.observations[0], audit.observations[1]
	if dispatch.AttemptID != "op-1:dispatch:dispatch" || dispatch.Request.DurableID != testDispatchUUID || dispatch.Ambiguous.Evidence != model.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt {
		t.Fatalf("dispatch=%+v", dispatch)
	}
	if prompt.AttemptID != "op-1:dispatch:prompt" || prompt.Request.DurableID != testPromptUUID || prompt.Ambiguous.Evidence != model.IssueOpsHandoffDeliveryEvidenceOmoSendResponseLost {
		t.Fatalf("prompt=%+v", prompt)
	}
}

func TestObserveFailurePropagatesAuditErrors(t *testing.T) {
	boom := errors.New("append boom")
	cause := &port.OrcaError{Code: "x", Invoked: true}
	if err := ObserveFailure(&memAudit{appendErr: boom}, deliveryRequest("codex"), testIdentity(), cause, fixedClock()); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	readErr := errors.New("read boom")
	if err := ObserveFailure(&memAudit{readErr: readErr}, deliveryRequest("codex"), testIdentity(), cause, fixedClock()); !errors.Is(err, readErr) {
		t.Fatalf("err=%v", err)
	}
	// 정체성 불일치 응답은 기록하지 않는다.
	audit := &memAudit{}
	mismatch := &port.OrcaError{Code: "dispatch_request_identity_mismatch", Invoked: true}
	if err := ObserveFailure(audit, deliveryRequest("codex"), testIdentity(), mismatch, fixedClock()); err != nil || len(audit.observations) != 0 {
		t.Fatalf("err=%v observations=%d", err, len(audit.observations))
	}
}

func TestObservedInvokerCallbacksRecordPhasesAndRejectUnknownPhase(t *testing.T) {
	audit := &memAudit{}
	next := &observedNext{scriptedNext: scriptedNext{identity: testIdentity(), invokeReceipt: port.ExecutionOrcaIntentReceipt{RequestID: testDispatchUUID}}}
	next.phases = []port.ExecutionOrcaCallObservation{
		{Phase: port.ExecutionOrcaCallStaged, CallKind: "dispatch"},
		{Phase: port.ExecutionOrcaCallCompleted, CallKind: "dispatch", Receipt: port.ExecutionOrcaIntentReceipt{RequestID: testDispatchUUID}},
	}
	if _, err := newProvisioner(audit, next).InvokeIntent(context.Background(), deliveryRequest("codex")); err != nil {
		t.Fatal(err)
	}
	if len(audit.observations) != 2 || audit.observations[0].CallStaged.Status != model.IssueOpsHandoffDeliveryStateObserved || audit.observations[1].InputAccepted.Status != model.IssueOpsHandoffDeliveryStateObserved {
		t.Fatalf("observations=%+v", audit.observations)
	}

	audit = &memAudit{}
	next.phases = []port.ExecutionOrcaCallObservation{{Phase: "bogus"}}
	_, err := newProvisioner(audit, next).InvokeIntent(context.Background(), deliveryRequest("codex"))
	if err == nil || !strings.Contains(err.Error(), `unsupported Orca delivery call observation phase "bogus"`) {
		t.Fatalf("err=%v", err)
	}
}

func TestStagedCallWithoutAcceptedResponseBlocksRetry(t *testing.T) {
	audit := &memAudit{}
	request := deliveryRequest("codex")
	if err := ObserveStaged(audit, request, testIdentity(), "dispatch", port.ExecutionOrcaIntentReceipt{}, fixedClock()); err != nil {
		t.Fatal(err)
	}
	next := &scriptedNext{identity: testIdentity()}
	p := newProvisioner(audit, next)
	if _, err := p.InvokeIntent(context.Background(), request); err == nil || !strings.Contains(err.Error(), "ambiguous after a staged call") || next.invokes != 0 {
		t.Fatalf("err=%v invokes=%d", err, next.invokes)
	}
	if _, err := p.InspectIntent(context.Background(), request); err == nil || !strings.Contains(err.Error(), "ambiguous after a staged call") {
		t.Fatalf("inspect err=%v", err)
	}
}

func TestRetryRequestIDWithoutDurableObservationIsRejected(t *testing.T) {
	next := &scriptedNext{identity: testIdentity()}
	request := deliveryRequest("codex")
	request.RetryRequestID = testDispatchUUID
	if _, err := newProvisioner(&memAudit{}, next).InvokeIntent(context.Background(), request); err == nil || !strings.Contains(err.Error(), "no exact delivery observation") || next.invokes != 0 {
		t.Fatalf("err=%v invokes=%d", err, next.invokes)
	}
}

func TestInvokeAdoptsDurableRequestIDFromAuditForRetry(t *testing.T) {
	audit := &memAudit{}
	request := deliveryRequest("codex")
	if err := ObserveCompleted(audit, request, testIdentity(), "dispatch", port.ExecutionOrcaIntentReceipt{RequestID: testDispatchUUID}, fixedClock()); err != nil {
		t.Fatal(err)
	}
	next := &scriptedNext{identity: testIdentity(), invokeReceipt: port.ExecutionOrcaIntentReceipt{RequestID: testDispatchUUID}}
	if _, err := newProvisioner(audit, next).InvokeIntent(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(next.seen) != 1 || next.seen[0].RetryRequestID != testDispatchUUID {
		t.Fatalf("seen=%+v", next.seen)
	}
}

func TestInspectDelegatesWhenNoDeliveryEvidenceExists(t *testing.T) {
	next := &scriptedNext{identity: testIdentity()}
	inv, err := newProvisioner(&memAudit{}, next).InspectIntent(context.Background(), deliveryRequest("codex"))
	if err != nil || !inv.AuthoritativeZero || next.inspects != 1 {
		t.Fatalf("inv=%+v err=%v inspects=%d", inv, err, next.inspects)
	}
}

func recordedDispatch(t *testing.T, host string) (*memAudit, port.ExecutionOrcaIntentRequest) {
	t.Helper()
	audit := &memAudit{}
	request := deliveryRequest(host)
	if err := ObserveCompleted(audit, request, testIdentity(), "dispatch", port.ExecutionOrcaIntentReceipt{RequestID: testDispatchUUID, TerminalPTYID: "pty-1", TerminalHandle: "pane-1"}, fixedClock()); err != nil {
		t.Fatal(err)
	}
	request.RetryRequestID = testDispatchUUID
	return audit, request
}

func TestInspectRecoversExistingDispatchAsCandidate(t *testing.T) {
	audit, request := recordedDispatch(t, "codex")
	candidate := port.ExecutionOrcaIntentReceipt{RequestID: testDispatchUUID, TaskID: "task-1", DispatchID: "dispatch-9", TerminalPTYID: "pty-1", TerminalHandle: "pane-1"}
	next := &scriptedNext{
		identity: testIdentity(), dispatch: candidate, dispatchOK: true,
		observed: port.OrcaRequestObservation{RequestID: testDispatchUUID, Method: "orchestration.dispatch", RuntimeID: "rt-1", Status: "completed"},
	}
	inv, err := newProvisioner(audit, next).InspectIntent(context.Background(), request)
	if err != nil || len(inv.Candidates) != 1 || inv.Candidates[0].DispatchID != "dispatch-9" || next.inspects != 0 {
		t.Fatalf("inv=%+v err=%v", inv, err)
	}
	if len(next.dispatchReqs) != 1 || next.dispatchReqs[0].RetryRequestID != testDispatchUUID {
		t.Fatalf("dispatchReqs=%+v", next.dispatchReqs)
	}
}

func TestInspectReplaysWhenCompletedDispatchHasNoCandidate(t *testing.T) {
	audit, request := recordedDispatch(t, "codex")
	next := &scriptedNext{
		identity: testIdentity(),
		observed: port.OrcaRequestObservation{RequestID: testDispatchUUID, Method: "orchestration.dispatch", RuntimeID: "rt-1", Status: "pending"},
	}
	inv, err := newProvisioner(audit, next).InspectIntent(context.Background(), request)
	if err != nil || !inv.ExactReplay || len(inv.Candidates) != 0 {
		t.Fatalf("inv=%+v err=%v", inv, err)
	}
}

func TestInspectRecoveryRejectsInconsistentExternalEvidence(t *testing.T) {
	audit, request := recordedDispatch(t, "codex")
	observed := port.OrcaRequestObservation{RequestID: testDispatchUUID, Method: "orchestration.dispatch", RuntimeID: "rt-1", Status: "completed"}
	cases := map[string]struct {
		next *scriptedNext
		want string
	}{
		"identity mismatch": {&scriptedNext{identity: testIdentity(), observed: port.OrcaRequestObservation{RequestID: "other", Method: observed.Method, RuntimeID: "rt-1", Status: "completed"}}, "identity mismatch"},
		"absent":            {&scriptedNext{identity: testIdentity(), observed: port.OrcaRequestObservation{RequestID: testDispatchUUID, Method: observed.Method, RuntimeID: "rt-1", Status: "absent"}}, "absent"},
		"invalid status":    {&scriptedNext{identity: testIdentity(), observed: port.OrcaRequestObservation{RequestID: testDispatchUUID, Method: observed.Method, RuntimeID: "rt-1", Status: "weird"}}, "state is invalid"},
		"observe error":     {&scriptedNext{identity: testIdentity(), observeErr: errors.New("rpc down")}, "rpc down"},
		"inspect error":     {&scriptedNext{identity: testIdentity(), observed: observed, dispatchErr: errors.New("inspect down")}, "inspect down"},
		"receipt mismatch":  {&scriptedNext{identity: testIdentity(), observed: observed, dispatchOK: true, dispatch: port.ExecutionOrcaIntentReceipt{RequestID: testDispatchUUID, TaskID: "other-task", DispatchID: "d", TerminalPTYID: "pty-1", TerminalHandle: "pane-1"}}, "does not match the durable request"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newProvisioner(audit, test.next).InspectIntent(context.Background(), request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want %q", err, test.want)
			}
		})
	}
}

func TestInspectRejectsDispatchEvidenceWithoutDurableRequestID(t *testing.T) {
	// durable request ID가 없는 dispatch 증거는 모호하므로 복구를 거부한다.
	audit := &memAudit{}
	request := deliveryRequest("codex")
	if err := ObserveCompleted(audit, request, testIdentity(), "dispatch", port.ExecutionOrcaIntentReceipt{}, fixedClock()); err != nil {
		t.Fatal(err)
	}
	next := &scriptedNext{identity: testIdentity()}
	if _, err := newProvisioner(audit, next).InspectIntent(context.Background(), request); err == nil || !strings.Contains(err.Error(), "ambiguous without a durable request ID") {
		t.Fatalf("err=%v", err)
	}
}

func TestInspectOmoRecoversPromptDeliveryAsCandidateWithPromptReceipt(t *testing.T) {
	audit := &memAudit{}
	request := deliveryRequest("omo")
	id := testIdentity()
	if err := ObserveCompleted(audit, request, id, "dispatch", port.ExecutionOrcaIntentReceipt{RequestID: testDispatchUUID, TerminalPTYID: "pty-1", TerminalHandle: "pane-1"}, fixedClock()); err != nil {
		t.Fatal(err)
	}
	baseline := uint64(7)
	promptReceipt := port.ExecutionOrcaIntentReceipt{
		RequestID: testDispatchUUID, TerminalPTYID: "pty-1", TerminalHandle: "pane-1",
		PromptReceipt: &port.OrcaPromptReceipt{RequestID: testPromptUUID, Stages: []string{"input_accepted"}, ProcessIncarnation: "proc-1", Generation: 2, BaselineWorkingSequence: &baseline},
	}
	if err := ObserveCompleted(audit, request, id, "prompt", promptReceipt, fixedClock()); err != nil {
		t.Fatal(err)
	}
	request.RetryRequestID = testDispatchUUID
	request.PromptRetryRequestID = testPromptUUID
	next := &scriptedNext{
		identity:   id,
		observed:   port.OrcaRequestObservation{RequestID: testDispatchUUID, Method: "orchestration.dispatch", RuntimeID: "rt-1", Status: "completed"},
		dispatchOK: true,
		dispatch: port.ExecutionOrcaIntentReceipt{
			RequestID: testDispatchUUID, TaskID: "task-1", DispatchID: "dispatch-9", TerminalPTYID: "pty-1", TerminalHandle: "pane-1",
		},
	}
	inv, err := newProvisioner(audit, next).InspectIntent(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Candidates) != 1 || inv.Candidates[0].PromptReceipt == nil || inv.Candidates[0].PromptReceipt.RequestID != testPromptUUID || inv.Candidates[0].PromptReceipt.Provider != "omo" {
		t.Fatalf("inv=%+v", inv)
	}
}
