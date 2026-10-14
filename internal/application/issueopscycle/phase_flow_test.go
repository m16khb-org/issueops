package issueopscycle

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	gatescontract "issueops/internal/contract/gates"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

// phaseFlow is a hermetic PhaseStore around one in-memory record.
type phaseFlow struct {
	record     model.IssueOpsRecord
	events     []string
	writes     []model.IssueOpsRecord
	mutateErr  error
	readErr    error
	writeErr   error
	prefetches int
}

func (p *phaseFlow) store() cycleport.PhaseStore {
	return cycleport.PhaseStore{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			p.events = append(p.events, "read")
			return p.record, p.readErr
		},
		WithLock: func(_, _ string, fn func() error) error {
			p.events = append(p.events, "lock")
			return fn()
		},
		ValidateMutation: func(model.IssueOpsRecord) error {
			p.events = append(p.events, "authorize")
			return p.mutateErr
		},
		Write: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			p.events = append(p.events, "write")
			if p.writeErr != nil {
				return model.IssueOpsRecord{}, p.writeErr
			}
			p.writes = append(p.writes, r)
			p.record = r
			return r, nil
		},
		Now: func() string { return "2026-10-12T00:00:00Z" },
	}
}

func readyReadiness() model.IssueOpsReadiness { return model.IssueOpsReadiness{OK: true, Ready: true} }

// flowService wires every readiness to "ready" except where a test overrides.
func flowService(p *phaseFlow, ready Readiness) PhaseService {
	return PhaseService{
		Store:     p.store(),
		Readiness: ready,
		Transitions: cycleport.PhaseTransitionObservations{
			Head:        func(model.IssueOpsRecord) string { return "head-sha" },
			Fingerprint: func(model.IssueOpsRecord) string { return "fp-sha" },
		},
	}
}

func readyPaths() cycleport.ReadinessObservations {
	return cycleport.ReadinessObservations{
		WorktreePathValid:    func(string) bool { return true },
		PlanPathExists:       func(string, string) bool { return true },
		PlanInLinkedWorktree: func(model.IssueOpsRecord) bool { return true },
		WorkspaceMatches:     func(string, string) bool { return true },
		LinkedPlanDigest:     func(model.IssueOpsRecord) (string, error) { return "", nil },
	}
}

func TestProblemAndGrillReadinessReportMissingIntentAndGrill(t *testing.T) {
	problem := ProblemReadiness(model.IssueOpsRecord{ID: "io-1", IssueURL: "https://example/issues/1"})
	if problem.Ready || !problem.OK || !reflect.DeepEqual(problem.Missing, []string{"intent_contract"}) || problem.IssueURL != "https://example/issues/1" {
		t.Fatalf("problem readiness=%+v", problem)
	}
	grill := GrillReadiness(model.IssueOpsRecord{})
	if grill.Ready || len(grill.Missing) == 0 {
		t.Fatalf("grill readiness of an empty record must not be ready: %+v", grill)
	}
}

func TestAdvanceReportRejectsUnknownPhaseWithoutWriting(t *testing.T) {
	p := &phaseFlow{record: model.IssueOpsRecord{ID: "io-1", Phase: model.IssueOpsPhaseProblem}}
	_, _, err := flowService(p, Readiness{}).AdvanceReport("state", "io-1", "launch")
	if err == nil || !strings.Contains(err.Error(), `unknown issueops phase "launch"`) || len(p.writes) != 0 {
		t.Fatalf("err=%v writes=%d", err, len(p.writes))
	}
}

func TestAdvanceReportSurfacesReadAndAuthorityFailuresBeforeAnyWrite(t *testing.T) {
	readBoom := errors.New("record unreadable")
	p := &phaseFlow{record: model.IssueOpsRecord{ID: "io-1"}, readErr: readBoom}
	if _, _, err := flowService(p, Readiness{}).AdvanceReport("state", "io-1", "grill"); !errors.Is(err, readBoom) || len(p.writes) != 0 {
		t.Fatalf("read failure err=%v writes=%d", err, len(p.writes))
	}
	denied := errors.New("not the holder")
	p = &phaseFlow{record: model.IssueOpsRecord{ID: "io-1"}, mutateErr: denied}
	if _, _, err := flowService(p, Readiness{}).AdvanceReport("state", "io-1", "grill"); !errors.Is(err, denied) || len(p.writes) != 0 {
		t.Fatalf("authority failure err=%v writes=%d", err, len(p.writes))
	}
}

