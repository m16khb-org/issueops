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

// Readiness 타입은 사이클의 phase별 준비 상태를 관찰 결과(경로, git, 변경 파일)와 도메인 규칙으로 계산한다. 값 복사로 관찰 범위를 나눠 쓴다.
type Readiness struct {
	NewObservationScope  func(Readiness) Readiness
	ClearGitObservations func()
	Paths                port.ReadinessObservations
	Git                  port.PRGitObservations
	HasEvidence          func(model.Record) bool
	ObserveChanges       func(model.Record, string) review.LocalChangeObservation
	ChangedPaths         func(model.Record) []string
	ChildMissing         func(string, model.Record) ([]string, []string)
}

func (s Readiness) scoped() Readiness {
	if s.NewObservationScope != nil {
		s = s.NewObservationScope(s)
		s.NewObservationScope = nil
	}
	return s
}

// Plan 메서드는 계획 phase의 준비 상태를 돌려준다.
func (s Readiness) Plan(record model.Record) model.Readiness {
	s.scoped()
	return readinessFromMissing(record, domain.PlanReadinessMissing(record))
}

// Compatibility 메서드는 호환성 검토 phase의 준비 상태를 돌려준다.
func (s Readiness) Compatibility(record model.Record) model.Readiness {
	scoped := s.scoped()
	return readinessFromMissing(record, compatibilityReadinessMissing(record, scoped.Paths))
}

// Implementation 메서드는 구현 phase의 준비 상태를 돌려준다.
func (s Readiness) Implementation(record model.Record) model.Readiness {
	scoped := s.scoped()
	return readinessFromMissing(record, implementationReadinessMissing(record, true, scoped.Paths))
}

