package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
)

type PRGitObservations interface {
	Root(model.IssueOpsRecord) string
	Head(model.IssueOpsRecord) string
	IsWorktree(string) bool
	Branch(string) string
	Clean(string) bool
	BaseAdvanced(root, ref string) bool
	Upstream(string) string
	Fetch(string) review.UpstreamFetch
	Counts(string) string
}

type PhaseStore struct {
	Read             func(string, string) (model.IssueOpsRecord, error)
	WithLock         func(string, string, func() error) error
	ValidateMutation func(model.IssueOpsRecord) error
	Write            func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Now              func() string
}
