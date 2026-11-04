package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
)

// PRGitObservations is the git view the pr readiness checks use: the worktree root, head, branch, cleanliness,
// upstream, fetch result, and the ancestry and base-branch relations of commits.
type PRGitObservations interface {
	Root(model.Record) string
	Head(model.Record) string
	IsWorktree(string) bool
	Branch(string) string
	Clean(string) bool
	// BaseState reports whether ref resolves and, if so, whether it is not an ancestor of HEAD.
	BaseState(root, ref string) (present, advanced bool)
	// IsAncestor reports whether commit is an ancestor of ref.
	IsAncestor(root, commit, ref string) bool
	// ContentAbsorbed reports whether merging commit into ref changes nothing.
	ContentAbsorbed(root, ref, commit string) bool
	Upstream(string) string
	Fetch(string) review.UpstreamFetch
	Counts(string) string
}

// PhaseStore is the record store phase changes use: reading, locked read-modify-write, mutation validation,
// writing, and the clock.
type PhaseStore struct {
	Read             func(string, string) (model.Record, error)
	WithLock         func(string, string, func() error) error
	ValidateMutation func(model.Record) error
	Write            func(string, model.Record) (model.Record, error)
	Now              func() string
}
