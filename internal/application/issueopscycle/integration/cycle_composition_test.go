package cycleintegration

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

func testChangeReader() implementation.Reader {
	return implementation.Reader{GitCmd: preflight.GitCmd, GitCmdRaw: preflight.GitCmdRaw}
}
func testReadinessGit() core.ReadinessGit {
	return core.ReadinessGit{Run: preflight.GitCmd, Output: preflight.GitOut}
}
func testCycleReadiness() cycle.Readiness {
	changes := testChangeReader()
	observer := review.LocalChangeObserver{Source: reviewport.LocalChangeSource{BaseRef: changes.DiffBaseRef, Paths: changes.ObservedPathsIn, Fingerprint: implementation.FingerprintSnapshot}}
	cleanupStatus := cleanup.StructuralStatus{Environment: core.CleanupStatusEnvironment{RunGit: preflight.GitCmd, ReadGit: preflight.GitOut}}
	return cycle.Readiness{Paths: core.ReadinessPathObservations(), Git: testReadinessGit(), HasEvidence: changes.HasEvidence,
		ObserveChanges: observer.Observe, ChangedPaths: changes.ChangedPaths,
		Cleanup: func(record model.IssueOpsRecord) model.IssueOpsCleanupStatus {
			return cleanupStatus.ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: false})
		},
		ChildMissing: (delegation.ChildGates{Scan: core.ScanReadableIssueOps}).PRMissing}
}
func testCyclePhaseService(actor *model.IssueOpsActor) cycle.PhaseService {
	store := core.NewReviewMutationStore(actor)
	return cycle.PhaseService{Store: cycleport.PhaseStore{Read: store.Read, WithLock: store.WithLock, ValidateMutation: store.ValidateMutation, Write: store.Write, Now: store.Now}, Readiness: testCycleReadiness(), Transitions: cycleport.PhaseTransitionObservations{Head: testReadinessGit().Head, Fingerprint: testChangeReader().ChangeFingerprint}}
}

func advancePhaseForTest(root, id, to string) (model.IssueOpsRecord, error) {
	record, _, err := testCyclePhaseService(nil).AdvanceReport(root, id, to)
	return record, err
}
