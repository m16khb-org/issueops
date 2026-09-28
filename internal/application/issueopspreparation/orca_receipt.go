package issueopspreparation

import (
	"fmt"
	"strings"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

type OrcaReceiptDecision struct {
	Record   leasecontract.Record
	Intent   preparationcontract.Intent
	Complete bool
	Result   preparationcontract.Result
}

func ApplyOrcaReceipt(state IntentState, receipt preparationcontract.IntentReceipt, artifactDir string, validateDelivery func() error) (OrcaReceiptDecision, error) {
	intent := state.Intent
	intent.InvocationState = preparationcontract.InvocationNotInvoked
	intent.InvocationAttempts = 0
	record := state.Snapshot.Record
	execution := *record.Execution
	record.Execution = &execution
	if execution.Pending != nil {
		pending := *execution.Pending
		record.Execution.Pending = &pending
	}
	switch state.Intent.Stage {
	case preparationcontract.IntentStageWorktree:
		if receipt.Workspace == nil {
			return OrcaReceiptDecision{}, fmt.Errorf("Orca worktree candidate does not match the sealed intent")
		}
		prepared := *receipt.Workspace
		intent.Prepared = &prepared
		intent.Launch = &preparationcontract.LaunchIdentity{
			PromptPath: state.OwnerArtifacts.OwnerPromptPath, PromptSHA256: state.OwnerArtifacts.OwnerPromptSHA256,
			ContextPacketPath: state.OwnerArtifacts.ContextPacketPath, ContextPacketSHA256: state.OwnerArtifacts.ContextPacketSHA256,
		}
		intent.ClaimTokenSHA256 = state.OwnerArtifacts.ClaimTokenSHA256
		intent.Stage = preparationcontract.IntentStageTerminal
		record.WorktreePath = prepared.Workspace.Root
		record.PlanPath = state.OwnerArtifacts.PlanPath
		record.Execution.Workspace = leasecontract.Workspace{
			SourceRoot: prepared.Workspace.SourceRoot, Root: prepared.Workspace.Root,
			Branch: prepared.Workspace.Branch, BaseHead: prepared.Workspace.BaseHead,
			ParentWorktree: prepared.Workspace.ParentWorktree, Driver: prepared.Workspace.Driver,
			LinkedAt: state.Intent.StartedAt, ArtifactDir: artifactDir,
		}
	case preparationcontract.IntentStageTerminal:
		if strings.TrimSpace(receipt.TerminalPTYID) == "" {
			return OrcaReceiptDecision{}, fmt.Errorf("Orca terminal candidate is incomplete")
		}
		intent.TerminalPTYID = strings.TrimSpace(receipt.TerminalPTYID)
		intent.Stage = preparationcontract.IntentStageRun
	case preparationcontract.IntentStageRun:
		if strings.TrimSpace(receipt.RunID) == "" {
			return OrcaReceiptDecision{}, fmt.Errorf("Orca Run candidate is incomplete")
		}
		intent.RunID = strings.TrimSpace(receipt.RunID)
		intent.Stage = preparationcontract.IntentStageRunBind
	case preparationcontract.IntentStageRunBind:
		if strings.TrimSpace(receipt.RunID) != state.Intent.RunID || !receipt.RunBound {
			return OrcaReceiptDecision{}, fmt.Errorf("Orca Run binding candidate is incomplete")
		}
		intent.RunBound = true
		intent.Stage = preparationcontract.IntentStageTask
	case preparationcontract.IntentStageTask:
		if strings.TrimSpace(receipt.TaskID) == "" {
			return OrcaReceiptDecision{}, fmt.Errorf("Orca task candidate is incomplete")
		}
		intent.TaskID = strings.TrimSpace(receipt.TaskID)
		intent.Stage = preparationcontract.IntentStageDispatch
	case preparationcontract.IntentStageDispatch:
		if err := validateDelivery(); err != nil {
			return OrcaReceiptDecision{}, fmt.Errorf("Orca dispatch candidate is incomplete: %w", err)
		}
		if state.Intent.Prepared == nil {
			return OrcaReceiptDecision{}, fmt.Errorf("Orca prepared workspace receipt is missing")
		}
		if state.Intent.Launch == nil {
			return OrcaReceiptDecision{}, fmt.Errorf("Orca sealed owner artifact identity is missing")
		}
		intent.OrcaRequestID = strings.TrimSpace(receipt.RequestID)
		if receipt.PromptReceipt != nil {
			intent.OrcaPromptRequestID = strings.TrimSpace(receipt.PromptReceipt.RequestID)
		}
		record.Execution.Lease = leasecontract.Lease{Generation: state.Intent.Generation, Status: "claimable", ClaimTokenSHA256: state.Intent.ClaimTokenSHA256}
		record.Execution.Orca = &leasecontract.OrcaBinding{
			RuntimeID: state.Intent.Prepared.RuntimeID, RepoID: state.Intent.Prepared.RepoID,
			WorktreeID: state.Intent.Prepared.WorktreeID, WorktreeInstanceID: state.Intent.Prepared.WorktreeInstanceID,
			LeaseGeneration: state.Intent.Generation, OwnerHost: state.Intent.Probe.Host,
			ArtifactIdentityVersion: leasecontract.OrcaArtifactIdentityVersion,
			IssueBodySHA256:         state.Intent.IssueBodySHA256, ContextPacketSHA256: state.Intent.Launch.ContextPacketSHA256,
			OwnerPromptSHA256: state.Intent.Launch.PromptSHA256,
			OwnerModel:        state.Intent.Probe.Model, OwnerEffort: state.Intent.Probe.Effort,
			RunID: state.Intent.RunID, TaskID: state.Intent.TaskID, DispatchID: strings.TrimSpace(receipt.DispatchID),
			TerminalPTYID: state.Intent.TerminalPTYID,
		}
		record.Execution.Pending = nil
		record.Execution.Failure = nil
		result := preparationcontract.Result{
			OK: true, ID: record.ID, ResolvedMode: preparationcontract.ModeOrca,
			Workspace: record.Execution.Workspace, Execution: record.Execution,
			ClaimTokenPath: state.OwnerArtifacts.ClaimTokenPath, IssueBodySHA256: state.Intent.IssueBodySHA256,
			ContextPacketPath: state.OwnerArtifacts.ContextPacketPath, ContextPacketSHA256: state.OwnerArtifacts.ContextPacketSHA256,
			OwnerPromptPath: state.OwnerArtifacts.OwnerPromptPath, OwnerPromptSHA256: state.OwnerArtifacts.OwnerPromptSHA256,
			IssueSnapshotSource: state.Owner.Source,
		}
		return OrcaReceiptDecision{Record: record, Intent: intent, Complete: true, Result: result}, nil
	default:
		return OrcaReceiptDecision{}, fmt.Errorf("unsupported Orca intent stage %q", state.Intent.Stage)
	}
	record.Execution.Pending.Kind = preparationdomain.PendingKind(intent.Stage)
	record.Execution.Failure = nil
	return OrcaReceiptDecision{Record: record, Intent: intent}, nil
}
