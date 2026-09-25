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

type RegressStore struct {
	Read           func(string, string) (model.IssueOpsRecord, error)
	ActiveChildren func(string, model.IssueOpsRecord) ([]string, error)
	TouchWrite     func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Now            func() string
}

// ImplementationReviewStore supplies observations and persistence without owning review policy.
type ImplementationReviewStore struct {
	Read             func(string, string) (model.IssueOpsRecord, error)
	Fingerprint      func(model.IssueOpsRecord) string
	WithLock         func(string, string, func() error) error
	ValidateMutation func(model.IssueOpsRecord) error
	Write            func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Now              func() string
}
