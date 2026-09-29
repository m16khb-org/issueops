package benchmark

import domain "issueops/internal/domain/issueopsbenchmark"

func EvaluateIssueOpsAutoresearchGate(req IssueOpsAutoresearchGateRequest) IssueOpsAutoresearchGateResult {
	return domain.EvaluateAutoresearchGate(req)
}
