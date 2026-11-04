package issueopsreview

import "fmt"

// ValidateAISlopCleanEvidence rejects ai-slop-clean evidence that lacks a cleanup
// category or a verification entry; the error text names the CLI flag to add.
func ValidateAISlopCleanEvidence(categories, verification []string) error {
	if len(categories) == 0 {
		return fmt.Errorf("ai-slop-clean evidence requires at least one cleanup category (--category NAME, repeatable)")
	}
	if len(verification) == 0 {
		return fmt.Errorf("ai-slop-clean evidence requires at least one verification entry (--verification TEXT)")
	}
	return nil
}
