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

// EvidenceReviewStore supplies observations and persistence without owning review policy.
type EvidenceReviewStore struct {
	Read             func(string, string) (model.IssueOpsRecord, error)
	Fingerprint      func(model.IssueOpsRecord) string
	WithLock         func(string, string, func() error) error
	ValidateMutation func(model.IssueOpsRecord) error
	Write            func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Now              func() string
}

type ProjectDocsReviewStore struct {
	EvidenceReviewStore
	ChangedPaths func(model.IssueOpsRecord) []string
	Root         func(model.IssueOpsRecord) string
	RelativePath func(string, string) string
	FileExists   func(string, string) bool
}

type ReviewMutationStore struct {
	WithLock         func(string, string, func() error) error
	Read             func(string, string) (model.IssueOpsRecord, error)
	ValidateMutation func(model.IssueOpsRecord) error
	Write            func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Now              func() string
}

type AISlopCleanStore struct {
	ReviewMutationStore
	Refresh func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
}

type DesignReviewStore struct {
	Read          func(string, string) (model.IssueOpsRecord, error)
	PlanReadiness func(model.IssueOpsRecord) model.IssueOpsReadiness
	TouchWrite    func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Now           func() string
}
