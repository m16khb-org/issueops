package issueops

import (
	"fmt"
)

// recordValidators runs in declaration order; the first error wins.
var recordValidators = []func(*IssueOpsRecord) error{
	validateRecordHeader,
	validateRecordPhaseLedger,
	validateRecordIntent,
	validateRecordFeedback,
	validateRecordRegressEvents,
	validateRecordRoutingTrace,
	validateRecordBranchPrepare,
	validateRecordRemoteArtifact,
	validateRecordChildCycles,
	validateRecordCleanupAttempt,
	validateRecordCleanupFinishFailure,
	validateRecordCleanupAbandonFailure,
	validateRecordChildCreateOperations,
	validateRecordIssueCreateIntent,
}

// ValidateRecord checks persisted field shapes; cross-field lifecycle rules
// belong to domain/issueops.ValidateRecordInvariants.
func ValidateRecord(record IssueOpsRecord) error {
	for _, validate := range recordValidators {
		if err := validate(&record); err != nil {
			return err
		}
	}
	return nil
}

func validateRecordHeader(record *IssueOpsRecord) error {
	if record.SchemaVersion != IssueOpsSchemaVersion {
		return fmt.Errorf("issueops record schema version is invalid")
	}
	if !knownRecordPhase(record.Phase) {
		return fmt.Errorf("issueops record phase %q is invalid", record.Phase)
	}
	return nil
}

func validateRecordPhaseLedger(record *IssueOpsRecord) error {
	for phase, entry := range record.PhaseLedger {
		if !knownRecordPhase(phase) || entry.Phase != phase {
			return fmt.Errorf("issueops phase ledger identity is invalid")
		}
	}
	return nil
}

func validateRecordIntent(record *IssueOpsRecord) error {
	if record.Intent == nil {
		return nil
	}
	_, err := NormalizeIntentClass(record.Intent.IntentClass)
	return err
}

func validateRecordFeedback(record *IssueOpsRecord) error {
	for _, feedback := range record.Feedback {
		if !KnownFeedbackClassification(feedback.Classification) {
			return fmt.Errorf("issueops feedback classification is invalid")
		}
		if !KnownFeedbackResolution(feedback.Resolution) {
			return fmt.Errorf("issueops feedback resolution is invalid")
		}
	}
	return nil
}

func validateRecordRegressEvents(record *IssueOpsRecord) error {
	for _, event := range record.RegressEvents {
		if !knownRecordPhase(event.FromPhase) {
			return fmt.Errorf("issueops regress event phase is invalid")
		}
	}
	return nil
}

func validateRecordRoutingTrace(record *IssueOpsRecord) error {
	for _, routing := range record.RoutingTrace {
		if !knownRecordPhase(IssueOpsPhase(routing.Phase)) {
			return fmt.Errorf("issueops routing phase is invalid")
		}
	}
	return nil
}

func validateRecordBranchPrepare(record *IssueOpsRecord) error {
	if record.BranchPrepare != nil &&
		record.BranchPrepare.Provider != "github" &&
		record.BranchPrepare.Provider != "gitlab" {
		return fmt.Errorf("issueops branch provider is invalid")
	}
	return nil
}

func validateRecordRemoteArtifact(record *IssueOpsRecord) error {
	if record.RemoteArtifact != nil &&
		!knownRemoteArtifactIdentity(record.RemoteArtifact.Provider, record.RemoteArtifact.Kind) {
		return fmt.Errorf("issueops remote artifact identity is invalid")
	}
	return nil
}

func validateRecordChildCycles(record *IssueOpsRecord) error {
	for _, child := range record.ChildCycles {
		if !knownChildValidationVerdict(child.ValidationVerdict) {
			return fmt.Errorf("issueops child validation verdict is invalid")
		}
	}
	return nil
}

func validateRecordCleanupAttempt(record *IssueOpsRecord) error {
	return ValidateCleanupAttempt(record.CleanupAttempt)
}

func validateRecordCleanupFinishFailure(record *IssueOpsRecord) error {
	if record.CleanupFinishFailure != nil &&
		!knownCleanupFinishFailureStep(record.CleanupFinishFailure.Step) {
		return fmt.Errorf("issueops cleanup finish failure step is invalid")
	}
	return nil
}

func validateRecordCleanupAbandonFailure(record *IssueOpsRecord) error {
	if record.CleanupAbandonFailure != nil &&
		!knownCleanupAbandonFailureStep(record.CleanupAbandonFailure.Step) {
		return fmt.Errorf("issueops cleanup abandon failure step is invalid")
	}
	return nil
}

func validateRecordChildCreateOperations(record *IssueOpsRecord) error {
	return ValidateChildCreateOperations(record.ChildCreateOperations)
}

func validateRecordIssueCreateIntent(record *IssueOpsRecord) error {
	if record.IssueCreateIntent == nil {
		return nil
	}
	return ValidateIssueCreateIntent(*record.IssueCreateIntent)
}

func KnownFeedbackClassification(classification string) bool {
	switch classification {
	case "",
		"contract_change",
		"defect",
		"question",
		"noise",
		"valid_review",
		"stale_review",
		"rollout_evidence_missing",
		"environment_debt":
		return true
	default:
		return false
	}
}

func KnownFeedbackResolution(resolution string) bool {
	switch resolution {
	case "", "valid-defect", "question-answered", "noise-dismissed":
		return true
	default:
		return false
	}
}

func knownRemoteArtifactIdentity(provider, kind string) bool {
	return provider == "github" && kind == "pr" ||
		provider == "gitlab" && kind == "mr"
}

func knownChildValidationVerdict(verdict string) bool {
	switch verdict {
	case "", "accepted", "rejected", "dropped":
		return true
	default:
		return false
	}
}

func knownCleanupFinishFailureStep(step string) bool {
	switch step {
	case CleanupFailureStepWorkspaceProcessesStop,
		CleanupFailureStepOrcaRemove,
		CleanupFailureStepWorktreeRemove,
		CleanupFailureStepBranchDelete,
		CleanupFailureStepRecordDelete:
		return true
	default:
		return false
	}
}

func knownCleanupAbandonFailureStep(step string) bool {
	switch step {
	case CleanupFailureStepApplying,
		CleanupFailureStepClosePR,
		CleanupFailureStepCloseIssue,
		CleanupFailureStepRemoteBranchDelete,
		CleanupFailureStepWorkspaceProcessesStop,
		CleanupFailureStepWorktreeRemove,
		CleanupFailureStepBranchDelete,
		CleanupFailureStepRecordDelete:
		return true
	default:
		return false
	}
}

func knownRecordPhase(phase IssueOpsPhase) bool {
	for _, known := range IssueOpsPhases {
		if phase == known {
			return true
		}
	}
	return false
}
