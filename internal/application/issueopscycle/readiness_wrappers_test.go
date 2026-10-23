package issueopscycle

import model "issueops/internal/contract/issueops"

func localPRForTest(s Readiness, record model.IssueOpsRecord) model.IssueOpsReadiness {
	ready, _ := s.ObserveLocalPR(record)
	return ready
}

func strictPRForTest(s Readiness, record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	ready, _ := s.observePR(record, s.Git.Fetch, s.PR(record))
	return ready
}

// observedPRForTest is the fetch-free observation with the cleanup status
// pr-readiness prints, so scope tests see both Git and cleanup observations.
func observedPRForTest(s Readiness, record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	ready, _ := s.observePR(record, nil, s.PR(record))
	return ready
}