func TestAdvanceReportPrefetchReadFailureStopsBeforeLock(t *testing.T) {
	readBoom := errors.New("record unreadable")
	p := &phaseFlow{record: model.IssueOpsRecord{ID: "io-1"}, readErr: readBoom}
	_, _, err := flowService(p, Readiness{}).AdvanceReport("state", "io-1", "pr")
	if !errors.Is(err, readBoom) {
		t.Fatalf("err=%v events=%v", err, p.events)
	}
	for _, e := range p.events {
		if e == "lock" {
			t.Fatalf("lock taken after prefetch read failure: %v", p.events)
		}
	}
}

func TestAdvanceReportToCurrentPhaseIsIdempotent(t *testing.T) {
	p := &phaseFlow{record: model.IssueOpsRecord{ID: "io-1", Phase: model.IssueOpsPhasePlan}}
	got, materials, err := flowService(p, Readiness{}).AdvanceReport("state", "io-1", " plan ")
	if err != nil || got.Phase != model.IssueOpsPhasePlan || len(p.writes) != 0 || !reflect.DeepEqual(materials, model.IssueOpsTrackedMaterials{}) {
		t.Fatalf("same-phase advance must be a no-op: got=%+v err=%v writes=%d", got, err, len(p.writes))
	}
}

func TestAdvanceReportRefusesAISlopCleanRefreshWhenNotReady(t *testing.T) {
	// A record already in pr with a prior clean stamp takes the refresh path (refused here as not ready), not a backward move.
	record := model.IssueOpsRecord{ID: "io-1", Phase: model.IssueOpsPhasePR, AISlopCleanAt: "old"}
	p := &phaseFlow{record: record}
	service := flowService(p, Readiness{Paths: readyPaths(), HasEvidence: func(model.IssueOpsRecord) bool { return false }})
	got, _, err := service.AdvanceReport("state", "io-1", "ai-slop-clean")
	if err == nil || !strings.Contains(err.Error(), "cannot refresh ai-slop-clean phase") || got.OK || len(p.writes) != 0 {
		t.Fatalf("got=%+v err=%v writes=%d", got, err, len(p.writes))
	}
}

func TestAdvanceReportRefusesBackwardAndUnreadyEntriesWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name string
		from model.IssueOpsPhase
		to   string
		want string
	}{
		{"backward", model.IssueOpsPhasePlan, "problem", "cannot move issueops phase backward from plan to problem"},
		{"leave done", model.IssueOpsPhaseDone, "feedback", "cannot leave done phase"},
		{"grill needs intent", model.IssueOpsPhaseProblem, "grill", "cannot enter grill phase: missing intent_contract"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := model.IssueOpsRecord{ID: "io-1", Phase: tc.from}
			p := &phaseFlow{record: record}
			got, _, err := flowService(p, Readiness{}).AdvanceReport("state", "io-1", tc.to)
			if err == nil || !strings.Contains(err.Error(), tc.want) || got.OK || len(p.writes) != 0 {
				t.Fatalf("got=%+v err=%v writes=%d", got, err, len(p.writes))
			}
		})
	}
}

func TestAdvanceReportPersistsForwardTransitionWithUpdatedAtAndLedger(t *testing.T) {
	record := model.IssueOpsRecord{
		ID: "io-1", Phase: model.IssueOpsPhaseProblem,
		Intent: &model.IssueOpsIntentContract{RawRequest: "r", InterpretedIntent: "i", SuccessCriteria: []string{"c"}, IntentClass: "trivial"},
	}
	p := &phaseFlow{record: record}
	got, _, err := flowService(p, Readiness{}).AdvanceReport("state", "io-1", "grill")
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != model.IssueOpsPhaseGrill || got.UpdatedAt != "2026-10-12T00:00:00Z" || len(p.writes) != 1 || p.writes[0].Phase != model.IssueOpsPhaseGrill {
		t.Fatalf("got=%+v writes=%d", got, len(p.writes))
	}
	if _, ok := got.PhaseLedger[model.IssueOpsPhaseGrill]; !ok {
		t.Fatalf("ledger lacks the entered phase: %+v", got.PhaseLedger)
	}
}