// AISlopClean 메서드는 ai-slop-clean phase의 준비 상태를 돌려준다. 구현 준비 상태에 구현 증거 누락을 더하고 누락 키를 정렬·중복 제거한다.
func (s Readiness) AISlopClean(record model.Record) model.Readiness {
	scoped := s.scoped()
	ready := readinessFromMissing(record, implementationReadinessMissing(record, false, scoped.Paths))
	missing := append([]string{}, ready.Missing...)
	if key := reviewdomain.ImplementationEvidenceMissing(scoped.HasEvidence(record)); key != "" {
		missing = append(missing, key)
	}
	ready.Missing = stringlist.UniqueSorted(missing)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

// PR is the local PR readiness. Post-merge cleanup readiness is not part of
// it: it can never be ready before the merge and costs a network ls-remote;
// `issueops cleanup status` owns it.
func (s Readiness) PR(record model.Record) model.Readiness {
	scoped := s.scoped()
	missing := stringlist.UniqueSorted(localPRReadinessMissing(record, scoped.Paths))
	return model.Readiness{OK: true, Ready: len(missing) == 0, Missing: missing, Warnings: domain.PRReadinessWarnings(record), IssueURL: record.IssueURL, PlanPath: record.PlanPath, WorktreePath: record.WorktreePath, Branch: record.Branch}
}

// Completion 메서드는 phase 완료 시점의 준비 상태를 돌려준다.
func (s Readiness) Completion(record model.Record, phase model.Phase) model.Readiness {
	scoped := s.scoped()
	return phaseCompletion(record, phase, port.PhaseCompletionReadiness{Compatibility: scoped.Compatibility, AISlopClean: scoped.AISlopClean, PR: scoped.PR, RemoteArtifactMissing: domain.RemoteArtifactMissing})
}

// ObserveLocalPR is the fetch-free, network-free PR readiness `next` reads.
func (s Readiness) ObserveLocalPR(record model.Record) (model.Readiness, review.LocalChangeObservation) {
	scoped := s.scoped()
	return scoped.observePR(record, nil)
}

// StrictPRWithState 메서드는 upstream을 fetch한 strict PR 준비 상태를 돌려준다. 자식 사이클 게이트의 누락·경고도 반영한다.
func (s Readiness) StrictPRWithState(root string, record model.Record) model.Readiness {
	scoped := s.scoped()
	return scoped.strictPRWithFetch(root, record, scoped.Git.Fetch)
}
func (s Readiness) strictPRWithFetch(root string, record model.Record, fetch func(string) review.UpstreamFetch) model.Readiness {
	scoped := s.scoped()
	ready, _ := scoped.observePR(record, fetch)
	missing, warnings := scoped.ChildMissing(root, record)
	if len(missing) == 0 && len(warnings) == 0 {
		return ready
	}
	return applyChildPRGate(ready, missing, warnings)
}

// prefetchUpstream runs outside the lifecycle lock. Its result is valid only for the observed root.
func (s Readiness) prefetchUpstream(record model.Record) func(string) review.UpstreamFetch {
	scoped := s.scoped()
	root := scoped.Git.Root(record)
	fetched := review.UpstreamFetch{Root: root, Failed: true, Stderr: "upstream was not fetched before this readiness check; retry the command"}
	if root != "" && scoped.Git.Upstream(root) != "" {
		fetched = scoped.Git.Fetch(root)
	}
	return func(root string) review.UpstreamFetch {
		if root != fetched.Root {
			return review.UpstreamFetch{Root: root, Failed: true, Stderr: "IssueOps worktree changed after the upstream fetch; retry the command"}
		}
		return fetched
	}
}
func (s Readiness) observePR(record model.Record, fetch func(string) review.UpstreamFetch) (model.Readiness, review.LocalChangeObservation) {
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
		// Fetch first so the base is compared with what origin holds now.
		facts.Upstream = s.Git.Upstream(root)
		originFetched := false
		if facts.Upstream != "" && syncUpstream {
			if s.ClearGitObservations != nil {
				s.ClearGitObservations()
			}
			fetched := fetch(root)
			if s.ClearGitObservations != nil {
				s.ClearGitObservations()
			}
			facts.FetchFailed, facts.FetchStderr = fetched.Failed, fetched.Stderr
			originFetched = !fetched.Failed && strings.HasPrefix(facts.Upstream, "origin/")
		}
		s.observeBase(record, root, originFetched, &facts)
		if facts.Upstream != "" && syncUpstream {
			facts.UpstreamCounts = s.Git.Counts(root)
		}
	}
	// Strict schema inspection follows fetch; local classification reuses the verified snapshot.
	schemaMissing := ObservedSchemaEvidenceMissing(record, syncUpstream, changes.Paths, fingerprint, s.ChangedPaths)
	artifacts := observedPRFacts{CurrentFingerprint: fingerprint, SchemaMissing: schemaMissing}
	if path := strings.TrimSpace(record.PlanPath); path != "" {
		artifacts.PlanExists = s.Paths.PlanPathExists(root, path)
	}
	artifacts.PlanInWorktree = s.Paths.PlanInLinkedWorktree(record)
	if path := strings.TrimSpace(record.WorktreePath); path != "" {
		artifacts.WorktreeValid = s.Paths.WorktreePathValid(path)
	}
	return composeObservedPRReadiness(record, ready, observedPRReadinessFacts{Git: facts, Artifact: artifacts, CurrentHead: head, CurrentFingerprint: fingerprint}), changes
}

// observeBase compares origin/<prepared base> with HEAD. A missing ref is a
// fact only after a pruning fetch of origin; otherwise it is skipped.
func (s Readiness) observeBase(record model.Record, root string, originFetched bool, facts *domain.PRGitFacts) {
	base := domain.PreparedBaseRef(record)
	if base == "" {
		return
	}
	ref := "origin/" + base
	present, advanced := s.Git.BaseState(root, ref)
	if !present {
		if originFetched {
			facts.BaseRemoteRef, facts.BaseMissing = ref, true
		}
		return
	}
	if !advanced {
		return
	}
	facts.BaseRemoteRef, facts.BaseAdvanced = ref, true
	if commits := domain.BaseCommitsInWork(record); len(commits) > 0 {
		facts.BaseRewritten = !domain.BaseHoldsWorkBase(commits,
			func(commit string) bool { return s.Git.IsAncestor(root, commit, ref) },
			func(commit string) bool { return s.Git.ContentAbsorbed(root, ref, commit) })
	}
}
