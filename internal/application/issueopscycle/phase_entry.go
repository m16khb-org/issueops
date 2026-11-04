package issueopscycle

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func validatePhaseEntry(ready cycleport.PhaseEntryReadiness, record model.Record, phase model.Phase) error {
	if err := cycledomain.ValidatePhaseProgression(record.Phase, phase); err != nil {
		return err
	}
	if err := validateReadinessForPhase(ready, record, phase); err != nil {
		return err
	}
	switch phase {
	case model.PhaseFeedback:
		if strings.TrimSpace(record.AISlopCleanAt) == "" {
			return fmt.Errorf("cannot enter feedback phase before ai-slop-clean phase")
		}
	case model.PhaseDone:
		return validateDoneEntry(ready, record)
	}
	return nil
}

// validateReadinessForPhase checks the readiness probes that gate entering phase.
func validateReadinessForPhase(ready cycleport.PhaseEntryReadiness, record model.Record, phase model.Phase) error {
	missing := func(label string, probe func(model.Record) model.Readiness) error {
		if result := probe(record); !result.Ready {
			return fmt.Errorf("cannot enter %s phase: missing %s", label, strings.Join(result.Missing, ", "))
		}
		return nil
	}
	switch phase {
	case model.PhaseGrill:
		return missing("grill", ready.Problem)
	case model.PhasePlan:
		if err := missing("plan", ready.Plan); err != nil {
			return err
		}
		if result := ready.Grill(record); !result.Ready {
			return fmt.Errorf("cannot enter plan phase: grill incomplete: missing %s", strings.Join(result.Missing, ", "))
		}
	case model.PhaseCompatibilityReview:
		return missing("compatibility-review", ready.Compatibility)
	case model.PhaseImplement:
		return missing("implement", ready.Implement)
	case model.PhaseAISlopClean:
		return missing("ai-slop-clean", ready.AISlopClean)
	case model.PhasePR:
		return missing("pr", ready.StrictPR)
	}
	return nil
}

func validateDoneEntry(ready cycleport.PhaseEntryReadiness, record model.Record) error {
	if record.Phase != model.PhasePR {
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
