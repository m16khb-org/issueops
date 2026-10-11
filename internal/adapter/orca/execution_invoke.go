package orca

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"issueops/internal/port"
)

func (p *ExecutionProvisioner) InvokeIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	return p.invokeIntent(ctx, req, nil)
}

func (p *ExecutionProvisioner) InvokeIntentObserved(ctx context.Context, req port.ExecutionOrcaIntentRequest, observe func(port.ExecutionOrcaCallObservation) error) (port.ExecutionOrcaIntentReceipt, error) {
	return p.invokeIntent(ctx, req, observe)
}

func (p *ExecutionProvisioner) invokeIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest, observe func(port.ExecutionOrcaCallObservation) error) (port.ExecutionOrcaIntentReceipt, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaIntentReceipt{}, executionPreflightError(fmt.Errorf("Orca client is unavailable"))
	}
	if err := validateExecutionIntentRequest(req); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, executionPreflightError(err)
	}
	switch req.Stage {
	case port.ExecutionOrcaIntentWorktree:
		return p.invokeWorktreeIntent(ctx, req)
	case port.ExecutionOrcaIntentTerminal:
		return p.invokeTerminalIntent(ctx, req)
	case port.ExecutionOrcaIntentRun:
		return p.invokeRunIntent(ctx, req)
	case port.ExecutionOrcaIntentRunBind:
		return p.invokeRunBindIntent(ctx, req)
	case port.ExecutionOrcaIntentTask:
		return p.invokeTaskIntent(ctx, req)
	case port.ExecutionOrcaIntentDispatch:
		return p.invokeDispatchIntent(ctx, req, observe)
	default:
		return port.ExecutionOrcaIntentReceipt{}, executionPreflightError(fmt.Errorf("unsupported stage %q", req.Stage))
	}
}

func (p *ExecutionProvisioner) invokeWorktreeIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	created, err := p.client.CreateWorktree(ctx, port.OrcaCreateWorktreeRequest{
		Repo: req.Workspace.SourceRoot, Name: req.Workspace.Branch, BaseBranch: req.Workspace.BaseHead,
		ParentWorktree: req.Workspace.ParentWorktree,
		Provider:       req.Probe.Provider, Issue: req.Probe.Issue, Comment: req.Marker,
	})
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if err := validateExecutionWorktree(created, req.Workspace, req.Probe); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "worktree_identity_mismatch", Detail: err.Error(), Invoked: true}
	}
	receipt := executionWorkspaceReceipt(req.Workspace, created)
	return port.ExecutionOrcaIntentReceipt{Workspace: &receipt}, nil
}

func (p *ExecutionProvisioner) invokeTerminalIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	created, err := p.client.CreateTerminal(ctx, port.OrcaCreateTerminalRequest{
		WorktreeID: req.Prepared.WorktreeID, Agent: req.Probe.Host, Model: req.Probe.Model, ReasoningEffort: req.Probe.Effort,
		Title: req.Marker, AllowCodexHookTrustBypass: req.Probe.Host == "codex",
		ExtraArgs: req.RoleAgentArgs,
	})
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	created, err = p.reconcileCreatedTerminal(ctx, created, *req.Prepared, req.Marker)
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "terminal_identity_mismatch", Detail: err.Error(), Invoked: true}
	}
	return port.ExecutionOrcaIntentReceipt{TerminalPTYID: created.PTYID}, nil
}

func (p *ExecutionProvisioner) invokeRunIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	created, err := p.client.CreateRun(ctx, port.OrcaCreateRunRequest{Objective: req.Marker})
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if err := validateExecutionIntentRun(created, *req.Prepared, req.Marker); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "run_identity_mismatch", Detail: err.Error(), Invoked: true}
	}
	return port.ExecutionOrcaIntentReceipt{RunID: created.ID}, nil
}

func (p *ExecutionProvisioner) invokeRunBindIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	current, err := p.client.CurrentRun(ctx)
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if current == nil || current.ID != req.RunID {
		used, err := p.client.UseRun(ctx, req.RunID)
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		current = &used
	}
	if err := validateExecutionIntentRun(*current, *req.Prepared, req.Marker); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "run_binding_mismatch", Detail: err.Error(), Invoked: current != nil}
	}
	return port.ExecutionOrcaIntentReceipt{RunID: current.ID, RunBound: true}, nil
}

