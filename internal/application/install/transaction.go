package install

import (
	"context"
	"errors"
	"fmt"

	activationcontract "issueops/internal/contract/nativeactivation"
	"issueops/internal/port"
)

// PathTransaction and HostTransaction preserve the installation's rollback boundaries.
// Their implementations own filesystem snapshots and writes.
type PathTransaction interface {
	Apply(*port.NativeInstallResult) error
	Rollback(*port.NativeInstallResult) error
	Finalize(*port.NativeInstallResult) error
}

type HostTransaction interface{ RollbackHosts() error }

type TransactionEffects interface {
	PreparePath(*port.NativeInstallResult, port.NativeInstallRequest, string, string) (PathTransaction, error)
	Install(port.NativeInstallRequest) (port.NativeInstallResult, error)
	PlanShell(*port.NativeInstallResult, port.NativeInstallRequest, string) error
	PrepareHost(port.NativeInstallResult) (HostTransaction, error)
	AppendUpstream(*port.NativeInstallResult, string, bool)
}

type Activation interface {
	Begin(context.Context, activationcontract.Request) (activationcontract.Result, error)
	Seal(context.Context, activationcontract.Request) (activationcontract.Result, error)
	Abort(context.Context, activationcontract.Request) (activationcontract.Result, error)
}

type TransactionRequest struct {
	Install       port.NativeInstallRequest
	CandidatePath string
	PathMode      string
	Step          string
	TransitionID  string
	Activation    activationcontract.Request
}

type TransactionOutcome struct {
	Install    *port.NativeInstallResult
	Activation *activationcontract.Result
}

func RunTransaction(ctx context.Context, request TransactionRequest, effects TransactionEffects, activation Activation) (TransactionOutcome, error) {
	if request.Step == "abort" {
		request.Activation.TransitionID = request.TransitionID
		aborted, err := activation.Abort(ctx, request.Activation)
		if err != nil {
			return TransactionOutcome{}, fmt.Errorf("abort native activation: %w", err)
		}
		return TransactionOutcome{Activation: &aborted}, nil
	}
	preflight := port.NativeInstallResult{OK: true, Root: request.Install.Root, Home: request.Install.Home, CodexHome: request.Install.CodexHome, BinPath: request.Install.BinPath}
	pathTransaction, err := effects.PreparePath(&preflight, request.Install, request.CandidatePath, request.PathMode)
	if err != nil {
		return TransactionOutcome{}, err
	}
	if request.Install.DryRun {
		result, installErr := effects.Install(request.Install)
		result.Links = append(preflight.Links, result.Links...)
		result.Files = append(preflight.Files, result.Files...)
		result.Messages = append(preflight.Messages, result.Messages...)
		result.CommandPath = preflight.CommandPath
		effects.AppendUpstream(&result, request.Install.Root, true)
		return TransactionOutcome{Install: &result}, installErr
	}
	hostPlanReq := request.Install
	hostPlanReq.DryRun = true
	hostPlan, hostPlanErr := effects.Install(hostPlanReq)
	if hostPlanErr != nil || !hostPlan.OK {
		if hostPlanErr == nil {
			hostPlanErr = fmt.Errorf("native installer dry-run preflight reported ok=false")
		}
		return hostPreflightFailure(hostPlan, hostPlanErr, preflight, request)
	}
	if hostPlanErr = effects.PlanShell(&hostPlan, hostPlanReq, request.PathMode); hostPlanErr != nil {
		return hostPreflightFailure(hostPlan, hostPlanErr, preflight, request)
	}
	hostTransaction, hostTransactionErr := effects.PrepareHost(hostPlan)
	if hostTransactionErr != nil {
		return hostPreflightFailure(hostPlan, hostTransactionErr, preflight, request)
	}
	if request.Step == "seal" {
		request.Activation.TransitionID = request.TransitionID
	} else {
		pending, beginErr := activation.Begin(ctx, request.Activation)
		if beginErr != nil {
			return TransactionOutcome{}, fmt.Errorf("begin native activation: %w", beginErr)
		}
		request.Activation.TransitionID = pending.TransitionID
		if request.Step == "begin" {
			return TransactionOutcome{Activation: &pending}, nil
		}
	}
	result := preflight
	result.TransitionID = request.Activation.TransitionID
	if applyErr := pathTransaction.Apply(&result); applyErr != nil {
		return failedInstall(ctx, result, applyErr, request, pathTransaction, hostTransaction, activation)
	}
	installed, installErr := effects.Install(request.Install)
	installed.Links = append(result.Links, installed.Links...)
	installed.CommandPath = result.CommandPath
	installed.TransitionID = request.Activation.TransitionID
	result = installed
	if installErr == nil && result.OK {
		installErr = effects.PlanShell(&result, request.Install, request.PathMode)
	}
	if installErr != nil || !result.OK {
		return failedInstall(ctx, result, installErr, request, pathTransaction, hostTransaction, activation)
	}
	sealed, sealErr := activation.Seal(ctx, request.Activation)
	if sealErr != nil || !sealed.OK || !sealed.Sealed || sealed.Receipt == nil {
		if sealErr == nil {
			sealErr = fmt.Errorf("native activation receipt was not sealed")
		}
		return failedInstall(ctx, result, sealErr, request, pathTransaction, hostTransaction, activation)
	}
	result.Committed = true
	result.Messages = append(result.Messages, "native activation receipt sealed after strict Codex/Claude/Omo/omp MCP and lifecycle readback")
	effects.AppendUpstream(&result, request.Install.Root, false)
	if finalizeErr := pathTransaction.Finalize(&result); finalizeErr != nil {
		result.Messages = append(result.Messages, "native activation is committed; command backup cleanup requires manual recovery: "+finalizeErr.Error())
		if result.CommandPath != nil {
			result.CommandPath.BackupRetained = true
		}
	}
	return TransactionOutcome{Install: &result}, nil
}

func hostPreflightFailure(result port.NativeInstallResult, cause error, preflight port.NativeInstallResult, request TransactionRequest) (TransactionOutcome, error) {
	result.OK = false
	result.Links = append(preflight.Links, result.Links...)
	result.CommandPath = preflight.CommandPath
	if request.Step == "seal" {
		result.TransitionID = request.TransitionID
		result.AbortRequired = true
		if result.CommandPath != nil {
			result.CommandPath.AbortRequired = true
		}
	}
	return TransactionOutcome{Install: &result}, cause
}

func failedInstall(ctx context.Context, result port.NativeInstallResult, cause error, request TransactionRequest, path PathTransaction, host HostTransaction, activation Activation) (TransactionOutcome, error) {
	result.OK = false
	if cause == nil {
		cause = fmt.Errorf("native installer reported ok=false")
	}
	hostRollbackErr := host.RollbackHosts()
	pathRollbackErr := path.Rollback(&result)
	if request.Step == "seal" {
		result.AbortRequired = true
		if result.CommandPath != nil {
			result.CommandPath.AbortRequired = true
		}
	} else {
		_, abortErr := activation.Abort(ctx, request.Activation)
		cause = errors.Join(cause, abortErr)
	}
	return TransactionOutcome{Install: &result}, errors.Join(cause, hostRollbackErr, pathRollbackErr)
}
