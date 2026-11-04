package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
)

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

type PhaseStore struct {
	Read             func(string, string) (model.Record, error)
	WithLock         func(string, string, func() error) error
	ValidateMutation func(model.Record) error
	Write            func(string, model.Record) (model.Record, error)
	Now              func() string
}
