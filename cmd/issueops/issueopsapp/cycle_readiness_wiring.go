package issueopsapp

import (
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/adapter/preflight"
	cleanup "issueops/internal/application/issueopscleanup"
	cycle "issueops/internal/application/issueopscycle"
	delegation "issueops/internal/application/issueopsdelegation"
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
	cleanupStatus := cleanup.StructuralStatus{Environment: core.CleanupStatusEnvironment{RunGit: preflight.GitCmd, ReadGit: preflight.GitOut}}
	return cycle.Readiness{Paths: core.ReadinessPathObservations(), Git: newReadinessGit(), HasEvidence: changes.HasEvidence,
		ObserveChanges: observer.Observe, ChangedPaths: changes.ChangedPaths,
		Cleanup: func(record model.IssueOpsRecord) model.IssueOpsCleanupStatus {
			return cleanupStatus.ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: false})
		},
		ChildMissing: (delegation.ChildGates{Scan: core.ScanReadableIssueOps}).PRMissing}
}
func newCyclePhaseService(actor *model.IssueOpsActor) cycle.PhaseService {
	store := core.NewReviewMutationStore(actor)
	return cycle.PhaseService{Store: cycleport.PhaseStore{Read: store.Read, WithLock: store.WithLock, ValidateMutation: store.ValidateMutation, Write: store.Write, Now: store.Now}, Readiness: newCycleReadiness(), Transitions: cycleport.PhaseTransitionObservations{Head: newReadinessGit().Head, Fingerprint: newChangeReader().ChangeFingerprint}}
}
