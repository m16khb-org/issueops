package remote

import "fmt"

func ValidateConfirmCreateMetadata(confirm bool, labels, assignees []string) error {
	if !confirm {
		return nil
	}
	labels = CleanValues(labels)
	assignees = CleanValues(assignees)
	if len(labels) == 0 {
		return fmt.Errorf("at least one label is required with --confirm")
	}
	if len(assignees) == 0 {
		return fmt.Errorf("at least one assignee is required with --confirm")
	}
	return nil
}
