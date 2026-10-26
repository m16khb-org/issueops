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

// Provider body limits in characters: GitHub issue and PR bodies allow 65536
// code points, GitLab descriptions 1048576 characters. issueops appends its
// create marker after this check, so createMarkerReserve characters stay free.
const (
	githubBodyChars     = 65536
	gitlabBodyChars     = 1 << 20
	createMarkerReserve = 128
)

// ValidateProviderBodyLength refuses a create body the provider would reject
// only after the create intent is recorded.
func ValidateProviderBodyLength(provider, kind, body string) error {
	name, limit := "GitLab", gitlabBodyChars
	if strings.EqualFold(strings.TrimSpace(provider), "github") {
		name, limit = "GitHub", githubBodyChars
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(body)); n > limit-createMarkerReserve {
		return fmt.Errorf("%s body has %d characters; %s allows %d and issueops reserves %d for its marker, so shorten the body", kind, n, name, limit, createMarkerReserve)
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
