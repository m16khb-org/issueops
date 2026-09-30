package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
	"slices"
	"sort"
	"strings"
)

func NeedsPRGateRead(to string) bool {
	return model.IssueOpsPhase(strings.TrimSpace(to)) == model.IssueOpsPhasePR
}
func NeedsPRGateEvaluation(phase model.IssueOpsPhase) bool { return phase != model.IssueOpsPhasePR }
func PRGateError(ready model.IssueOpsReadiness) error {
	if ready.Ready {
		return nil
	}
	return fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
}
func MergeGateReadiness(ready model.IssueOpsReadiness, missing, warnings []string) model.IssueOpsReadiness {
	if len(missing) == 0 && len(warnings) == 0 {
		return ready
	}
	ready.Missing = gateMissingKeys(append(append([]string{}, ready.Missing...), missing...))
	ready.Warnings = append(ready.Warnings, warnings...)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func gateMissingKeys(values []string) []string {
	keys := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			keys = append(keys, value)
		}
	}
	sort.Strings(keys)
	return slices.Compact(keys)
}
