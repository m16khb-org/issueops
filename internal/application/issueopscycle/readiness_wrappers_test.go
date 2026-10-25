package issueopscycle

import model "issueops/internal/contract/issueops"

func strictPRForTest(s Readiness, record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	ready, _ := s.observePR(record, s.Git.Fetch)
	return ready
}
