package issueopsreview

import "fmt"

func ValidateAISlopCleanEvidence(categories, verification []string) error {
	if len(categories) == 0 {
		return fmt.Errorf("ai-slop-clean evidence requires at least one cleanup category (--category NAME, repeatable)")
	}
	if len(verification) == 0 {
		return fmt.Errorf("ai-slop-clean evidence requires at least one verification entry (--verification TEXT)")
	}
	return nil
}
