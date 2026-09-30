package issueopspreparation

import (
	"fmt"
	"strings"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

type ResumeReceiptDecision struct {
	Record   preparationcontract.Record
	Intent   preparationcontract.Intent
	Complete bool
}

func ApplyResumeReceipt(record preparationcontract.Record, intent preparationcontract.Intent, receipt preparationcontract.IntentReceipt, deliveryErr error) (ResumeReceiptDecision, error) {
	if err := ValidateIntentRecordAuthority(record, intent); err != nil {
		return ResumeReceiptDecision{}, err
	}
	if err := ValidateReconcileIntentIssueIdentity(record, intent); err != nil {
		return ResumeReceiptDecision{}, err
	}
	if normalizedPurpose(intent) != preparationcontract.PurposeResume {
		return ResumeReceiptDecision{}, fmt.Errorf("Orca resume intent is required")
	}
	nextStage, complete, err := NextOrcaReceiptStage(intent.Stage)
	if err != nil {
		return ResumeReceiptDecision{}, err
	}
	if intent.Stage == preparationcontract.IntentStageWorktree {
		return ResumeReceiptDecision{}, fmt.Errorf("Orca resume intent cannot create a worktree")
	}
	nextIntent := intent
	nextIntent.Stage = nextStage
	nextIntent.InvocationState = preparationcontract.InvocationNotInvoked
	nextIntent.InvocationAttempts = 0
	nextRecord := record
	execution := *record.Execution
	nextRecord.Execution = &execution
	if execution.Pending != nil {
		pending := *execution.Pending
		nextRecord.Execution.Pending = &pending
	}
	switch intent.Stage {
	case preparationcontract.IntentStageTerminal:
		if strings.TrimSpace(receipt.TerminalPTYID) == "" {
			return ResumeReceiptDecision{}, fmt.Errorf("Orca terminal candidate is incomplete")
		}
		nextIntent.TerminalPTYID = strings.TrimSpace(receipt.TerminalPTYID)
	case preparationcontract.IntentStageRun:
		if strings.TrimSpace(receipt.RunID) == "" {
			return ResumeReceiptDecision{}, fmt.Errorf("Orca Run candidate is incomplete")
		}
		nextIntent.RunID = strings.TrimSpace(receipt.RunID)
	case preparationcontract.IntentStageRunBind:
		if strings.TrimSpace(receipt.RunID) != intent.RunID || !receipt.RunBound {
			return ResumeReceiptDecision{}, fmt.Errorf("Orca Run binding candidate is incomplete")
		}
		nextIntent.RunBound = true
	case preparationcontract.IntentStageTask:
		if strings.TrimSpace(receipt.TaskID) == "" {
			return ResumeReceiptDecision{}, fmt.Errorf("Orca task candidate is incomplete")
		}
		nextIntent.TaskID = strings.TrimSpace(receipt.TaskID)
	case preparationcontract.IntentStageDispatch:
		if deliveryErr != nil {
			return ResumeReceiptDecision{}, fmt.Errorf("Orca dispatch candidate is incomplete: %w", deliveryErr)
		}
		if intent.Prepared == nil || intent.Launch == nil {
			return ResumeReceiptDecision{}, fmt.Errorf("Orca sealed owner artifact identity is missing")
		}
		nextIntent.OrcaRequestID = strings.TrimSpace(receipt.RequestID)
		if receipt.PromptReceipt != nil {
			nextIntent.OrcaPromptRequestID = strings.TrimSpace(receipt.PromptReceipt.RequestID)
		}
		nextRecord.Execution.Orca = &preparationcontract.OrcaBinding{
			RuntimeID: intent.Prepared.RuntimeID, RepoID: intent.Prepared.RepoID,
			WorktreeID: intent.Prepared.WorktreeID, WorktreeInstanceID: intent.Prepared.WorktreeInstanceID,
			LeaseGeneration: intent.Generation, OwnerHost: intent.Probe.Host,
			ArtifactIdentityVersion: preparationcontract.OrcaArtifactIdentityVersion,
			IssueBodySHA256:         intent.IssueBodySHA256, ContextPacketSHA256: intent.Launch.ContextPacketSHA256,
			OwnerPromptSHA256: intent.Launch.PromptSHA256,
			OwnerModel:        intent.Probe.Model, OwnerEffort: intent.Probe.Effort,
			RunID: intent.RunID, TaskID: intent.TaskID, DispatchID: receipt.DispatchID,
			TerminalPTYID: intent.TerminalPTYID,
		}
		nextRecord.Execution.Pending = nil
		nextRecord.Execution.Failure = nil
	}
	if !complete {
		nextRecord.Execution.Pending.Kind = PendingKind(nextIntent.Stage)
		nextRecord.Execution.Failure = nil
	}
	return ResumeReceiptDecision{Record: nextRecord, Intent: nextIntent, Complete: complete}, nil
}
