package issueops

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateIssueURL requires an http(s) issue URL without a query or anchor
// that points at an issue, not a pull or merge request, and whose issue
// number is numeric.
func ValidateIssueURL(issueURL string) error {
	if issueURL == "" {
		return fmt.Errorf("issue_url is required")
	}
	parsed, err := url.Parse(issueURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("issue_url must be an http(s) URL")
	}
	// Probe: link-issue accepted a pull request URL, a comment anchor, and
	// /issues/abc; each fails only later (branch steps resolve the number,
	// readbacks compare the exact URL).
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		clean := *parsed
		clean.RawQuery, clean.Fragment = "", ""
		return fmt.Errorf("issue_url %s carries a query or #anchor; pass the issue URL itself: %s", issueURL, clean.String())
	}
	return validateIssueURLPath(issueURL, strings.Split(strings.Trim(parsed.Path, "/"), "/"))
}

// validateIssueURLPath는 경로 조각이 PR/MR이 아니라 이슈 번호로 끝나는 이슈 경로인지 검사한다.
func validateIssueURLPath(issueURL string, parts []string) error {
	for i, part := range parts {
		switch part {
		case "pull", "pulls", "merge_requests":
			return fmt.Errorf("issue_url %s is a pull/merge request, not an issue; link the issue it closes", issueURL)
		case "issues", "work_items":
			if i+1 < len(parts) && !allDigits(parts[i+1]) {
				return fmt.Errorf("issue_url %s does not end in an issue number after /%s/", issueURL, part)
			}
		}
	}
	return nil
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
