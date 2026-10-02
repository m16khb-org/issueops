package issueopsbranch

import (
	"context"
	"strings"
	"time"

	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
)

type WorkspaceLinkFiles interface {
	PlanPath(worktree, path string) string
	SamePlan(left, right string) bool
	PlanExists(repo, path string) bool
	PlanInside(worktree, path string) bool
	ReadPlan(path string) (string, error)
	WorktreeDirectory(path string) bool
	ObserveWorktree(repo, path string) domain.WorktreeLocation
	WorktreeBranch(path string) string
}

type WorkspaceLinker struct {
	Records   CycleRecords
	Authority cycleapp.MutationAuthority
	Files     WorkspaceLinkFiles
	Now       func() time.Time
}

func (s WorkspaceLinker) Plan(ctx context.Context, id, path string, actor *model.IssueOpsActor) (model.IssueOpsRecord, error) {
	var result model.IssueOpsRecord
	err := s.Records.WithinLock(ctx, id, func(spanCtx context.Context) error {
		record, err := s.Records.Load(id)
		if err != nil {
			return err
		}
		if err = s.Authority.ValidatePlanLink(ctx, record, actor); err != nil {
			return err
		}
		path = strings.TrimSpace(path)
		if err = domain.ValidateWorkspaceLinkPath("plan_path", path); err != nil {
			return err
		}
		if err = domain.ValidatePlanLinkReadiness(record, stringlist.UniqueSorted(cycleapp.DesignReviewMissing(record))); err != nil {
			return err
		}
		worktree := strings.TrimSpace(record.WorktreePath)
		path = s.Files.PlanPath(worktree, path)
		if err = domain.ValidatePlanLinkLocation(path, worktree, s.Files.PlanExists(record.Repo, path), s.Files.PlanInside(worktree, path)); err != nil {
			return err
		}
		linked := strings.TrimSpace(record.PlanPath)
		same := linked != "" && s.Files.SamePlan(s.Files.PlanPath(worktree, linked), path)
		unchanged, err := domain.PlanLinkIsUnchanged(linked, same)
		if err != nil {
			return err
		}
		if unchanged {
			result = record
			return nil
		}
		body, readErr := s.Files.ReadPlan(path)
		missing := domain.RequiredPlanSections
		if readErr == nil {
			missing = domain.MissingPlanSections(body)
		}
		if err = domain.ValidateLinkedPlanSections(missing); err != nil {
			return err
		}
		result, err = s.Records.Save(spanCtx, domain.ApplyPlanLink(record, path, s.Now().UTC().Format(time.RFC3339Nano)))
		return err
	})
	return result, err
}

func (s WorkspaceLinker) Worktree(ctx context.Context, id, path string, actor *model.IssueOpsActor) (model.IssueOpsRecord, error) {
	var result model.IssueOpsRecord
	err := s.Records.WithinLock(ctx, id, func(spanCtx context.Context) error {
		record, err := s.Records.Load(id)
		if err != nil {
			return err
		}
		if err = s.Authority.Validate(ctx, record, actor); err != nil {
			return err
		}
		path = strings.TrimSpace(path)
		if err = domain.ValidateWorkspaceLinkPath("worktree_path", path); err != nil {
			return err
		}
		if err = domain.ValidateLinkBranchEvidence(record, "worktree"); err != nil {
			return err
		}
		if err = domain.ValidateWorktreeDirectory(path, s.Files.WorktreeDirectory(path)); err != nil {
			return err
		}
		if err = domain.ValidateWorktreeLocation(s.Files.ObserveWorktree(record.Repo, path)); err != nil {
			return err
		}
		if strings.TrimSpace(record.Branch) != "" {
			if err = domain.ValidateLinkedWorktreeBranch(record.Branch, s.Files.WorktreeBranch(path)); err != nil {
				return err
			}
		}
		if plan := strings.TrimSpace(record.PlanPath); plan != "" {
			if err = domain.ValidateLinkedPlanContainment(path, s.Files.PlanInside(path, plan)); err != nil {
				return err
			}
		}
		result, err = s.Records.Save(spanCtx, domain.ApplyWorktreeLink(record, path, s.Now().UTC().Format(time.RFC3339Nano)))
		return err
	})
	return result, err
}
