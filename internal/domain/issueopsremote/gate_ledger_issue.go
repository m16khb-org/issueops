package remote

func GateLedgerIssueNumber(linkedURL, preparedURL string) string {
	if number := IssueNumber(linkedURL); number != "" {
		return number
	}
	return IssueNumber(preparedURL)
}
