package remote

import (
	"fmt"
	"strings"
)

func PreparationProvider(provider, issueURL string) (string, error) {
	if provider == "" {
		provider = ProviderFromURL(issueURL)
	}
	if provider != "github" && provider != "gitlab" {
		return "", fmt.Errorf("provider must be github or gitlab")
	}
	return provider, nil
}

func ValidatePreparationIssueNumber(issueURL, branch string) error {
	if number := IssueNumber(issueURL); number != "" && !strings.HasPrefix(branch, number+"-") {
		return fmt.Errorf("issueops branch for issue %s must start with %s-; for example %s-fix-login", number, number, number)
	}
	return nil
}

func PreparationCodeProject(provider, issueURL, requested, observed string, observationErr error) (string, error) {
	issueProject := ProjectKey(issueURL, provider, "issue")
	if declared := strings.TrimSpace(requested); declared != "" {
		if !ValidProjectKey(declared) {
			return "", fmt.Errorf("code_project_key %q must be a canonical provider project key such as example.com/group/project", declared)
		}
		if declared == issueProject {
			return "", nil
		}
		return declared, nil
	}
	observed = strings.TrimSpace(observed)
	if observationErr != nil || observed == "" || observed == issueProject || !ValidProjectKey(observed) {
		return "", nil
	}
	return observed, nil
}
