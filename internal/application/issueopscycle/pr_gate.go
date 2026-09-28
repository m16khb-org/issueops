package issueopscycle

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/stringlist"
	cycleport "issueops/internal/port/issueopscycle"
)

func ApplyLoopGate(ready model.IssueOpsReadiness, repo string, observe func(repo string) ([]string, []string)) model.IssueOpsReadiness {
	missing, warnings := observe(repo)
	if len(missing) == 0 && len(warnings) == 0 {
		return ready
	}
	ready.Missing = stringlist.UniqueSorted(append(append([]string{}, ready.Missing...), missing...))
	ready.Warnings = append(ready.Warnings, warnings...)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func GuardPRPhase(stateRoot, id, to string, ports cycleport.PRPhaseGuard) error {
	if model.IssueOpsPhase(strings.TrimSpace(to)) != model.IssueOpsPhasePR {
		return nil
	}
	record, err := ports.Read(stateRoot, id)
	if err != nil {
		return err
	}
	if record.Phase == model.IssueOpsPhasePR {
		return nil
	}
	if ready := ports.Gate(record); !ready.Ready {
		return fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
	}
	return nil
}
