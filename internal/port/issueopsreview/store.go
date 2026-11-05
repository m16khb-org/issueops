// Package issueopsreview defines the function-valued store ports that the issueops review use cases
// (devil's advocate, compatibility, regress, project docs, AI slop clean, architecture) use to read and
// write cycle records. Adapters supply the implementations; review policy stays in the application and domain layers.
package issueopsreview

import model "issueops/internal/contract/issueops"

// DevilsAdvocateStore is the record access the devil's-advocate review needs: read a record, write it with its
// timestamp touched, and compute the plan digest the review is bound to.
type DevilsAdvocateStore struct {
	Read       func(string, string) (model.Record, error)
	TouchWrite func(string, model.Record) (model.Record, error)
	PlanDigest func(string, model.Record) (string, error)
}

// CompatibilityStore is the record access the compatibility review needs, including readiness and phase ordering lookups.
type CompatibilityStore struct {
	Read       func(string, string) (model.Record, error)
	TouchWrite func(string, model.Record) (model.Record, error)
	Ready      func(model.Record) model.Readiness
	PhaseRank  func(model.Phase) int
}

// RegressStore is the record access the regress use case needs: reading a record, listing its active children,
// writing with the timestamp touched, and the clock.
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

// ProjectDocsReviewStore extends EvidenceReviewStore with the repository observations the project-docs review needs:
// changed paths, the project root, and file and project-doc presence.
type ProjectDocsReviewStore struct {
	EvidenceReviewStore
	ChangedPaths func(model.Record) []string
	Root         func(model.Record) string
	RelativePath func(string, string) string
	FileExists   func(string, string) bool
	// ProjectDocsPresent reports whether the root has any project doc.
	ProjectDocsPresent func(string) bool
}

// ReviewMutationStore is the locked read-validate-write access used by reviews that mutate a record.
type ReviewMutationStore struct {
	WithLock         func(string, string, func() error) error
	Read             func(string, string) (model.Record, error)
	ValidateMutation func(model.Record) error
	Write            func(string, model.Record) (model.Record, error)
	Now              func() string
}

// AISlopCleanStore extends ReviewMutationStore with Refresh, which re-seals the ai-slop-clean state of the record after evidence is recorded.
type AISlopCleanStore struct {
	ReviewMutationStore
	Refresh func(string, model.Record) (model.Record, error)
}

// ArchitectureReviewStore is the record access the architecture review needs, including plan readiness and the document under review.
type ArchitectureReviewStore struct {
	Read          func(string, string) (model.Record, error)
	PlanReadiness func(model.Record) model.Readiness
	TouchWrite    func(string, model.Record) (model.Record, error)
	Now           func() string
	// Document은 검토 대상 아키텍처 문서 본문이다. 실행 전에는 스테이징본, 실행 뒤에는
	// 봉인본이며, 문서가 없으면 빈 문자열이다.
	Document func(string, model.Record) (string, error)
}
