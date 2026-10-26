package remote

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxProviderTitleChars is the stricter provider title limit (GitLab 255
// characters, GitHub 256). A longer title is refused locally because the
// provider refusal arrives after the create intent is recorded, which leaves
// the outcome unknown and forces reconciliation.
const MaxProviderTitleChars = 255

func ValidateProviderTitleLength(kind, title string) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(title)); n > MaxProviderTitleChars {
		return fmt.Errorf("%s title has %d characters; GitLab allows %d and GitHub 256, so shorten --title", kind, n, MaxProviderTitleChars)
	}
	return nil
}

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
