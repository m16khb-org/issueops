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
	if phase == model.IssueOpsPhaseGrill {
		if result := ready.Problem(record); !result.Ready {
			return fmt.Errorf("cannot enter grill phase: missing %s", strings.Join(result.Missing, ", "))
		}
	}
	if phase == model.IssueOpsPhasePlan {
		if result := ready.Plan(record); !result.Ready {
			return fmt.Errorf("cannot enter plan phase: missing %s", strings.Join(result.Missing, ", "))
		}
		if result := ready.Grill(record); !result.Ready {
			return fmt.Errorf("cannot enter plan phase: grill incomplete: missing %s", strings.Join(result.Missing, ", "))
		}
	}
	if phase == model.IssueOpsPhaseCompatibilityReview {
		if result := ready.Compatibility(record); !result.Ready {
			return fmt.Errorf("cannot enter compatibility-review phase: missing %s", strings.Join(result.Missing, ", "))
		}
	}
	if phase == model.IssueOpsPhaseImplement {
		if result := ready.Implement(record); !result.Ready {
			return fmt.Errorf("cannot enter implement phase: missing %s", strings.Join(result.Missing, ", "))
		}
	}
	if phase == model.IssueOpsPhaseAISlopClean {
		if result := ready.AISlopClean(record); !result.Ready {
			return fmt.Errorf("cannot enter ai-slop-clean phase: missing %s", strings.Join(result.Missing, ", "))
		}
	}
	if phase == model.IssueOpsPhaseFeedback && strings.TrimSpace(record.AISlopCleanAt) == "" {
		return fmt.Errorf("cannot enter feedback phase before ai-slop-clean phase")
	}
	if phase == model.IssueOpsPhasePR {
		if result := ready.StrictPR(record); !result.Ready {
			return fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(result.Missing, ", "))
		}
	}
	if phase == model.IssueOpsPhaseDone && record.Phase != model.IssueOpsPhasePR {
		return fmt.Errorf("cannot enter done phase before pr phase")
	}
	if phase == model.IssueOpsPhaseDone {
		if missing := ready.RemoteArtifactMissing(record); len(missing) > 0 {
			return fmt.Errorf("cannot enter done phase before remote artifact verification: missing %s", strings.Join(missing, ", "))
		}
		if record.Execution == nil || record.Execution.Completion == nil || record.Execution.Lease.Status != model.LeaseStatusReleased {
			return fmt.Errorf("cannot enter done phase before issueops execution completion")
		}
	}
	return nil
}
