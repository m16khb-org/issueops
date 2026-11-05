package issueopsremote

// GateLedgerIssueNumber returns the issue number of linkedURL, falling back to preparedURL. It returns "" when neither carries one.
func GateLedgerIssueNumber(linkedURL, preparedURL string) string {
	if number := issueNumber(linkedURL); number != "" {
		return number
	}
	return issueNumber(preparedURL)
}
