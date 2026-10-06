package issueops

import (
	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/adapter/preflight"
	cleanup "issueops/internal/application/issueopscleanup"
	cycle "issueops/internal/application/issueopscycle"
	delegation "issueops/internal/application/issueopsdelegation"
	materialapp "issueops/internal/application/issueopsremote"
	review "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	cycleport "issueops/internal/port/issueopscycle"
	reviewport "issueops/internal/port/issueopsreview"
)

func testChangeReader() implementation.Reader {
	return implementation.Reader{GitCmd: preflight.GitCmd, GitCmdRaw: preflight.GitCmdRaw}
}
func testReadinessGit() ReadinessGit      { return ReadinessGit{Run: GitCmd, Output: GitOut} }
func testCycleReadiness() cycle.Readiness { return testCycleReadinessWithChanges(testChangeReader()) }
func testCycleReadinessWithChanges(changes implementation.Reader) cycle.Readiness {
	observer := review.LocalChangeObserver{Source: reviewport.LocalChangeSource{BaseRef: changes.DiffBaseRef, Paths: changes.ObservedPathsIn, Fingerprint: implementation.FingerprintSnapshot}}
	cleanupStatus := cleanup.StructuralStatus{Environment: CleanupStatusEnvironment{RunGit: GitCmd, ReadGit: GitOut}}
	return cycle.Readiness{Paths: ReadinessPathObservations(), Git: testReadinessGit(), HasEvidence: changes.HasEvidence,
		ObserveChanges: observer.Observe, ChangedPaths: changes.ChangedPaths,
		Cleanup: func(record model.IssueOpsRecord) model.IssueOpsCleanupStatus {
			return cleanupStatus.ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: false})
		},
		ChildMissing: (delegation.ChildGates{Scan: ScanReadableIssueOps}).PRMissing}
}
func testCyclePhaseService(actor *model.IssueOpsActor) cycle.PhaseService {
	store := NewReviewMutationStore(actor)
	return cycle.PhaseService{WriteMaterials: (materialapp.TrackedMaterials{Files: MaterialFiles{}}).Write, Store: cycleport.PhaseStore{Read: store.Read, WithLock: store.WithLock, ValidateMutation: store.ValidateMutation, Write: store.Write, Now: store.Now}, Readiness: testCycleReadiness(), Transitions: cycleport.PhaseTransitionObservations{Head: testReadinessGit().Head, Fingerprint: testChangeReader().ChangeFingerprint}}
}

func IssueOpsAISlopCleanReadiness(record model.IssueOpsRecord) model.IssueOpsReadiness {
	return testCycleReadiness().AISlopClean(record)
}
func IssueOpsPRReadiness(record model.IssueOpsRecord) model.IssueOpsReadiness {
	return testCycleReadiness().PR(record)
}
func IssueOpsLocalPRReadiness(record model.IssueOpsRecord) model.IssueOpsReadiness {
	ready, _ := testCycleReadiness().ObserveLocalPR(record)
	return ready
}
func IssueOpsStrictPRReadiness(record model.IssueOpsRecord) model.IssueOpsReadiness {
	readiness := testCycleReadiness()
	readiness.ChildMissing = func(string, model.IssueOpsRecord) ([]string, []string) { return nil, nil }
	return readiness.StrictPRWithState("", record)
}
func IssueOpsStrictPRReadinessWithState(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
	return testCycleReadiness().StrictPRWithState(root, record)
}
func IssueOpsPhaseCompletion(record model.IssueOpsRecord, phase model.IssueOpsPhase) model.IssueOpsReadiness {
	return testCycleReadiness().Completion(record, phase)
}

func AdvanceIssueOpsPhase(root, id, to string) (model.IssueOpsRecord, error) {
	record, _, err := testCyclePhaseService(nil).AdvanceReport(root, id, to)
	return record, err
}
func AdvanceIssueOpsPhaseWithActor(root, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	record, _, err := testCyclePhaseService(&actor).AdvanceReport(root, id, to)
	return record, err
}

func ObserveIssueOpsLocalPRReadiness(record model.IssueOpsRecord) (model.IssueOpsReadiness, reviewcontract.LocalChangeObservation) {
	return testCycleReadiness().ObserveLocalPR(record)
}

func IssueOpsImplementationReadiness(record model.IssueOpsRecord) model.IssueOpsReadiness {
	return testCycleReadiness().Implementation(record)
}