func TestAdvanceReportWriteFailureIsReturned(t *testing.T) {
	boom := errors.New("disk full")
	record := model.IssueOpsRecord{
		ID: "io-1", Phase: model.IssueOpsPhaseProblem,
		Intent: &model.IssueOpsIntentContract{RawRequest: "r", InterpretedIntent: "i", SuccessCriteria: []string{"c"}, IntentClass: "trivial"},
	}
	p := &phaseFlow{record: record, writeErr: boom}
	if _, _, err := flowService(p, Readiness{}).AdvanceReport("state", "io-1", "grill"); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

func TestAdvanceReportWritesTrackedMaterialsOnlyWhenEnteringImplementOrAISlopClean(t *testing.T) {
	// Entering grill must never trigger material writes, even though the hook is wired.
	calls := 0
	record := model.IssueOpsRecord{
		ID: "io-1", Phase: model.IssueOpsPhaseProblem,
		Intent: &model.IssueOpsIntentContract{RawRequest: "r", InterpretedIntent: "i", SuccessCriteria: []string{"c"}, IntentClass: "trivial"},
	}
	p := &phaseFlow{record: record}
	service := flowService(p, Readiness{})
	service.WriteMaterials = func(model.IssueOpsRecord) model.IssueOpsTrackedMaterials {
		calls++
		return model.IssueOpsTrackedMaterials{}
	}
	if _, _, err := service.AdvanceReport("state", "io-1", "grill"); err != nil || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestAdvanceReportToPRWithoutPrefetchedUpstreamDoesNotBypassFetchGate(t *testing.T) {
	// A record already in pr skips prefetch; a direct locked advance from feedback with a
	// service whose prefetch was skipped must report upstream_fetch rather than pass.
	locked := false
	events := []string{}
	ready, _ := readinessForEffects(t, &locked, &events)
	record := model.IssueOpsRecord{ID: "io-1", Repo: "/repo", Branch: "feature", Phase: model.IssueOpsPhaseFeedback, Execution: &model.Execution{Mode: model.ExecutionModeDirect}}
	service := flowService(&phaseFlow{record: record}, ready)
	got := service.entryReadiness("state", nil).StrictPR(record)
	if got.Ready || !strings.Contains(strings.Join(got.Missing, ","), "upstream_fetch") {
		t.Fatalf("missing prefetch must fail closed: %+v events=%v", got, events)
	}
	for _, e := range events {
		if e == "fetch" {
			t.Fatalf("fallback performed a network fetch: %v", events)
		}
	}
}

func TestPrefetchForPhaseFetchesOnlyForPRTargetsAfterAuthority(t *testing.T) {
	p := &phaseFlow{record: model.IssueOpsRecord{ID: "io-1", Phase: model.IssueOpsPhasePR}}
	service := flowService(p, Readiness{})
	if fetch, err := service.prefetchForPhase("state", "io-1", "feedback"); err != nil || fetch != nil || len(p.events) != 0 {
		t.Fatalf("non-pr target fetched or read: fetch=%v err=%v events=%v", fetch != nil, err, p.events)
	}
	// pr from pr reseals pr_head through the same strict gate, so it needs the fetch too,
	// but only after the caller is authorized.
	p.mutateErr = errors.New("not the holder")
	if fetch, err := service.prefetchForPhase("state", "io-1", "pr"); err == nil || fetch != nil {
		t.Fatalf("unauthorized reseal fetched: fetch=%v err=%v", fetch != nil, err)
	}
	if want := []string{"read", "authorize"}; !reflect.DeepEqual(p.events, want) {
		t.Fatalf("events=%v want %v", p.events, want)
	}
}

// ---- GateService ----

type gateHarness struct {
	discovered []string
	checks     []gatescontract.CheckRequest
	incomplete bool
}

func (g *gateHarness) ports() cycleport.GateLedgerReadiness {
	return cycleport.GateLedgerReadiness{
		Discover: func(string) ([]string, error) { return g.discovered, nil },
		Check: func(req gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
			g.checks = append(g.checks, req)
			return gatescontract.CheckResult{Complete: !g.incomplete}, nil
		},
	}
}

func TestStrictPRReadinessWithStateScopesLedgersToWorktreeAndIssue(t *testing.T) {
	g := &gateHarness{discovered: []string{"/wt/.issueops/issues/5/gates.md", "/wt/.issueops/issues/9/gates.md"}, incomplete: true}
	record := model.IssueOpsRecord{
		ID: "io-1", Repo: "/repo", WorktreePath: "/wt",
		BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "https://github.com/o/r/issues/5"},
	}
	var baseSeen string
	service := GateService{
		BaseReadiness: func(root string, _ model.IssueOpsRecord) model.IssueOpsReadiness {
			baseSeen = root
			return readyReadiness()
		},
		Ledger: g.ports(),
	}
	got := service.StrictPRReadinessWithState("state-root", record)
	if baseSeen != "state-root" {
		t.Fatalf("base readiness root=%q", baseSeen)
	}
	if got.Ready || !reflect.DeepEqual(got.Missing, []string{"gates_incomplete:.issueops/issues/5/gates.md"}) {
		t.Fatalf("got=%+v", got)
	}
	if len(g.checks) != 1 || g.checks[0].WorkspaceRoot != "/wt" || !reflect.DeepEqual(g.checks[0].Files, []string{"/wt/.issueops/issues/5/gates.md"}) {
		t.Fatalf("checks=%+v (the issue 9 ledger must be skipped)", g.checks)
	}
}

func TestStrictPRReadinessPrefersLinkedIssueNumberOverPreparedURL(t *testing.T) {
	g := &gateHarness{discovered: []string{"/repo/.issueops/issues/5/gates.md", "/repo/.issueops/issues/7/gates.md"}, incomplete: true}
	record := model.IssueOpsRecord{
		ID: "io-1", Repo: "/repo", IssueURL: "https://github.com/o/r/issues/7",
		BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "https://github.com/o/r/issues/5"},
	}
	service := GateService{BaseReadiness: func(string, model.IssueOpsRecord) model.IssueOpsReadiness { return readyReadiness() }, Ledger: g.ports()}
	got := service.StrictPRReadinessWithState("s", record)
	if !reflect.DeepEqual(got.Missing, []string{"gates_incomplete:.issueops/issues/7/gates.md"}) {
		t.Fatalf("got=%+v", got)
	}
}

