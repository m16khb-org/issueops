package issueopsapp

import (
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/adapter/preflight"
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
	return newCycleReadinessReading(preflight.GitCmd)
}

// newCycleReadinessReading runs the scoped readiness git probes through run,
// so a caller that already observed the repository in the same request can
// share its reads.
func newCycleReadinessReading(run func(string, ...string) (int, string, string)) cycle.Readiness {
	changes := newChangeReader()
	observer := review.LocalChangeObserver{Source: reviewport.LocalChangeSource{BaseRef: changes.DiffBaseRef, Paths: changes.ObservedPathsIn, Fingerprint: implementation.FingerprintSnapshot}}
	return cycle.Readiness{Paths: core.ReadinessPathObservations(), Git: newReadinessGit(), HasEvidence: changes.HasEvidence,
		ObserveChanges: observer.Observe, ChangedPaths: changes.ChangedPaths,
		NewObservationScope: func(s cycle.Readiness) cycle.Readiness {
			s.Git, s.ClearGitObservations = core.NewReadinessGitObservations(run)
			return s
		},
		ChildMissing: (delegation.ChildGates{Scan: core.ScanReadableIssueOps}).PRMissing}
}
func newCyclePhaseService(actor *model.IssueOpsActor) cycle.PhaseService {
	store := core.NewReviewMutationStore(actor)
	return cycle.PhaseService{WriteMaterials: (materialapp.TrackedMaterials{Files: core.MaterialFiles{}}).Write, Store: cycleport.PhaseStore{Read: store.Read, WithLock: store.WithLock, ValidateMutation: store.ValidateMutation, Write: store.Write, Now: store.Now}, Readiness: newCycleReadiness(), Transitions: cycleport.PhaseTransitionObservations{Head: newReadinessGit().Head, Fingerprint: newChangeReader().ChangeFingerprint}}
}
