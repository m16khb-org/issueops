package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
	"slices"
	"sort"
	"strings"
)

// NeedsPRGateRead reports whether moving to phase to requires reading the PR
// gate readiness, which holds only for the pr phase.
func NeedsPRGateRead(to string) bool {
	return model.Phase(strings.TrimSpace(to)) == model.PhasePR
}

// PRGateError returns nil for a ready record and otherwise an error listing
// the gates that block entering the pr phase.
func PRGateError(ready model.Readiness) error {
	if ready.Ready {
		return nil
	}
	return fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
}

// MergeGateReadiness adds the extra missing gates and warnings to ready. The
// merged missing list is sorted and deduplicated, and Ready becomes true only
// when none remain. It returns ready unchanged when there is nothing to add.
func MergeGateReadiness(ready model.Readiness, missing, warnings []string) model.Readiness {
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
