package issueopsdelegation

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

type childRecords interface {
	WithinLock(context.Context, string, func(context.Context) error) error
	Load(string) (model.Record, error)
	SavePair(context.Context, model.Record, model.Record) (model.Record, model.Record, error)
}
type childIdentity interface {
	CanonicalRepo(string) string
	StableID(string, string) string
}

// ChildStarter creates or updates a delegated child cycle record under a
// parent and links the child issue, all with the parent's mutation authority.
type ChildStarter struct {
	Records   childRecords
	Identity  childIdentity
	Authority cycleapp.MutationAuthority
	Links     branchapp.Linker
	Now       func() time.Time
}

// Start validates the request, then under the parent's lock saves the child and
// parent records together. When the request names a child issue URL it also
// links it to the parent; a link failure other than "already linked" is
// reported as ChildLinkWarning instead of an error.
func (s ChildStarter) Start(ctx context.Context, req model.ChildStartRequest, actor *model.Actor) (model.ChildStartResult, error) {
	req, err := domain.PrepareChildStart(req)
	if err != nil {
		return model.ChildStartResult{OK: false}, err
	}
	var result model.ChildStartResult
	err = s.Records.WithinLock(ctx, req.ParentID, func(spanCtx context.Context) error {
		var err error
		result, err = s.startLocked(ctx, req, actor)
		return err
	})
	if err != nil {
		return model.ChildStartResult{OK: false}, err
	}
	if req.ChildIssueURL != "" {
		// create-child already linked this URL; the link is there, which is the
		// whole request (QA read the "already linked" warning as a problem).
		if _, err := s.Links.Child(ctx, result.ParentID, req.ChildIssueURL, req.Title, actor); err != nil && !errors.Is(err, domain.ErrChildIssueAlreadyLinked) {
			result.ChildLinkWarning = err.Error()
		}
	}
	return result, nil
}

// startLocked는 부모 락 안에서 자식 레코드를 만들거나 갱신하고 부모와 함께 저장한다.
func (s ChildStarter) startLocked(ctx context.Context, req model.ChildStartRequest, actor *model.Actor) (model.ChildStartResult, error) {
	parent, err := s.Records.Load(req.ParentID)
	if err != nil {
		return model.ChildStartResult{}, err
	}
	if err = s.Authority.Validate(ctx, parent, actor); err != nil {
		return model.ChildStartResult{}, err
	}
	if missing := domain.ChildStartMissingPreconditions(parent, req); len(missing) > 0 {
		return model.ChildStartResult{}, fmt.Errorf("cannot start issueops child: missing %s", strings.Join(missing, ", "))
	}
	repo := s.Identity.CanonicalRepo(parent.Repo)
	if repo == "" {
		return model.ChildStartResult{}, fmt.Errorf("repo is required")
	}
	if err = domain.ValidateBranch(req.Branch); err != nil {
		return model.ChildStartResult{}, err
	}
	id := s.Identity.StableID(repo, req.Branch)
	child, err := s.Records.Load(id)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return model.ChildStartResult{}, err
	}
	now := s.Now().UTC().Format(time.RFC3339Nano)
	if errors.Is(err, fs.ErrNotExist) {
		child = domain.NewCycleRecord(id, repo, req.Branch, now)
	}
	if err = domain.ValidateChildMutation(child); err != nil {
		return model.ChildStartResult{}, err
	}
	child = domain.BuildDelegatedProfile(parent, child, req, now)
	child.DevilsAdvocateReview = reviewdomain.InheritedParentReview(parent.ID, now)
	child.UpdatedAt = now
	parent, ref, changed := domain.RegisterChildReference(parent, child, req, now)
	if changed {
		parent.UpdatedAt = now
	}
	parent, child, err = s.Records.SavePair(ctx, parent, child)
	if err != nil {
		return model.ChildStartResult{}, err
	}
	return model.ChildStartResult{OK: true, ParentID: parent.ID, Child: child, ParentRef: ref, Guidance: childStartGuidance(parent, child)}, nil
}

// childStartGuidance tells the parent how the child session begins. The child
// cycle owns its branch and worktree (its own branch prepare and execution
// prepare create them); the parent does not make the worktree for it.
func childStartGuidance(parent, child model.Record) string {
	guidance := "base_branch=" + parent.Branch + "; start the child session in the source checkout: its own `branch prepare --id " + child.ID +
		" --base-branch " + parent.Branch
	if parent.Execution != nil && parent.Execution.Workspace.Root != "" {
		guidance += " --parent-worktree " + parent.Execution.Workspace.Root
	}
	guidance += "` and `execution prepare` create the child branch and worktree"
	// QA (GitLab, no child issue): every issue-less child wrote the parent's
	// .issueops/issues/<n>/ files, so the second child PR conflicted with the
	// first after it merged into the parent branch.
	if child.Delegation != nil && strings.TrimSpace(child.Delegation.ChildIssueURL) == "" {
		guidance += "; this child has no issue of its own, so it writes the parent issue's .issueops/issues/ files and its PR conflicts with sibling children there: create a child issue (issueops remote create-child) and pass --child-issue-url to keep them apart"
	}
	return guidance
}