func (p *ExecutionProvisioner) invokeTaskIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	created, err := p.client.CreateTask(ctx, port.OrcaCreateTaskRequest{
		RunID: req.RunID, Spec: req.Launch.Prompt, Title: executionTaskTitle(req.Marker, req.Launch.PromptSHA256), DisplayName: req.Workspace.Branch,
	})
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	title := executionTaskTitle(req.Marker, req.Launch.PromptSHA256)
	if err := validateExecutionIntentTask(created, *req.Prepared, req.RunID, title, req.Workspace.Branch); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "task_identity_mismatch", Detail: err.Error(), Invoked: true}
	}
	return port.ExecutionOrcaIntentReceipt{TaskID: created.ID}, nil
}

// invokeDispatchIntent dispatches the task to the resolved terminal and, for
// hosts that do not inject the preamble themselves, delivers it as a prompt.
func (p *ExecutionProvisioner) invokeDispatchIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest, observe func(port.ExecutionOrcaCallObservation) error) (port.ExecutionOrcaIntentReceipt, error) {
	terminal, err := p.resolveIntentTerminal(ctx, req)
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, executionPreflightError(err)
	}
	inject := req.Probe.Host != "omo"
	dispatchReceipt := port.ExecutionOrcaIntentReceipt{TaskID: req.TaskID, TerminalPTYID: terminal.PTYID, TerminalHandle: terminal.Handle}
	if err := observeDelivery(observe, "dispatch", port.ExecutionOrcaCallStaged, dispatchReceipt, port.OrcaError{Invoked: false, CallPhase: "orca_dispatch"}); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	dispatch, err := p.client.Dispatch(ctx, port.OrcaDispatchRequest{
		RunID: req.RunID, TaskID: req.TaskID, ToHandle: terminal.Handle, Inject: inject, ReturnPreamble: true, RetryRequestID: req.RetryRequestID,
	})
	if err != nil {
		if typed, ok := errors.AsType[*port.OrcaError](err); ok {
			typed.CallPhase = "orca_dispatch"
		}
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if err := validateInvokedDispatchResponse(dispatch, req, terminal, inject); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	dispatchReceipt = port.ExecutionOrcaIntentReceipt{TaskID: dispatch.TaskID, DispatchID: dispatch.ID, RequestID: dispatch.RequestID, TerminalPTYID: terminal.PTYID, TerminalHandle: terminal.Handle}
	dispatchRequestID := strings.TrimSpace(dispatch.RequestID)
	if err := observeDelivery(observe, "dispatch", port.ExecutionOrcaCallCompleted, dispatchReceipt, port.OrcaError{Invoked: true, DispatchRequestID: dispatchRequestID, CallPhase: "orca_dispatch"}); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if !inject {
		promptReceipt, err := p.deliverDispatchPrompt(ctx, req, terminal, dispatch, dispatchReceipt, observe)
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		dispatchReceipt.PromptReceipt = promptReceipt
	}
	return dispatchReceipt, nil
}

// validateInvokedDispatchResponse checks the durable request identity and the
// dispatch identity of an invoked dispatch against the sealed intent.
func validateInvokedDispatchResponse(dispatch port.OrcaDispatch, req port.ExecutionOrcaIntentRequest, terminal port.OrcaTerminal, inject bool) error {
	if err := port.ValidateOrcaDurableRequestID(dispatch.RequestID, req.RetryRequestID); err != nil {
		return &port.OrcaError{
			Code: "dispatch_request_identity_mismatch", Detail: err.Error(), Invoked: true,
			OrchestrationRequestID: strings.TrimSpace(dispatch.RequestID), CallPhase: "orca_dispatch",
		}
	}
	if err := validateExecutionInvokedDispatch(dispatch, req.Prepared.RuntimeID, req.TaskID, terminal.Handle, inject); err != nil {
		return &port.OrcaError{Code: "dispatch_identity_mismatch", Detail: err.Error(), Invoked: true}
	}
	return nil
}

