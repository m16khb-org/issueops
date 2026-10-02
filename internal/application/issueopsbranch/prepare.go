package issueopsbranch

import (
	"context"
	"fmt"
	"strings"
	"time"

	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
)

type Preparer struct {
	Records               CycleRecords
	Authority             cycleapp.MutationAuthority
	CleanParentPath       func(string) (string, bool)
	ResolveBaseCommit     func(string, string) (string, error)
	UmbrellaForChildIssue func(string, string) (model.IssueOpsRecord, bool)
	ObserveCodeProjectKey func(string, string) (string, error)
	Steps                 func(string, string, string, string, string) []model.IssueOpsBranchPrepareStep
	Now                   func() time.Time
}

func (s Preparer) Prepare(ctx context.Context, id string, req model.IssueOpsBranchPrepareRequest, actor *model.IssueOpsActor) (model.IssueOpsRecord, error) {
	var result model.IssueOpsRecord
	err := s.Records.WithinLock(ctx, id, func(spanCtx context.Context) error {
		record, err := s.Records.Load(id)
		if err != nil {
			return err
		}
		if err := s.Authority.Validate(ctx, record, actor); err != nil {
			return err
		}
		req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
		req.IssueURL = strings.TrimSpace(req.IssueURL)
		if req.IssueURL == "" {
			req.IssueURL = strings.TrimSpace(record.IssueURL)
		}
		if err := domain.ValidateIssueURL(req.IssueURL); err != nil {
			return err
		}
		req.Provider, err = remote.PreparationProvider(req.Provider, req.IssueURL)
		if err != nil {
			return err
		}
		req.Branch, req.BaseBranch = strings.TrimSpace(req.Branch), strings.TrimSpace(req.BaseBranch)
		if err := domain.ValidatePreparationNames(req.Branch, req.BaseBranch); err != nil {
			return err
		}
		req.ParentWorktree = strings.TrimSpace(req.ParentWorktree)
		if req.ParentWorktree != "" {
			var absolute bool
			req.ParentWorktree, absolute = s.CleanParentPath(req.ParentWorktree)
			if err := domain.ValidatePreparationParentPath(absolute); err != nil {
				return err
			}
		}
		if err := remote.ValidatePreparationIssueNumber(req.IssueURL, req.Branch); err != nil {
			return err
		}
		record, err = domain.AdoptPreparedBranch(record, req.IssueURL, req.Branch)
		if err != nil {
			return err
		}
		if s.UmbrellaForChildIssue != nil {
			if umbrella, found := s.UmbrellaForChildIssue(record.Repo, record.IssueURL); found {
				if err := domain.ValidatePreparationUmbrella(umbrella, req.Branch, req.BaseBranch); err != nil {
					return err
				}
			}
		}
		req.BaseSHA = strings.TrimSpace(req.BaseSHA)
		if req.BaseSHA != "" {
			if s.ResolveBaseCommit == nil {
				return fmt.Errorf("base_sha validation is unavailable")
			}
			resolved, err := s.ResolveBaseCommit(record.Repo, req.BaseSHA)
			if err != nil {
				return fmt.Errorf("base_sha %q does not resolve to a local commit: %w", req.BaseSHA, err)
			}
			if strings.TrimSpace(resolved) == "" {
				return fmt.Errorf("base_sha %q resolved to an empty commit OID", req.BaseSHA)
			}
			req.BaseSHA = strings.TrimSpace(resolved)
		}
		var observed string
		var observationErr error
		if strings.TrimSpace(req.CodeProjectKey) == "" && s.ObserveCodeProjectKey != nil {
			observed, observationErr = s.ObserveCodeProjectKey(record.Repo, req.Provider)
		}
		codeProject, err := remote.PreparationCodeProject(req.Provider, req.IssueURL, req.CodeProjectKey, observed, observationErr)
		if err != nil {
			return err
		}
		req.RemoteBranchURL = strings.TrimSpace(req.RemoteBranchURL)
		steps := s.Steps(req.Provider, req.IssueURL, req.Branch, req.BaseBranch, req.BaseSHA)
		result, err = s.Records.Save(spanCtx, domain.ApplyBranchPreparation(record, req, codeProject, steps, s.Now().UTC().Format(time.RFC3339Nano), s.Now().UTC().Format(time.RFC3339Nano)))
		return err
	})
	return result, err
}
