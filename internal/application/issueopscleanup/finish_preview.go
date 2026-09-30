package issueopscleanup

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type FinishEnvironment interface {
	Git(string, ...string) (int, string)
	Directory(string) (bool, error)
	SamePath(string, string) bool
	PathWithin(string, string) bool
}

type FinishWorkspaceObservation struct {
	Occupants    []model.CleanupWorkspaceProcess
	Receipts     []model.NativeProcessReceipt
	Terminals    []string
	RuntimeReady bool
	AppPID       int
}

type FinishPreviewer struct {
	Environment     FinishEnvironment
	Workspace       func(context.Context, model.IssueOpsRecord, string) (FinishWorkspaceObservation, []string)
	ObserveArtifact func(string) (domain.ArtifactObservation, error)
}

func (s FinishPreviewer) Plan(ctx context.Context, record model.IssueOpsRecord, req model.CleanupFinishRequest) (model.CleanupFinishInventory, model.CleanupFinishResult) {
	inventory := domain.CleanupFinishTargets(record)
	var facts domain.CleanupFinishObservation
	if !req.Merged {
		if err := s.verifySupersedingArtifact(record, req.SupersededBy); err != nil {
			facts.SupersedeError = err.Error()
		} else {
			inventory.SupersededBy = strings.TrimSpace(req.SupersededBy)
		}
	}
	if base := domain.PreparedBaseBranch(record); base != "" && strings.TrimSpace(req.MergedBaseBranch) != "" && inventory.SupersededBy == "" {
		facts.DefaultBranch, facts.PreparedBasePresent, facts.BaseObserved = s.observeMergedBaseRefs(record.Repo, base)
	}
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
		inventory.OrcaTerminals = observation.Terminals
		inventory.OrcaAppPID = observation.AppPID
		inventory.OrcaRuntimeReady = observation.RuntimeReady
		code, out := s.Environment.Git(inventory.WorktreeRoot, "status", "--porcelain=v1")
		facts.WorktreeClean = code == 0 && strings.TrimSpace(out) == ""
	}
	if inventory.Branch != "" {
		code, out := s.Environment.Git(record.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+inventory.Branch)
		oid := strings.TrimSpace(out)
		facts.BranchObservable = code == 1 || (code == 0 && oid != "")
		if code == 0 {
			inventory.BranchOID = oid
		}
		code, out = s.Environment.Git(record.Repo, "ls-remote", "--heads", "origin", "refs/heads/"+inventory.Branch)
		facts.RemoteReadable = code == 0
		if fields := strings.Fields(strings.TrimSpace(out)); len(fields) > 0 {
			facts.RemoteOID = fields[0]
		}
	}
	return inventory, domain.BuildCleanupFinishPreview(record, req, inventory, facts)
}

func (s FinishPreviewer) observeMergedBaseRefs(repo, base string) (string, bool, bool) {
	code, out := s.Environment.Git(repo, "ls-remote", "--heads", "origin", "refs/heads/"+base)
	if code != 0 {
		return "", false, false
	}
	present := len(strings.Fields(strings.TrimSpace(out))) > 0
	code, out = s.Environment.Git(repo, "ls-remote", "--symref", "origin", "HEAD")
	if code != 0 {
		return "", present, false
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), "ref: refs/heads/")
		if !found {
			continue
		}
		if name := strings.TrimSpace(strings.SplitN(rest, "\t", 2)[0]); name != "" {
			return name, present, true
		}
	}
	return "", present, false
}

func (s FinishPreviewer) verifySupersedingArtifact(record model.IssueOpsRecord, candidate string) error {
	candidate = strings.TrimSpace(candidate)
	if err := domain.ValidateCleanupSupersedeInput(record, candidate, s.ObserveArtifact != nil); err != nil {
		return err
	}
	replacement, err := s.ObserveArtifact(candidate)
	if err != nil {
		return fmt.Errorf("superseding artifact %s could not be observed: %w", candidate, err)
	}
	return domain.ValidateSupersedingArtifact(domain.ArtifactObservation{URL: record.RemoteArtifact.URL, Provider: record.RemoteArtifact.Provider}, replacement)
}