// deliverDispatchPrompt sends the dispatch preamble to the terminal as a prompt,
// validates the receipt, and reports staged/completed prompt observations.
func (p *ExecutionProvisioner) deliverDispatchPrompt(ctx context.Context, req port.ExecutionOrcaIntentRequest, terminal port.OrcaTerminal, dispatch port.OrcaDispatch, dispatchReceipt port.ExecutionOrcaIntentReceipt, observe func(port.ExecutionOrcaCallObservation) error) (*port.OrcaPromptReceipt, error) {
	dispatchRequestID := strings.TrimSpace(dispatch.RequestID)
	if err := observeDelivery(observe, "prompt", port.ExecutionOrcaCallStaged, dispatchReceipt, port.OrcaError{Invoked: true, DispatchRequestID: dispatchRequestID, CallPhase: "terminal_send"}); err != nil {
		return nil, err
	}
	receipt, err := p.client.SendTerminalPrompt(ctx, terminal.Handle, dispatch.Preamble, req.PromptRetryRequestID)
	if err != nil {
		if typed, ok := errors.AsType[*port.OrcaError](err); ok {
			typed.Invoked = true
			typed.CallPhase = "terminal_send"
			typed.DispatchRequestID = dispatchRequestID
		}
		return nil, err
	}
	if err := validateDispatchPromptReceipt(receipt, req, dispatchRequestID); err != nil {
		return nil, err
	}
	completed := dispatchReceipt
	completed.PromptReceipt = &receipt
	if err := observeDelivery(observe, "prompt", port.ExecutionOrcaCallCompleted, completed, port.OrcaError{Invoked: true, OrchestrationRequestID: strings.TrimSpace(receipt.RequestID), DispatchRequestID: dispatchRequestID, CallPhase: "terminal_send"}); err != nil {
		return nil, err
	}
	return &receipt, nil
}

// validateDispatchPromptReceipt checks the prompt receipt's durable request
// identity and process incarnation against the sealed expectations.
func validateDispatchPromptReceipt(receipt port.OrcaPromptReceipt, req port.ExecutionOrcaIntentRequest, dispatchRequestID string) error {
	if err := port.ValidateOrcaDurableRequestID(receipt.RequestID, req.PromptRetryRequestID); err != nil {
		code := "terminal_prompt_receipt_invalid"
		if strings.TrimSpace(req.PromptRetryRequestID) != "" {
			code = "terminal_prompt_request_identity_mismatch"
		}
		return &port.OrcaError{
			Code: code, Detail: err.Error(), Invoked: true,
			OrchestrationRequestID: strings.TrimSpace(receipt.RequestID), DispatchRequestID: dispatchRequestID, CallPhase: "terminal_send",
		}
	}
	if err := port.ValidateOrcaPromptReceipt(receipt, req.PromptRetryRequestID, req.ExpectedPromptProcessIncarnation); err != nil {
		code := "terminal_prompt_receipt_invalid"
		if expected := strings.TrimSpace(req.ExpectedPromptProcessIncarnation); expected != "" && strings.TrimSpace(receipt.ProcessIncarnation) != expected {
			code = "terminal_process_incarnation_mismatch"
		}
		return &port.OrcaError{
			Code: code, Detail: err.Error(), Invoked: true,
			OrchestrationRequestID: strings.TrimSpace(receipt.RequestID), DispatchRequestID: dispatchRequestID, CallPhase: "terminal_send",
		}
	}
	return nil
}

// observeDelivery reports one call observation. An observer failure becomes a
// delivery_observation_failed OrcaError built from the failure template.
func observeDelivery(observe func(port.ExecutionOrcaCallObservation) error, callKind string, phase port.ExecutionOrcaCallPhase, receipt port.ExecutionOrcaIntentReceipt, failure port.OrcaError) error {
	if observe == nil {
		return nil
	}
	if err := observe(port.ExecutionOrcaCallObservation{CallKind: callKind, Phase: phase, Receipt: receipt}); err != nil {
		failure.Code = "delivery_observation_failed"
		failure.Detail = err.Error()
		return &failure
	}
	return nil
}

func executionPreflightError(err error) error {
	if err == nil {
		return nil
	}
	return &port.OrcaError{Code: "intent_preflight_rejected", Detail: err.Error(), Invoked: false}
}