func TestAdvancePhaseReportGuardsPRThenAdvances(t *testing.T) {
	g := &gateHarness{discovered: []string{"/repo/.issueops/issues/5/gates.md"}, incomplete: true}
	record := model.IssueOpsRecord{ID: "io-1", Repo: "/repo", Phase: model.IssueOpsPhaseFeedback, IssueURL: "https://github.com/o/r/issues/5"}
	advances := 0
	service := GateService{
		LoopReadiness: func(repo string) model.IssueOpsReadiness {
			if repo != "/repo" {
				t.Fatalf("loop readiness repo=%q", repo)
			}
			return readyReadiness()
		},
		ReadRecord: func(string, string) (model.IssueOpsRecord, error) { return record, nil },
		AdvanceRecord: func(string, string, string, model.IssueOpsActor) (model.IssueOpsRecord, model.IssueOpsTrackedMaterials, error) {
			advances++
			return record, model.IssueOpsTrackedMaterials{}, nil
		},
		Ledger: g.ports(),
	}
	got, _, err := service.AdvancePhaseReport("s", "io-1", "pr", model.IssueOpsActor{})
	if err == nil || !strings.Contains(err.Error(), "cannot enter pr phase: missing gates_incomplete:.issueops/issues/5/gates.md") || got.OK || advances != 0 {
		t.Fatalf("incomplete ledger must block the pr transition: err=%v advances=%d", err, advances)
	}

	g.incomplete = false
	if _, _, err := service.AdvancePhaseReport("s", "io-1", "pr", model.IssueOpsActor{}); err != nil || advances != 1 {
		t.Fatalf("complete ledger must advance exactly once: err=%v advances=%d", err, advances)
	}

	// Non-pr targets never consult the gate.
	g.incomplete = true
	g.checks = nil
	if _, _, err := service.AdvancePhaseReport("s", "io-1", "feedback", model.IssueOpsActor{}); err != nil || advances != 2 || len(g.checks) != 0 {
		t.Fatalf("non-pr target consulted ledgers: err=%v advances=%d checks=%d", err, advances, len(g.checks))
	}
}

