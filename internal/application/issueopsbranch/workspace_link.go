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

type workspaceLinkFiles interface {
	PlanPath(worktree, path string) string
	SamePlan(left, right string) bool
	PlanExists(repo, path string) bool
	PlanInside(worktree, path string) bool
	ReadPlan(path string) (string, error)
	WorktreeDirectory(path string) bool
	ObserveWorktree(repo, path string) domain.WorktreeLocation
	WorktreeBranch(path string) string
}

// WorkspaceLinker attaches a plan file or a worktree directory to a cycle
// after checking them against the cycle's branch and plan evidence.
type WorkspaceLinker struct {
	Records   CycleRecords
	Authority cycleapp.MutationAuthority
	Files     workspaceLinkFiles
	Now       func() time.Time
}

// Plan links the plan file at path to the cycle. The plan must exist inside
// the worktree, the architecture review must be done, and the plan must
// contain every required section; relinking the same plan is a no-op.
func (s WorkspaceLinker) Plan(ctx context.Context, id, path string, actor *model.Actor) (model.Record, error) {
	var result model.Record
	err := s.Records.WithinLock(ctx, id, func(spanCtx context.Context) error {
		record, err := s.Records.Load(id)
		if err != nil {
			return err
		}
		if err = s.Authority.Validate(ctx, record, actor); err != nil {
			return err
		}
		path = strings.TrimSpace(path)
		if err = domain.ValidateWorkspaceLinkPath("plan_path", path); err != nil {
			return err
		}
		if err = domain.ValidatePlanLinkReadiness(record, stringlist.UniqueSorted(cycleapp.ArchitectureReviewMissing(record))); err != nil {
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

// Worktree links the worktree directory at path to the cycle after checking
// that it is a real worktree of the repo on the cycle's branch and contains
// any linked plan.
func (s WorkspaceLinker) Worktree(ctx context.Context, id, path string, actor *model.Actor) (model.Record, error) {
	var result model.Record
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
