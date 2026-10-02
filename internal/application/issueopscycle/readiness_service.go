package issueopscycle

import (
	"strings"

	model "issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
	domain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	"issueops/internal/domain/stringlist"
	port "issueops/internal/port/issueopscycle"
)

type Readiness struct {
	NewObservationScope  func(Readiness) Readiness
	ClearGitObservations func()
	Paths                port.ReadinessObservations
	Git                  port.PRGitObservations
	HasEvidence          func(model.IssueOpsRecord) bool
	ObserveChanges       func(model.IssueOpsRecord, string) review.LocalChangeObservation
	ChangedPaths         func(model.IssueOpsRecord) []string
	Cleanup              func(model.IssueOpsRecord) model.IssueOpsCleanupStatus
	ChildMissing         func(string, model.IssueOpsRecord) ([]string, []string)
}

func (s Readiness) scoped() Readiness {
	if s.NewObservationScope != nil {
		s = s.NewObservationScope(s)
		s.NewObservationScope = nil
	}
	return s
}

func (s Readiness) Plan(record model.IssueOpsRecord) model.IssueOpsReadiness {
	s.scoped()
	return ReadinessFromMissing(record, domain.PlanReadinessMissing(record))
}
func (s Readiness) Compatibility(record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	return ReadinessFromMissing(record, CompatibilityReadinessMissing(record, s.Paths))
}
func (s Readiness) Implementation(record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	return ReadinessFromMissing(record, ImplementationReadinessMissing(record, true, s.Paths))
}
func (s Readiness) AISlopClean(record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	ready := ReadinessFromMissing(record, ImplementationReadinessMissing(record, false, s.Paths))
	missing := append([]string{}, ready.Missing...)
	if key := reviewdomain.ImplementationEvidenceMissing(s.HasEvidence(record)); key != "" {
		missing = append(missing, key)
	}
	ready.Missing = stringlist.UniqueSorted(missing)
	ready.Ready = len(ready.Missing) == 0
	return ready
}
func (s Readiness) PR(record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	missing := stringlist.UniqueSorted(LocalPRReadinessMissing(record, s.Paths))
	cleanup := s.Cleanup(record)
	return model.IssueOpsReadiness{OK: true, Ready: len(missing) == 0, Missing: missing, Warnings: domain.PRReadinessWarnings(record), CleanupReady: cleanup.Ready, CleanupMissing: cleanup.Missing, IssueURL: record.IssueURL, PlanPath: record.PlanPath, WorktreePath: record.WorktreePath, Branch: record.Branch}
}
func (s Readiness) Completion(record model.IssueOpsRecord, phase model.IssueOpsPhase) model.IssueOpsReadiness {
	s = s.scoped()
	return PhaseCompletion(record, phase, port.PhaseCompletionReadiness{Compatibility: s.Compatibility, AISlopClean: s.AISlopClean, PR: s.PR, RemoteArtifactMissing: domain.RemoteArtifactMissing})
}
func (s Readiness) LocalPR(record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	ready, _ := s.ObserveLocalPR(record)
	return ready
}
func (s Readiness) ObserveLocalPR(record model.IssueOpsRecord) (model.IssueOpsReadiness, review.LocalChangeObservation) {
	s = s.scoped()
	return s.observePR(record, nil)
}
func (s Readiness) StrictPR(record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	ready, _ := s.observePR(record, s.Git.Fetch)
	return ready
}
func (s Readiness) StrictPRWithState(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
	s = s.scoped()
	return s.StrictPRWithFetch(root, record, s.Git.Fetch)
}
func (s Readiness) StrictPRWithFetch(root string, record model.IssueOpsRecord, fetch func(string) review.UpstreamFetch) model.IssueOpsReadiness {
	s = s.scoped()
	ready, _ := s.observePR(record, fetch)
	missing, warnings := s.ChildMissing(root, record)
	if len(missing) == 0 && len(warnings) == 0 {
		return ready
	}
	return ApplyChildPRGate(ready, missing, warnings)
}

// PrefetchUpstream runs outside the lifecycle lock. Its result is valid only for the observed root.
func (s Readiness) PrefetchUpstream(record model.IssueOpsRecord) func(string) review.UpstreamFetch {
	s = s.scoped()
	root := s.Git.Root(record)
	fetched := review.UpstreamFetch{Root: root, Failed: true, Stderr: "upstream was not fetched before this readiness check; retry the command"}
	if root != "" && s.Git.Upstream(root) != "" {
		fetched = s.Git.Fetch(root)
	}
	return func(root string) review.UpstreamFetch {
		if root != fetched.Root {
			return review.UpstreamFetch{Root: root, Failed: true, Stderr: "IssueOps worktree changed after the upstream fetch; retry the command"}
		}
		return fetched
	}
}
func (s Readiness) observePR(record model.IssueOpsRecord, fetch func(string) review.UpstreamFetch) (model.IssueOpsReadiness, review.LocalChangeObservation) {
	syncUpstream := fetch != nil
	ready := s.PR(record)
	ready.Strict = syncUpstream
	head, fingerprint := "", ""
	changes := review.LocalChangeObservation{}
	root := s.Git.Root(record)
	facts := domain.PRGitFacts{RootAvailable: root != "", SyncUpstream: syncUpstream}
	if facts.RootAvailable {
		facts.GitWorktree = s.Git.IsWorktree(root)
	}
	if facts.GitWorktree {
		head = s.Git.Head(record)
		changes = s.ObserveChanges(record, root)
		fingerprint = changes.Fingerprint
		facts.FingerprintVerified = changes.Verified
		facts.CurrentBranch = s.Git.Branch(root)
		facts.WorktreeClean = s.Git.Clean(root)
		if base := domain.PreparedBaseRef(record); base != "" {
			ref := "origin/" + base
			if s.Git.BaseAdvanced(root, ref) {
				facts.BaseRemoteRef, facts.BaseAdvanced = ref, true
			}
		}
		facts.Upstream = s.Git.Upstream(root)
		if facts.Upstream != "" && syncUpstream {
			if s.ClearGitObservations != nil {
				s.ClearGitObservations()
			}
			fetched := fetch(root)
			if s.ClearGitObservations != nil {
				s.ClearGitObservations()
			}
			facts.FetchFailed, facts.FetchStderr = fetched.Failed, fetched.Stderr
			facts.UpstreamCounts = s.Git.Counts(root)
		}
	}
	// Strict schema inspection follows fetch; local classification reuses the verified snapshot.
	schemaMissing := ObservedSchemaEvidenceMissing(record, syncUpstream, changes.Paths, fingerprint, s.ChangedPaths)
	artifacts := ObservedPRFacts{CurrentFingerprint: fingerprint, SchemaMissing: schemaMissing}
	if path := strings.TrimSpace(record.PlanPath); path != "" {
		artifacts.PlanExists = s.Paths.PlanPathExists(root, path)
	}
	artifacts.PlanInWorktree = s.Paths.PlanInLinkedWorktree(record)
	if path := strings.TrimSpace(record.WorktreePath); path != "" {
		artifacts.WorktreeValid = s.Paths.WorktreePathValid(path)
	}
	return ComposeObservedPRReadiness(record, ready, ObservedPRReadinessFacts{Git: facts, Artifact: artifacts, CurrentHead: head, CurrentFingerprint: fingerprint}), changes
}