// ---- ValidatePhaseEntry branches ----

func notReady(keys ...string) func(model.IssueOpsRecord) model.IssueOpsReadiness {
	return func(model.IssueOpsRecord) model.IssueOpsReadiness { return model.IssueOpsReadiness{Missing: keys} }
}

func TestValidatePhaseEntryNamesTheProbeThatBlocksEachPhase(t *testing.T) {
	ready := cycleport.PhaseEntryReadiness{
		Problem:       notReady("intent_contract"),
		Compatibility: notReady("compat_gap"),
		Implement:     notReady("impl_gap"),
		AISlopClean:   notReady("clean_gap"),
	}
	for _, tc := range []struct {
		from, to model.IssueOpsPhase
		want     string
	}{
		{model.IssueOpsPhaseProblem, model.IssueOpsPhaseGrill, "cannot enter grill phase: missing intent_contract"},
		{model.IssueOpsPhasePlan, model.IssueOpsPhaseCompatibilityReview, "cannot enter compatibility-review phase: missing compat_gap"},
		{model.IssueOpsPhaseCompatibilityReview, model.IssueOpsPhaseImplement, "cannot enter implement phase: missing impl_gap"},
		{model.IssueOpsPhaseImplement, model.IssueOpsPhaseAISlopClean, "cannot enter ai-slop-clean phase: missing clean_gap"},
	} {
		err := ValidatePhaseEntry(ready, model.IssueOpsRecord{Phase: tc.from}, tc.to)
		if err == nil || err.Error() != tc.want {
			t.Fatalf("%s→%s: err=%v want %q", tc.from, tc.to, err, tc.want)
		}
	}
}

func TestValidatePhaseEntryRejectsProgressionErrorsBeforeAnyProbe(t *testing.T) {
	probed := false
	ready := cycleport.PhaseEntryReadiness{Problem: func(model.IssueOpsRecord) model.IssueOpsReadiness {
		probed = true
		return readyReadiness()
	}}
	if err := ValidatePhaseEntry(ready, model.IssueOpsRecord{Phase: model.IssueOpsPhasePlan}, model.IssueOpsPhaseGrill); err == nil || probed {
		t.Fatalf("backward move: err=%v probed=%v", err, probed)
	}
}

func TestValidatePhaseEntryFeedbackRequiresAISlopCleanStamp(t *testing.T) {
	err := ValidatePhaseEntry(cycleport.PhaseEntryReadiness{}, model.IssueOpsRecord{Phase: model.IssueOpsPhaseAISlopClean, AISlopCleanAt: "  "}, model.IssueOpsPhaseFeedback)
	if err == nil || !strings.Contains(err.Error(), "cannot enter feedback phase before ai-slop-clean phase") {
		t.Fatalf("err=%v", err)
	}
	if err := ValidatePhaseEntry(cycleport.PhaseEntryReadiness{}, model.IssueOpsRecord{Phase: model.IssueOpsPhaseAISlopClean, AISlopCleanAt: "t"}, model.IssueOpsPhaseFeedback); err != nil {
		t.Fatalf("stamped record refused: %v", err)
	}
}

