package issueopsreview

import model "issueops/internal/contract/issueops"

type DevilsAdvocateStore struct {
	Read       func(string, string) (model.IssueOpsRecord, error)
	TouchWrite func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	PlanDigest func(string, model.IssueOpsRecord) (string, error)
}

type CompatibilityStore struct {
	Read       func(string, string) (model.IssueOpsRecord, error)
	TouchWrite func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Ready      func(model.IssueOpsRecord) model.IssueOpsReadiness
	PhaseRank  func(model.IssueOpsPhase) int
}
