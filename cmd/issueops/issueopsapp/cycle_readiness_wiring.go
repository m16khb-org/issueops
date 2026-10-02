package issueopsapp

import (
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/adapter/preflight"
	cleanup "issueops/internal/application/issueopscleanup"
	cycle "issueops/internal/application/issueopscycle"
	delegation "issueops/internal/application/issueopsdelegation"
	materialapp "issueops/internal/application/issueopsremote"
	review "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
	reviewport "issueops/internal/port/issueopsreview"
)

func newChangeReader() implementation.Reader {
	return implementation.Reader{GitCmd: preflight.GitCmd, GitCmdRaw: preflight.GitCmdRaw}
}
func newReadinessGit() core.ReadinessGit {
	return core.ReadinessGit{Run: preflight.GitCmd, Output: preflight.GitOut}
}
func newCycleReadiness() cycle.Readiness {
	changes := newChangeReader()
	observer := review.LocalChangeObserver{Source: reviewport.LocalChangeSource{BaseRef: changes.DiffBaseRef, Paths: changes.ObservedPathsIn, Fingerprint: implementation.FingerprintSnapshot}}
	return cycle.Readiness{Paths: core.ReadinessPathObservations(), Git: newReadinessGit(), HasEvidence: changes.HasEvidence,
		ObserveChanges: observer.Observe, ChangedPaths: changes.ChangedPaths,
		NewObservationScope: func(s cycle.Readiness) cycle.Readiness {
			git, environment, reset := core.NewReadinessGitObservations(preflight.GitCmd)
			s.Git, s.ClearGitObservations = git, reset
			s.Cleanup = func(record model.IssueOpsRecord) model.IssueOpsCleanupStatus {
				return (cleanup.StructuralStatus{Environment: environment}).ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: false})
			}
			return s
		},
		ChildMissing: (delegation.ChildGates{Scan: core.ScanReadableIssueOps}).PRMissing}
}
func newCyclePhaseService(actor *model.IssueOpsActor) cycle.PhaseService {
	store := core.NewReviewMutationStore(actor)
	return cycle.PhaseService{WriteMaterials: (materialapp.TrackedMaterials{Files: core.MaterialFiles{}}).Write, Store: cycleport.PhaseStore{Read: store.Read, WithLock: store.WithLock, ValidateMutation: store.ValidateMutation, Write: store.Write, Now: store.Now}, Readiness: newCycleReadiness(), Transitions: cycleport.PhaseTransitionObservations{Head: newReadinessGit().Head, Fingerprint: newChangeReader().ChangeFingerprint}}
}