func TestValidatePhaseEntryDoneRequiresPRPhaseThenArtifactThenReleasedCompletion(t *testing.T) {
	ready := cycleport.PhaseEntryReadiness{RemoteArtifactMissing: func(model.IssueOpsRecord) []string { return nil }}
	record := model.IssueOpsRecord{Phase: model.IssueOpsPhaseFeedback}
	if err := ValidatePhaseEntry(ready, record, model.IssueOpsPhaseDone); err == nil || !strings.Contains(err.Error(), "before pr phase") {
		t.Fatalf("from feedback: %v", err)
	}
	record.Phase = model.IssueOpsPhasePR
	for _, execution := range []*model.Execution{
		nil,
		{Lease: model.WriteLease{Status: model.LeaseStatusReleased}},
		{Completion: &model.ExecutionCompletion{Generation: 1}, Lease: model.WriteLease{Status: model.LeaseStatusActive}},
	} {
		record.Execution = execution
		if err := ValidatePhaseEntry(ready, record, model.IssueOpsPhaseDone); err == nil || !strings.Contains(err.Error(), "before issueops execution completion") {
			t.Fatalf("execution=%+v err=%v", execution, err)
		}
	}
	record.Execution = &model.Execution{Completion: &model.ExecutionCompletion{Generation: 1}, Lease: model.WriteLease{Status: model.LeaseStatusReleased}}
	if err := ValidatePhaseEntry(ready, record, model.IssueOpsPhaseDone); err != nil {
		t.Fatalf("completed released execution refused: %v", err)
	}
}

// ---- PhaseCompletion remaining routes ----

func TestPhaseCompletionRoutesEachPhaseToItsOwnSource(t *testing.T) {
	calls := []string{}
	observations := cycleport.PhaseCompletionReadiness{
		AISlopClean: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			calls = append(calls, "clean")
			return model.IssueOpsReadiness{OK: true, Missing: []string{"from_clean_probe"}}
		},
		PR: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			calls = append(calls, "pr")
			return model.IssueOpsReadiness{OK: true, Ready: true, Missing: []string{"b_local", "b_local"}}
		},
	}
	record := model.IssueOpsRecord{}
	if got := PhaseCompletion(record, model.IssueOpsPhaseImplement, observations); !reflect.DeepEqual(got.Missing, []string{"from_clean_probe"}) || !reflect.DeepEqual(calls, []string{"clean"}) {
		t.Fatalf("implement completion=%+v calls=%v", got, calls)
	}
	if got := PhaseCompletion(record, model.IssueOpsPhaseAISlopClean, observations); got.Ready ||
		!reflect.DeepEqual(got.Missing, []string{"ai_slop_clean_at", "ai_slop_clean_fingerprint", "ai_slop_clean_head", "cleanup_evidence", "verification_evidence"}) {
		t.Fatalf("ai-slop-clean completion=%+v", got)
	}
	if got := PhaseCompletion(record, model.IssueOpsPhaseGrill, observations); got.Ready || !reflect.DeepEqual(got.Missing, []string{"branch", "domain_review", "issue_url", "split_decision"}) {
		t.Fatalf("grill completion=%+v", got)
	}
	if got := PhaseCompletion(record, model.IssueOpsPhaseCompatibilityReview, observations); got.Ready || !reflect.DeepEqual(got.Missing, []string{"compatibility_review"}) {
		t.Fatalf("compatibility completion=%+v", got)
	}
	if got := PhaseCompletion(record, model.IssueOpsPhaseFeedback, observations); !got.Ready || len(got.Missing) != 0 {
		t.Fatalf("feedback with no items must be complete: %+v", got)
	}
	unresolved := model.IssueOpsRecord{Feedback: []model.IssueOpsFeedbackItem{{Classification: "contract_change"}}}
	if got := PhaseCompletion(unresolved, model.IssueOpsPhaseFeedback, observations); got.Ready || !reflect.DeepEqual(got.Missing, []string{"contract_feedback_issue_update", "feedback_resolution"}) {
		t.Fatalf("unresolved contract feedback must block: %+v", got)
	}
	if !reflect.DeepEqual(calls, []string{"clean"}) {
		t.Fatalf("only the implement route may call the clean probe: %v", calls)
	}
}

