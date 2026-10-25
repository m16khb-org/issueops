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
	Records           CycleRecords
	Authority         cycleapp.MutationAuthority
	CleanParentPath   func(string) (string, bool)
	ResolveBaseCommit func(string, string) (string, error)
	// IsAncestor(repo, ancestor, descendant) guards a base change after the
	// worktree exists (nil refuses such a change).
	IsAncestor            func(string, string, string) (bool, error)
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
		req, err = s.normalizeRequest(record, req)
		if err != nil {
			return err
		}
		record, err = domain.AdoptPreparedBranch(record, req.IssueURL, req.Branch)
		if err != nil {
			return err
		}
		if err := s.validateUmbrella(record, req); err != nil {
			return err
		}
		req.BaseSHA, err = s.resolveBaseSHA(record.Repo, req.BaseSHA)
		if err != nil {
			return err
		}
		if err := s.guardBaseChange(record, req); err != nil {
			return err
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

// normalizeRequest trims/lowercases request fields, fills the issue URL from the record, and validates them.
func (s Preparer) normalizeRequest(record model.IssueOpsRecord, req model.IssueOpsBranchPrepareRequest) (model.IssueOpsBranchPrepareRequest, error) {
	var err error
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.IssueURL = strings.TrimSpace(req.IssueURL)
	if req.IssueURL == "" {
		req.IssueURL = strings.TrimSpace(record.IssueURL)
	}
	if err := domain.ValidateIssueURL(req.IssueURL); err != nil {
		return req, err
	}
	req.Provider, err = remote.PreparationProvider(req.Provider, req.IssueURL)
	if err != nil {
		return req, err
	}
	req.Branch, req.BaseBranch = strings.TrimSpace(req.Branch), strings.TrimSpace(req.BaseBranch)
	if err := domain.ValidatePreparationNames(req.Branch, req.BaseBranch); err != nil {
		return req, err
	}
	req.ParentWorktree = strings.TrimSpace(req.ParentWorktree)
	if req.ParentWorktree != "" {
		var absolute bool
		req.ParentWorktree, absolute = s.CleanParentPath(req.ParentWorktree)
		if err := domain.ValidatePreparationParentPath(absolute); err != nil {
			return req, err
		}
	}
	if err := remote.ValidatePreparationIssueNumber(req.IssueURL, req.Branch); err != nil {
		return req, err
	}
	return req, nil
}

// validateUmbrella checks the branch names against the umbrella issue of a child issue, when one exists.
func (s Preparer) validateUmbrella(record model.IssueOpsRecord, req model.IssueOpsBranchPrepareRequest) error {
	if s.UmbrellaForChildIssue == nil {
		return nil
	}
	umbrella, found := s.UmbrellaForChildIssue(record.Repo, record.IssueURL)
	if !found {
		return nil
	}
	return domain.ValidatePreparationUmbrella(umbrella, req.Branch, req.BaseBranch)
}

// resolveBaseSHA trims baseSHA and resolves it to a local commit OID; empty input stays empty.
func (s Preparer) resolveBaseSHA(repo, baseSHA string) (string, error) {
	baseSHA = strings.TrimSpace(baseSHA)
	if baseSHA == "" {
		return baseSHA, nil
	}
	if s.ResolveBaseCommit == nil {
		return "", fmt.Errorf("base_sha validation is unavailable")
	}
	resolved, err := s.ResolveBaseCommit(repo, baseSHA)
	if err != nil {
		return "", fmt.Errorf("base_sha %q does not resolve to a local commit: %w", baseSHA, err)
	}
	if strings.TrimSpace(resolved) == "" {
		return "", fmt.Errorf("base_sha %q resolved to an empty commit OID", baseSHA)
	}
	return strings.TrimSpace(resolved), nil
}

// guardBaseChange refuses a new base the worktree branch cannot reach by a
// merge alone. The branch was cut from the old base; when the new base lacks
// commits of the old one (main -> a develop that diverged), sync-base would
// merge the new base in and the PR would carry those foreign commits
// (probed: m.txt from main showed up in the develop diff).
func (s Preparer) guardBaseChange(record model.IssueOpsRecord, req model.IssueOpsBranchPrepareRequest) error {
	prior := record.BranchPrepare
	if prior == nil || strings.TrimSpace(prior.BaseSHA) == "" || req.BaseSHA == "" || req.BaseSHA == prior.BaseSHA {
		return nil
	}
	if record.Execution == nil {
		if !prior.LinkVerified {
			return nil
		}
		// Probed: the provider branch was cut from the old base, execution
		// prepare checks it out, and its identity check (HEAD == base_head)
		// then refuses forever with a worktree left behind.
		return fmt.Errorf("the linked branch %s was created at the old base %s (%s); a new base_sha now would make execution prepare refuse the checked-out branch. Keep %s, run execution prepare, then re-record the new base (it must contain the old one) and merge it with execution sync-base, or start a new cycle from %s",
			req.Branch, prior.BaseBranch, shortOID(prior.BaseSHA), shortOID(prior.BaseSHA), req.BaseBranch)
	}
	if s.IsAncestor == nil {
		return fmt.Errorf("base change after execution prepare needs an ancestry check, which is unavailable")
	}
	contains, err := s.IsAncestor(record.Repo, prior.BaseSHA, req.BaseSHA)
	if err != nil {
		return fmt.Errorf("base change after execution prepare: %w", err)
	}
	if !contains {
		return fmt.Errorf("new base %s (%s) does not contain the old base %s (%s) the worktree branch was cut from; merging it with sync-base would carry the old base's extra commits into the PR. Start a new cycle from %s instead (end this one with exits.abandon_command), or keep %s. If %s was squash-merged into %s (stacked work), keep it as the base, open the PR against it, and retarget the PR afterwards (branch retarget keeps the fork point)",
			req.BaseBranch, shortOID(req.BaseSHA), prior.BaseBranch, shortOID(prior.BaseSHA), req.BaseBranch, prior.BaseBranch, prior.BaseBranch, req.BaseBranch)
	}
	return nil
}

func shortOID(oid string) string {
	if len(oid) > 10 {
		return oid[:10]
	}
	return oid
}
