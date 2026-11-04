package issueopsreview

import model "issueops/internal/contract/issueops"

type DevilsAdvocateStore struct {
	Read       func(string, string) (model.Record, error)
	TouchWrite func(string, model.Record) (model.Record, error)
	PlanDigest func(string, model.Record) (string, error)
}

type CompatibilityStore struct {
	Read       func(string, string) (model.Record, error)
	TouchWrite func(string, model.Record) (model.Record, error)
	Ready      func(model.Record) model.Readiness
	PhaseRank  func(model.Phase) int
}

type RegressStore struct {
	Read           func(string, string) (model.Record, error)
	ActiveChildren func(string, model.Record) ([]string, error)
	TouchWrite     func(string, model.Record) (model.Record, error)
	Now            func() string
}

// EvidenceReviewStore supplies observations and persistence without owning review policy.
type EvidenceReviewStore struct {
	Read             func(string, string) (model.Record, error)
	Fingerprint      func(model.Record) string
	WithLock         func(string, string, func() error) error
	ValidateMutation func(model.Record) error
	Write            func(string, model.Record) (model.Record, error)
	Now              func() string
}

type ProjectDocsReviewStore struct {
	EvidenceReviewStore
	ChangedPaths func(model.Record) []string
	Root         func(model.Record) string
	RelativePath func(string, string) string
	FileExists   func(string, string) bool
	// ProjectDocsPresent reports whether the root has any project doc.
	ProjectDocsPresent func(string) bool
}

type ReviewMutationStore struct {
	WithLock         func(string, string, func() error) error
	Read             func(string, string) (model.Record, error)
	ValidateMutation func(model.Record) error
	Write            func(string, model.Record) (model.Record, error)
	Now              func() string
}

type AISlopCleanStore struct {
	ReviewMutationStore
	Refresh func(string, model.Record) (model.Record, error)
}

type ArchitectureReviewStore struct {
	Read          func(string, string) (model.Record, error)
	PlanReadiness func(model.Record) model.Readiness
	TouchWrite    func(string, model.Record) (model.Record, error)
	Now           func() string
	// Document은 검토 대상 아키텍처 문서 본문이다. 실행 전에는 스테이징본, 실행 뒤에는
	// 봉인본이며, 문서가 없으면 빈 문자열이다.
	Document func(string, model.Record) (string, error)
}
