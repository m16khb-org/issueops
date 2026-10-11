package issueopscycle

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func ValidatePhaseEntry(ready cycleport.PhaseEntryReadiness, record model.IssueOpsRecord, phase model.IssueOpsPhase) error {
	if err := cycledomain.ValidatePhaseProgression(record.Phase, phase); err != nil {
		return err
	}
	if err := validateReadinessForPhase(ready, record, phase); err != nil {
		return err
	}
	switch phase {
	case model.IssueOpsPhaseFeedback:
		if strings.TrimSpace(record.AISlopCleanAt) == "" {
			return fmt.Errorf("cannot enter feedback phase before ai-slop-clean phase")
		}
	case model.IssueOpsPhaseDone:
		return validateDoneEntry(ready, record)
	}
	return nil
}

// validateReadinessForPhase checks the readiness probes that gate entering phase.
func validateReadinessForPhase(ready cycleport.PhaseEntryReadiness, record model.IssueOpsRecord, phase model.IssueOpsPhase) error {
	missing := func(label string, probe func(model.IssueOpsRecord) model.IssueOpsReadiness) error {
		if result := probe(record); !result.Ready {
			return fmt.Errorf("cannot enter %s phase: missing %s", label, strings.Join(result.Missing, ", "))
		}
		return nil
	}
	switch phase {
	case model.IssueOpsPhaseGrill:
		return missing("grill", ready.Problem)
	case model.IssueOpsPhasePlan:
		if err := missing("plan", ready.Plan); err != nil {
			return err
		}
		if result := ready.Grill(record); !result.Ready {
			return fmt.Errorf("cannot enter plan phase: grill incomplete: missing %s", strings.Join(result.Missing, ", "))
		}
	case model.IssueOpsPhaseCompatibilityReview:
		return missing("compatibility-review", ready.Compatibility)
	case model.IssueOpsPhaseImplement:
		return missing("implement", ready.Implement)
	case model.IssueOpsPhaseAISlopClean:
		return missing("ai-slop-clean", ready.AISlopClean)
	case model.IssueOpsPhasePR:
		return missing("pr", ready.StrictPR)
	}
	return nil
}

func validateDoneEntry(ready cycleport.PhaseEntryReadiness, record model.IssueOpsRecord) error {
	if record.Phase != model.IssueOpsPhasePR {
		return fmt.Errorf("cannot enter done phase before pr phase")
	}
	if missing := ready.RemoteArtifactMissing(record); len(missing) > 0 {
		return fmt.Errorf("cannot enter done phase before remote artifact verification: missing %s", strings.Join(missing, ", "))
	}
	if record.Execution == nil || record.Execution.Completion == nil || record.Execution.Lease.Status != model.LeaseStatusReleased {
		return fmt.Errorf("cannot enter done phase before issueops execution completion")
	}
	return nil
}
