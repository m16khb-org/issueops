package issueopscycle

import model "issueops/internal/contract/issueops"

func localPRForTest(s Readiness, record model.IssueOpsRecord) model.IssueOpsReadiness {
	ready, _ := s.ObserveLocalPR(record)
	return ready
}

func strictPRForTest(s Readiness, record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	ready, _ := s.observePR(record, s.Git.Fetch)
	return ready
}
