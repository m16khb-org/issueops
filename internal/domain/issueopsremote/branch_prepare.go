package issueopsremote

import (
	"fmt"
	"strings"
)

// PreparationProvider returns provider, falling back to the one inferred from issueURL. It errors unless the result is github or gitlab.
func PreparationProvider(provider, issueURL string) (string, error) {
	if provider == "" {
		provider = ProviderFromURL(issueURL)
	}
	if provider != "github" && provider != "gitlab" {
		return "", fmt.Errorf("provider must be github or gitlab")
	}
	return provider, nil
}

// ValidatePreparationIssueNumber requires branch to start with "<issue number>-" when issueURL carries an issue number; a URL without a number accepts any branch.
func ValidatePreparationIssueNumber(issueURL, branch string) error {
	if number := issueNumber(issueURL); number != "" && !strings.HasPrefix(branch, number+"-") {
		return fmt.Errorf("issueops branch for issue %s must start with %s-; for example %s-fix-login", number, number, number)
	}
	return nil
}

// PreparationCodeProject chooses the code project key to seal for a prepared branch. A requested key must be canonical; otherwise the observed origin key is used when the observation succeeded and the key is valid. It returns "" when the code project equals the issue's own project, meaning no separate code project.
func PreparationCodeProject(provider, issueURL, requested, observed string, observationErr error) (string, error) {
	issueProject := ProjectKey(issueURL, provider, "issue")
	if declared := strings.TrimSpace(requested); declared != "" {
		if !validProjectKey(declared) {
			return "", fmt.Errorf("code_project_key %q must be a canonical provider project key such as example.com/group/project", declared)
		}
		if declared == issueProject {
			return "", nil
		}
		return declared, nil
	}
	observed = strings.TrimSpace(observed)
	if observationErr != nil || observed == "" || observed == issueProject || !validProjectKey(observed) {
		return "", nil
	}
	return observed, nil
}
