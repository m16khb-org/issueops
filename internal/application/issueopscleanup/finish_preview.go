package issueopscleanup

import (
	"context"
	"fmt"
	"slices"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type finishEnvironment interface {
	Git(string, ...string) (int, string)
	Directory(string) (bool, error)
	SamePath(string, string) bool
	PathWithin(string, string) bool
}

// FinishWorkspaceObservation lists the processes occupying a worktree and the
// receipts that bind them to the preview.
type FinishWorkspaceObservation struct {
	Occupants []model.CleanupWorkspaceProcess
	Receipts  []model.NativeProcessReceipt
}

// FinishPreviewer builds the read-only finish inventory and preview result
// from git, workspace and artifact observations without mutating anything.
type FinishPreviewer struct {
	Environment     finishEnvironment
	Workspace       func(context.Context, model.Record, string) (FinishWorkspaceObservation, []string)
	ObserveArtifact func(context.Context, string) (domain.ArtifactObservation, error)
}

// Plan observes the worktree, branch and remote refs and returns the finish
// inventory with the domain-built preview. A superseding artifact is verified
// only when the merge was not already observed.
func (s FinishPreviewer) Plan(ctx context.Context, record model.Record, req model.CleanupFinishRequest) (model.CleanupFinishInventory, model.CleanupFinishResult) {
	inventory := domain.CleanupFinishTargets(record)
	var facts domain.CleanupFinishObservation
	if !req.Merged {
		if err := s.verifySupersedingArtifact(ctx, record, req.SupersededBy); err != nil {
			facts.SupersedeError = err.Error()
		} else {
			inventory.SupersededBy = strings.TrimSpace(req.SupersededBy)
		}
	}
	base := domain.PreparedBaseBranch(record)
	observeBase := base != "" && strings.TrimSpace(req.MergedBaseBranch) != "" && inventory.SupersededBy == ""
	var refs []string
	if observeBase {
		refs = append(refs, "refs/heads/"+base)
	}
	if inventory.Branch != "" {
		refs = append(refs, "refs/heads/"+inventory.Branch)
	}
	remote := s.observeRemoteRefs(record.Repo, observeBase, refs)
	if observeBase && remote.readable {
		_, facts.PreparedBasePresent = remote.oids["refs/heads/"+base]
		facts.DefaultBranch = remote.defaultBranch
		facts.BaseObserved = remote.defaultBranch != ""
	}
	s.observeWorktree(ctx, record, req, &inventory, &facts)
	if inventory.Branch != "" {
		s.observeBranch(record.Repo, &inventory, &facts)
		facts.RemoteReadable = remote.readable
		facts.RemoteOID = remote.oids["refs/heads/"+inventory.Branch]
	}
	return inventory, domain.BuildCleanupFinishPreview(record, req, inventory, facts)
}

// observeWorktree는 연결된 worktree의 정체성, 존재, 점유, 청결을 관찰해
// inventory와 facts에 기록한다.
func (s FinishPreviewer) observeWorktree(ctx context.Context, record model.Record, req model.CleanupFinishRequest, inventory *model.CleanupFinishInventory, facts *domain.CleanupFinishObservation) {
	if linked := strings.TrimSpace(record.WorktreePath); linked != "" {
		facts.WorktreeIdentityConflict = !s.Environment.SamePath(inventory.WorktreeRoot, linked)
	}
	if inventory.WorktreeRoot != "" {
		var err error
		inventory.WorktreePresent, err = s.Environment.Directory(inventory.WorktreeRoot)
		facts.WorktreeUnobservable = err != nil
	}
	if inventory.WorktreePresent {
		if cwd := strings.TrimSpace(req.CWD); cwd != "" {
			facts.CWDWithin = s.Environment.PathWithin(cwd, inventory.WorktreeRoot)
		}
		observation, missing := s.Workspace(ctx, record, inventory.WorktreeRoot)
		facts.WorkspaceMissing = missing
		facts.Occupants = observation.Occupants
		inventory.WorkspaceProcesses = observation.Receipts
		code, out := s.Environment.Git(inventory.WorktreeRoot, "status", "--porcelain=v1")
		facts.WorktreeClean = code == 0 && strings.TrimSpace(out) == ""
	}
}

// observeBranch는 로컬 브랜치 OID를 관찰해 inventory와 facts에 기록한다.
func (s FinishPreviewer) observeBranch(repo string, inventory *model.CleanupFinishInventory, facts *domain.CleanupFinishObservation) {
	code, out := s.Environment.Git(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+inventory.Branch)
	oid := strings.TrimSpace(out)
	facts.BranchObservable = code == 1 || (code == 0 && oid != "")
	if code == 0 {
		inventory.BranchOID = oid
	}
}

// finishRemoteRefs is what one `git ls-remote origin` advertised.
type finishRemoteRefs struct {
	readable      bool
	oids          map[string]string // exact ref name -> OID
	defaultBranch string            // origin HEAD target when requested
}

// observeRemoteRefs answers the merged-base and remote-branch questions with
// a single network round trip instead of one `ls-remote` per question. Refs are
// matched by exact name, never by ls-remote's tail-match patterns.
func (s FinishPreviewer) observeRemoteRefs(repo string, withHEAD bool, refs []string) finishRemoteRefs {
	if !withHEAD && len(refs) == 0 {
		return finishRemoteRefs{}
	}
	args := []string{"ls-remote"}
	if withHEAD {
		args = append(args, "--symref")
	}
	args = append(args, "origin")
	if withHEAD {
		args = append(args, "HEAD")
	}
	args = append(args, refs...)
	code, out := s.Environment.Git(repo, args...)
	observed := finishRemoteRefs{readable: code == 0, oids: map[string]string{}}
	if code != 0 {
		return observed
	}
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if rest, found := strings.CutPrefix(line, "ref: refs/heads/"); found {
			if target, ref, ok := strings.Cut(rest, "\t"); ok && strings.TrimSpace(ref) == "HEAD" {
				observed.defaultBranch = strings.TrimSpace(target)
			}
			continue
		}
		if oid, ref, ok := strings.Cut(line, "\t"); ok && slices.Contains(refs, strings.TrimSpace(ref)) {
			observed.oids[strings.TrimSpace(ref)] = strings.TrimSpace(oid)
		}
	}
	return observed
}

func (s FinishPreviewer) verifySupersedingArtifact(ctx context.Context, record model.Record, candidate string) error {
	candidate = strings.TrimSpace(candidate)
	if err := domain.ValidateCleanupSupersedeInput(record, candidate, s.ObserveArtifact != nil); err != nil {
		return err
	}
	replacement, err := s.ObserveArtifact(ctx, candidate)
	if err != nil {
		return fmt.Errorf("superseding artifact %s could not be observed: %w", candidate, err)
	}
	return domain.ValidateSupersedingArtifact(domain.ArtifactObservation{URL: record.RemoteArtifact.URL, Provider: record.RemoteArtifact.Provider}, replacement)
}
