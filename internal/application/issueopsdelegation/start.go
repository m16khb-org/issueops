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
		parent, err := s.Records.Load(req.ParentID)
		if err != nil {
			return err
		}
		if err = s.Authority.Validate(ctx, parent, actor); err != nil {
			return err
		}
		if missing := domain.ChildStartMissingPreconditions(parent, req); len(missing) > 0 {
			return fmt.Errorf("cannot start issueops child: missing %s", strings.Join(missing, ", "))
		}
		repo := s.Identity.CanonicalRepo(parent.Repo)
		if repo == "" {
			return fmt.Errorf("repo is required")
		}
		if err = domain.ValidateBranch(req.Branch); err != nil {
			return err
		}
		id := s.Identity.StableID(repo, req.Branch)
		child, err := s.Records.Load(id)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		now := s.Now().UTC().Format(time.RFC3339Nano)
		if errors.Is(err, fs.ErrNotExist) {
			child = domain.NewCycleRecord(id, repo, req.Branch, now)
		}
		if err = domain.ValidateChildMutation(child); err != nil {
			return err
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
			return err
		}
		result = model.IssueOpsChildStartResult{OK: true, ParentID: parent.ID, Child: child, ParentRef: ref, Guidance: "base_branch=" + parent.Branch + "; create an isolated worktree for " + child.Branch + " and export ISSUEOPS_EXPECTED_WORKTREE after linking it"}
		return nil
	})
	if err != nil {
		return model.IssueOpsChildStartResult{OK: false}, err
	}
	if req.ChildIssueURL != "" {
		if _, err := s.Links.Child(ctx, result.ParentID, req.ChildIssueURL, req.Title, actor); err != nil {
			result.ChildLinkWarning = err.Error()
		}
	}
	return result, nil
}