func TestPhaseCompletionPRMergesLocalMissingWithRemoteArtifactAndTargetBranch(t *testing.T) {
	observations := cycleport.PhaseCompletionReadiness{PR: func(model.IssueOpsRecord) model.IssueOpsReadiness {
		return model.IssueOpsReadiness{OK: true, Ready: true, Missing: []string{"z_local", "a_local", "a_local"}}
	}}
	record := model.IssueOpsRecord{
		BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "main"},
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{
			URL: "https://example/pr/1", TargetBranch: "develop",
		},
	}
	got := PhaseCompletion(record, model.IssueOpsPhasePR, observations)
	if got.Ready || !reflect.DeepEqual(got.Missing, []string{"a_local", "target_branch_match", "z_local"}) {
		t.Fatalf("got=%+v", got)
	}
	record.RemoteArtifact = nil
	got = PhaseCompletion(record, model.IssueOpsPhasePR, observations)
	if !reflect.DeepEqual(got.Missing, []string{"a_local", "remote_artifact", "z_local"}) {
		t.Fatalf("got=%+v", got)
	}
	// A ready local probe with a complete remote artifact stays ready.
	ok := PhaseCompletion(model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "u"}},
		model.IssueOpsPhasePR,
		cycleport.PhaseCompletionReadiness{PR: func(model.IssueOpsRecord) model.IssueOpsReadiness { return readyReadiness() }})
	if !ok.Ready || len(ok.Missing) != 0 {
		t.Fatalf("ok=%+v", ok)
	}
}

func TestPhaseCompletionUnknownPhaseIsNeverReady(t *testing.T) {
	got := PhaseCompletion(model.IssueOpsRecord{}, model.IssueOpsPhase("bogus"), cycleport.PhaseCompletionReadiness{})
	if got.Ready || !got.OK || !reflect.DeepEqual(got.Missing, []string{"unknown_phase"}) {
		t.Fatalf("got=%+v", got)
	}
}

// ---- MutationAuthority.Authorize ----

func TestMutationAuthorityAuthorizeRejectsInvalidExecutionAndAllowsPlanningRecord(t *testing.T) {
	authority := NewMutationAuthority(func(string, string) bool { return true }, liveVerifier())
	err := authority.Authorize(context.Background(), model.IssueOpsRecord{Execution: &model.Execution{}}, model.IssueOpsActor{})
	if err == nil || !strings.Contains(err.Error(), "invalid IssueOps execution v1 record") {
		t.Fatalf("invalid execution accepted: %v", err)
	}
	// A record with no execution (planning phase) needs no holder.
	if err := authority.Authorize(context.Background(), model.IssueOpsRecord{}, model.IssueOpsActor{}); err != nil {
		t.Fatalf("planning record refused: %v", err)
	}
}

func TestVerifyHolderCallerSkipsVerificationUnlessLeaseIsActiveWithProcessReceipt(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 1, StartedAt: "s", Executable: "/h"}
	calls := 0
	verifier := verifierFunc(func(context.Context, model.NativeActor) (model.VerifiedActor, error) {
		calls++
		return model.VerifiedActor{}, nil
	})
	actor := &model.IssueOpsActor{Host: "codex", SessionID: "s"}
	for name, execution := range map[string]*model.Execution{
		"released lease": {Lease: model.WriteLease{Status: model.LeaseStatusReleased, Holder: &model.NativeActor{SessionProcess: &process}}},
		"holderless":     {Lease: model.WriteLease{Status: model.LeaseStatusActive}},
		"no process":     {Lease: model.WriteLease{Status: model.LeaseStatusActive, Holder: &model.NativeActor{Host: "codex", SessionID: "s"}}},
	} {
		verified, err := verifyHolderCaller(context.Background(), model.IssueOpsRecord{Execution: execution}, actor, verifier)
		if verified != nil || err != nil || calls != 0 {
			t.Fatalf("%s: verified=%v err=%v calls=%d", name, verified, err, calls)
		}
	}
}
