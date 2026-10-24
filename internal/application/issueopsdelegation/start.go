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

type ChildRecords interface {
	WithinLock(context.Context, string, func(context.Context) error) error
	Load(string) (model.IssueOpsRecord, error)
	SavePair(context.Context, model.IssueOpsRecord, model.IssueOpsRecord) (model.IssueOpsRecord, model.IssueOpsRecord, error)
}
type ChildIdentity interface {
	CanonicalRepo(string) string
	StableID(string, string) string
}
type ChildStarter struct {
	Records   ChildRecords
	Identity  ChildIdentity
	Authority cycleapp.MutationAuthority
	Links     branchapp.Linker
	Now       func() time.Time
}

func (s ChildStarter) Start(ctx context.Context, req model.IssueOpsChildStartRequest, actor *model.IssueOpsActor) (model.IssueOpsChildStartResult, error) {
	req, err := domain.PrepareChildStart(req)
	if err != nil {
		return model.IssueOpsChildStartResult{OK: false}, err
	}
	var result model.IssueOpsChildStartResult
	err = s.Records.WithinLock(ctx, req.ParentID, func(spanCtx context.Context) error {
		var err error
		result, err = s.startLocked(ctx, req, actor)
		return err
	})
	if err != nil {
		return model.IssueOpsChildStartResult{OK: false}, err
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
func (s ChildStarter) startLocked(ctx context.Context, req model.IssueOpsChildStartRequest, actor *model.IssueOpsActor) (model.IssueOpsChildStartResult, error) {
	parent, err := s.Records.Load(req.ParentID)
	if err != nil {
		return model.IssueOpsChildStartResult{}, err
	}
	if err = s.Authority.Validate(ctx, parent, actor); err != nil {
		return model.IssueOpsChildStartResult{}, err
	}
	if missing := domain.ChildStartMissingPreconditions(parent, req); len(missing) > 0 {
		return model.IssueOpsChildStartResult{}, fmt.Errorf("cannot start issueops child: missing %s", strings.Join(missing, ", "))
	}
	repo := s.Identity.CanonicalRepo(parent.Repo)
	if repo == "" {
		return model.IssueOpsChildStartResult{}, fmt.Errorf("repo is required")
	}
	if err = domain.ValidateBranch(req.Branch); err != nil {
		return model.IssueOpsChildStartResult{}, err
	}
	id := s.Identity.StableID(repo, req.Branch)
	child, err := s.Records.Load(id)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return model.IssueOpsChildStartResult{}, err
	}
	now := s.Now().UTC().Format(time.RFC3339Nano)
	if errors.Is(err, fs.ErrNotExist) {
		child = domain.NewCycleRecord(id, repo, req.Branch, now)
	}
	if err = domain.ValidateChildMutation(child); err != nil {
		return model.IssueOpsChildStartResult{}, err
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
		return model.IssueOpsChildStartResult{}, err
	}
	return model.IssueOpsChildStartResult{OK: true, ParentID: parent.ID, Child: child, ParentRef: ref, Guidance: childStartGuidance(parent, child)}, nil
}

// childStartGuidance tells the parent how the child session begins. The child
// cycle owns its branch and worktree (its own branch prepare and execution
// prepare create them); the parent does not make the worktree for it.
func childStartGuidance(parent, child model.IssueOpsRecord) string {
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
