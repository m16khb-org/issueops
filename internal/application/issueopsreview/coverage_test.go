package issueopsreview

import (
	"errors"
	"slices"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

const covNow = "2026-10-01T00:00:00Z"

// memStore is an in-memory record behind every store shape the review use cases take.
type memStore struct {
	rec         model.IssueOpsRecord
	reads       int
	writes      int
	readErrAt   int // 1-based read number that fails; 0 = never
	readErr     error
	validateErr error
	writeErr    error
	events      []string
}

func (m *memStore) read(_, _ string) (model.IssueOpsRecord, error) {
	m.reads++
	m.events = append(m.events, "read")
	if m.readErrAt == m.reads {
		return model.IssueOpsRecord{ID: "partial"}, m.readErr
	}
	return m.rec, nil
}

func (m *memStore) write(_ string, next model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	m.writes++
	m.events = append(m.events, "write")
	if m.writeErr != nil {
		return next, m.writeErr
	}
	m.rec = next
	return next, nil
}

func (m *memStore) lock(_, _ string, fn func() error) error {
	m.events = append(m.events, "lock")
	return fn()
}

func (m *memStore) validate(model.IssueOpsRecord) error {
	m.events = append(m.events, "authority")
	return m.validateErr
}

func (m *memStore) mutation() reviewport.ReviewMutationStore {
	return reviewport.ReviewMutationStore{WithLock: m.lock, Read: m.read, ValidateMutation: m.validate, Write: m.write, Now: func() string { return covNow }}
}

func (m *memStore) evidence() reviewport.EvidenceReviewStore {
	return reviewport.EvidenceReviewStore{
		Read: m.read, Fingerprint: func(model.IssueOpsRecord) string { return "fp" },
		WithLock: m.lock, ValidateMutation: m.validate, Write: m.write, Now: func() string { return covNow },
	}
}

func (m *memStore) planning() PlanningRecorder {
	return PlanningRecorder{
		Store:                  m.mutation(),
		PlanReadiness:          func(model.IssueOpsRecord) model.IssueOpsReadiness { return model.IssueOpsReadiness{Ready: true} },
		CompatibilityReadiness: func(model.IssueOpsRecord) model.IssueOpsReadiness { return model.IssueOpsReadiness{Ready: true} },
		PhaseRank:              issueopsdomain.IssueOpsPhaseRank,
		PlanDigest:             func(string, model.IssueOpsRecord) (string, error) { return "plan-digest", nil },
		ActiveChildren:         func(string, model.IssueOpsRecord) ([]string, error) { return nil, nil },
	}
}

func wantErr(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), substr) {
		t.Fatalf("err = %v, want containing %q", err, substr)
	}
}

// ---- LocalChangeObserver ----

type observerScript struct {
	paths        [][]string // per Paths call; nil entry = failure
	fingerprints []string   // per Fingerprint call; "" = failure
	pathCalls    int
	fpCalls      int
}

func (s *observerScript) source() reviewport.LocalChangeSource {
	return reviewport.LocalChangeSource{
		BaseRef: func(r model.IssueOpsRecord, root string) string { return "base:" + r.ID + ":" + root },
		Paths: func(_, _ string) ([]string, bool) {
			i := s.pathCalls
			s.pathCalls++
			if i >= len(s.paths) || s.paths[i] == nil {
				return nil, false
			}
			return s.paths[i], true
		},
		Fingerprint: func(string, []string) (string, bool) {
			i := s.fpCalls
			s.fpCalls++
			if i >= len(s.fingerprints) || s.fingerprints[i] == "" {
				return "", false
			}
			return s.fingerprints[i], true
		},
	}
}

func TestLocalChangeObserverReturnsNothingWithoutRootOrFingerprinter(t *testing.T) {
	s := &observerScript{paths: [][]string{{"a"}}}
	src := s.source()
	if got := (LocalChangeObserver{Source: src}).Observe(model.IssueOpsRecord{}, "  "); got.Verified || got.Paths != nil || s.pathCalls != 0 {
		t.Fatalf("blank root must not observe: %+v calls=%d", got, s.pathCalls)
	}
	src.Fingerprint = nil
	if got := (LocalChangeObserver{Source: src}).Observe(model.IssueOpsRecord{}, "/repo"); got.Verified || got.Paths != nil || s.pathCalls != 0 {
		t.Fatalf("missing fingerprinter must not observe: %+v calls=%d", got, s.pathCalls)
	}
}

func TestLocalChangeObserverVerifiesStableSnapshotAndDoesNotAliasSourcePaths(t *testing.T) {
	source := []string{"a.go", "b.go"}
	s := &observerScript{paths: [][]string{source, source}, fingerprints: []string{"fp", "fp"}}
	got := (LocalChangeObserver{Source: s.source()}).Observe(model.IssueOpsRecord{ID: "io-1"}, "/repo")
	if !got.Verified || got.Fingerprint != "fp" || !slices.Equal(got.Paths, source) || s.pathCalls != 2 || s.fpCalls != 2 {
		t.Fatalf("stable snapshot: %+v paths=%d fps=%d", got, s.pathCalls, s.fpCalls)
	}
	got.Paths[0] = "mutated"
	if source[0] != "a.go" {
		t.Fatal("observation must not alias the source's path slice")
	}
}

func TestLocalChangeObserverRetriesOnceWhenSnapshotMovesBetweenReads(t *testing.T) {
	s := &observerScript{
		paths:        [][]string{{"a"}, {"a"}, {"a", "b"}, {"a", "b"}},
		fingerprints: []string{"one", "two", "three", "three"},
	}
	got := (LocalChangeObserver{Source: s.source()}).Observe(model.IssueOpsRecord{}, "/repo")
	if !got.Verified || got.Fingerprint != "three" || !slices.Equal(got.Paths, []string{"a", "b"}) {
		t.Fatalf("second attempt must win: %+v", got)
	}
}

func TestLocalChangeObserverNeverVerifiesTwoUnstableAttempts(t *testing.T) {
	s := &observerScript{
		paths:        [][]string{{"a"}, {"a"}, {"b"}, {"c"}},
		fingerprints: []string{"1", "2", "3", "4"},
	}
	got := (LocalChangeObserver{Source: s.source()}).Observe(model.IssueOpsRecord{}, "/repo")
	if got.Verified || got.Fingerprint != "" || !slices.Equal(got.Paths, []string{"c"}) {
		t.Fatalf("unstable snapshot must carry the last paths and no fingerprint: %+v", got)
	}
	if s.pathCalls != 4 {
		t.Fatalf("exactly one retry allowed, Paths called %d times", s.pathCalls)
	}
}

func TestLocalChangeObserverTreatsDifferingPathsAsUnstableEvenWithSameFingerprint(t *testing.T) {
	s := &observerScript{
		paths:        [][]string{{"a"}, {"b"}, {"a"}, {"b"}},
		fingerprints: []string{"x", "x", "x", "x"},
	}
	got := (LocalChangeObserver{Source: s.source()}).Observe(model.IssueOpsRecord{}, "/repo")
	if got.Verified {
		t.Fatalf("paths changed between reads but snapshot was verified: %+v", got)
	}
}

func TestLocalChangeObserverGivesUpWhenPathsUnreadable(t *testing.T) {
	// first Paths read fails immediately: nothing to report, no fingerprint attempt
	s := &observerScript{paths: [][]string{nil}}
	got := (LocalChangeObserver{Source: s.source()}).Observe(model.IssueOpsRecord{}, "/repo")
	if got.Verified || got.Paths != nil || s.fpCalls != 0 || s.pathCalls != 1 {
		t.Fatalf("got=%+v fps=%d paths=%d", got, s.fpCalls, s.pathCalls)
	}
	// retry attempt cannot read paths: keeps the paths from the failed first attempt
	s = &observerScript{paths: [][]string{{"a"}, {"b"}, nil}, fingerprints: []string{"1", "2"}}
	got = (LocalChangeObserver{Source: s.source()}).Observe(model.IssueOpsRecord{}, "/repo")
	if got.Verified || !slices.Equal(got.Paths, []string{"b"}) {
		t.Fatalf("unreadable retry must keep last observed paths: %+v", got)
	}
}

func TestLocalChangeObserverRetriesWhenFingerprintFails(t *testing.T) {
	s := &observerScript{paths: [][]string{{"a"}, {"a"}, {"a"}}, fingerprints: []string{"", "ok", "ok"}}
	got := (LocalChangeObserver{Source: s.source()}).Observe(model.IssueOpsRecord{}, "/repo")
	if !got.Verified || got.Fingerprint != "ok" || s.pathCalls != 3 {
		t.Fatalf("failed fingerprint must trigger one retry: %+v paths=%d", got, s.pathCalls)
	}
	s = &observerScript{paths: [][]string{{"a"}, {"a"}, {"a"}, {"a"}}, fingerprints: []string{"ok", "", "ok", "ok"}}
	got = (LocalChangeObserver{Source: s.source()}).Observe(model.IssueOpsRecord{}, "/repo")
	if !got.Verified || got.Fingerprint != "ok" {
		t.Fatalf("failed second fingerprint must trigger one retry: %+v", got)
	}
	s = &observerScript{paths: [][]string{{"a"}, nil, {"a"}, {"a"}}, fingerprints: []string{"ok", "ok", "ok"}}
	got = (LocalChangeObserver{Source: s.source()}).Observe(model.IssueOpsRecord{}, "/repo")
	if !got.Verified || got.Fingerprint != "ok" || s.pathCalls != 4 || s.fpCalls != 3 {
		t.Fatalf("unreadable second Paths read must trigger one retry: %+v paths=%d fps=%d", got, s.pathCalls, s.fpCalls)
	}
}

// ---- record.go ----

func TestRecordDevilsAdvocateValidatesBeforeReading(t *testing.T) {
	reads := 0
	store := reviewport.DevilsAdvocateStore{Read: func(string, string) (model.IssueOpsRecord, error) { reads++; return model.IssueOpsRecord{}, nil }}
	_, err := RecordDevilsAdvocate(store, "s", "id", reviewcontract.DevilsAdvocateReviewRequest{Verdict: "maybe", ReviewerContext: "inline"}, "now")
	wantErr(t, err, "verdict must be pass, revise, or stop")
	if reads != 0 {
		t.Fatal("invalid request must be rejected before any state read")
	}
}

func TestRecordDevilsAdvocatePropagatesReadFailureAndRequiresResolver(t *testing.T) {
	boom := errors.New("read failed")
	req := reviewcontract.DevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "inline", Findings: []string{"attacked rollback"}}
	_, err := RecordDevilsAdvocate(reviewport.DevilsAdvocateStore{Read: func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, boom }}, "s", "id", req, "now")
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	writes := 0
	store := reviewport.DevilsAdvocateStore{
		Read: func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, nil },
		TouchWrite: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			writes++
			return model.IssueOpsRecord{}, nil
		},
	}
	_, err = RecordDevilsAdvocate(store, "s", "id", req, "now")
	wantErr(t, err, "plan digest resolver is unavailable")
	if writes != 0 {
		t.Fatal("no write without a plan digest resolver")
	}
}

func TestRecordDevilsAdvocateBindsDigestKeepsHistoryAndEnforcesReviseCap(t *testing.T) {
	prior := &model.IssueOpsDevilsAdvocateReview{Verdict: "revise", Findings: []string{"gap"}, ReviewerContext: "inline", RecordedAt: "earlier"}
	var written model.IssueOpsRecord
	store := reviewport.DevilsAdvocateStore{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{ID: "io", DevilsAdvocateReview: prior}, nil
		},
		PlanDigest: func(string, model.IssueOpsRecord) (string, error) { return "digest-1", nil },
		TouchWrite: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) { written = r; return r, nil },
	}
	out, err := RecordDevilsAdvocate(store, "s", "io", reviewcontract.DevilsAdvocateReviewRequest{Verdict: "PASS", ReviewerContext: "Subagent", Findings: []string{"attacked rollback"}}, "now")
	if err != nil {
		t.Fatal(err)
	}
	review := written.DevilsAdvocateReview
	if review == nil || out.DevilsAdvocateReview != review || review.Verdict != "pass" || review.ReviewedPlanDigest != "digest-1" || review.RecordedAt != "now" {
		t.Fatalf("recorded review = %+v", review)
	}
	if len(review.History) != 1 || review.History[0].Verdict != "revise" || review.History[0].RecordedAt != "earlier" {
		t.Fatalf("previous round must be preserved in history: %+v", review.History)
	}

	// the sixth unwaived revise is refused and nothing is written
	capped := &model.IssueOpsDevilsAdvocateReview{Verdict: "revise", History: make([]reviewcontract.DevilsAdvocateRound, 4)}
	for i := range capped.History {
		capped.History[i] = reviewcontract.DevilsAdvocateRound{Verdict: "revise"}
	}
	writes := 0
	store.Read = func(string, string) (model.IssueOpsRecord, error) {
		return model.IssueOpsRecord{DevilsAdvocateReview: capped}, nil
	}
	store.TouchWrite = func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		writes++
		return model.IssueOpsRecord{}, nil
	}
	_, err = RecordDevilsAdvocate(store, "s", "io", reviewcontract.DevilsAdvocateReviewRequest{Verdict: "revise", ReviewerContext: "inline", Findings: []string{"again"}}, "now")
	wantErr(t, err, "revise round cap reached")
	if writes != 0 {
		t.Fatal("capped review must not be written")
	}
}

func TestRecordCompatibilityReviewRejectsInvalidRequestAndReadFailureBeforeWriting(t *testing.T) {
	writes := 0
	reads := 0
	boom := errors.New("read failed")
	store := reviewport.CompatibilityStore{
		Read: func(string, string) (model.IssueOpsRecord, error) { reads++; return model.IssueOpsRecord{}, boom },
		TouchWrite: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			writes++
			return model.IssueOpsRecord{}, nil
		},
		Ready:     func(model.IssueOpsRecord) model.IssueOpsReadiness { return model.IssueOpsReadiness{Ready: true} },
		PhaseRank: issueopsdomain.IssueOpsPhaseRank,
	}
	_, err := RecordCompatibilityReview(store, "s", "id", reviewcontract.CompatibilityReviewRequest{}, "now")
	wantErr(t, err, "backward_compatibility")
	if reads != 0 {
		t.Fatal("invalid request must not read state")
	}
	req := reviewcontract.CompatibilityReviewRequest{BackwardCompatibility: []string{"ok"}, SideEffects: []string{"none"}, Verification: []string{"tested"}, RollbackPlan: "revert"}
	if _, err := RecordCompatibilityReview(store, "s", "id", req, "now"); !errors.Is(err, boom) || writes != 0 {
		t.Fatalf("err=%v writes=%d", err, writes)
	}
}

func TestRecordCompatibilityReviewNeverMovesPhaseBackward(t *testing.T) {
	req := reviewcontract.CompatibilityReviewRequest{BackwardCompatibility: []string{"ok"}, SideEffects: []string{"none"}, Verification: []string{"tested"}, RollbackPlan: "revert"}
	for _, phase := range []model.IssueOpsPhase{model.IssueOpsPhaseCompatibilityReview, model.IssueOpsPhaseImplement} {
		var written model.IssueOpsRecord
		store := reviewport.CompatibilityStore{
			Read:       func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{Phase: phase}, nil },
			TouchWrite: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) { written = r; return r, nil },
			Ready:      func(model.IssueOpsRecord) model.IssueOpsReadiness { return model.IssueOpsReadiness{Ready: true} },
			PhaseRank:  issueopsdomain.IssueOpsPhaseRank,
		}
		if _, err := RecordCompatibilityReview(store, "s", "id", req, covNow); err != nil {
			t.Fatal(err)
		}
		if written.Phase != phase || written.CompatibilityReview == nil {
			t.Fatalf("phase %s must be kept and review recorded: %+v", phase, written)
		}
	}
}

// ---- PlanningRecorder ----

func TestPlanningRecorderPlanPrepStoresEvidenceAndWaiversWithClock(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhasePlan}}
	evidence := model.IssueOpsPlanPrepItemRequest{Evidence: []string{" read ADR "}}
	out, err := m.planning().PlanPrep("s", "io", model.IssueOpsPlanPrepRequest{
		PriorDecisions: evidence, RelatedIssues: model.IssueOpsPlanPrepItemRequest{WaiveReason: "none exist"},
		WebResearch: evidence, CodebaseSurvey: evidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.PlanPrep == nil || out.PlanPrep.RelatedIssues.Status != "waived" || out.PlanPrep.PriorDecisions.Status != "evidence" ||
		out.PlanPrep.PriorDecisions.Evidence[0] != "read ADR" || out.PlanPrep.RecordedAt != covNow || out.UpdatedAt != covNow || m.writes != 1 {
		t.Fatalf("plan prep = %+v writes=%d", out, m.writes)
	}
	if got := strings.Join(m.events, ","); !strings.HasPrefix(got, "lock,read,authority") {
		t.Fatalf("authority must be checked under the lock first: %s", got)
	}
}

func TestPlanningRecorderPlanPrepRejectsIncompleteItemWithoutWrite(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io"}}
	_, err := m.planning().PlanPrep("s", "io", model.IssueOpsPlanPrepRequest{})
	wantErr(t, err, "provide evidence or a waive reason")
	if m.writes != 0 {
		t.Fatal("invalid plan prep must not write")
	}
}

func TestPlanningRecorderDesignExplainsMissingApprovalEvidence(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhasePlan}}
	req := reviewcontract.DesignReviewRequest{
		ProblemSummary: "p", ProposedDesign: "d", RefactorPlan: "r", Alternatives: []string{"a"}, Risks: []string{"x"},
		Verification: []string{"unit tests pass"}, Approved: true,
	}
	_, err := m.planning().Design("s", "io", req)
	wantErr(t, err, "design_review_evidence")
	if m.writes != 0 {
		t.Fatal("rejected approval must not write")
	}
	req.Verification = []string{"design review checked alternatives and risks"}
	out, err := m.planning().Design("s", "io", req)
	if err != nil || out.DesignReview == nil || !out.DesignReview.Approved || out.DesignReview.ReviewedAt != covNow || out.UpdatedAt != covNow || m.writes != 1 {
		t.Fatalf("approved design: %+v err=%v writes=%d", out, err, m.writes)
	}
}

func TestPlanningRecorderDesignPassesThroughOtherErrorsUnchanged(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io"}}
	_, err := m.planning().Design("s", "io", reviewcontract.DesignReviewRequest{})
	if err == nil || err.Error() != "problem_summary is required" {
		t.Fatalf("err=%v", err)
	}
}

func TestRecordDesignReviewReadinessIgnoresOnlyPlanPrepGaps(t *testing.T) {
	var written model.IssueOpsRecord
	missing := []string{"plan_prep_decisions"}
	store := reviewport.DesignReviewStore{
		Read:          func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{ID: "io"}, nil },
		PlanReadiness: func(model.IssueOpsRecord) model.IssueOpsReadiness { return model.IssueOpsReadiness{Missing: missing} },
		TouchWrite:    func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) { written = r; return r, nil },
		Now:           func() string { return covNow },
	}
	req := reviewcontract.DesignReviewRequest{ProblemSummary: "p", ProposedDesign: "d", Verification: []string{"checked"}}
	if _, err := RecordDesignReview(store, "s", "io", req); err != nil || written.DesignReview == nil {
		t.Fatalf("plan_prep-only gaps must not block: %v", err)
	}
	missing = []string{"plan_prep_decisions", "intent"}
	written = model.IssueOpsRecord{}
	_, err := RecordDesignReview(store, "s", "io", req)
	wantErr(t, err, "before intent contract: missing intent")
	if strings.Contains(err.Error(), "plan_prep") || written.DesignReview != nil {
		t.Fatalf("only non-plan-prep gaps may be listed and nothing written: %v", err)
	}
}

func TestRecordDesignReviewPropagatesReadFailureAndApprovalGaps(t *testing.T) {
	boom := errors.New("read failed")
	store := reviewport.DesignReviewStore{
		Read: func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, boom },
		TouchWrite: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			t.Fatal("must not write")
			return model.IssueOpsRecord{}, nil
		},
		Now: func() string { return covNow },
	}
	req := reviewcontract.DesignReviewRequest{ProblemSummary: "p", ProposedDesign: "d", Verification: []string{"v"}}
	if _, err := RecordDesignReview(store, "s", "io", req); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	store.Read = func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, nil }
	store.PlanReadiness = func(model.IssueOpsRecord) model.IssueOpsReadiness { return model.IssueOpsReadiness{Ready: true} }
	req.Approved = true
	_, err := RecordDesignReview(store, "s", "io", req)
	wantErr(t, err, "approved design review requires refactor_plan")
}

func TestPlanningRecorderCompatibilityAdvancesPhaseAndStampsUpdate(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhasePlan}}
	out, err := m.planning().Compatibility("s", "io", reviewcontract.CompatibilityReviewRequest{
		BackwardCompatibility: []string{"ok"}, SideEffects: []string{"none"}, Verification: []string{"tested"}, RollbackPlan: "revert",
	})
	if err != nil || out.Phase != model.IssueOpsPhaseCompatibilityReview || out.CompatibilityReview == nil || out.UpdatedAt != covNow || m.writes != 1 {
		t.Fatalf("compatibility: %+v err=%v", out, err)
	}
}

func TestPlanningRecorderDevilsAdvocateRecordsDigestAndExplainsReviseCap(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io-cap"}}
	out, err := m.planning().DevilsAdvocate("s", "io-cap", reviewcontract.DevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"attacked"}})
	if err != nil || out.DevilsAdvocateReview == nil || out.DevilsAdvocateReview.ReviewedPlanDigest != "plan-digest" || out.UpdatedAt != covNow {
		t.Fatalf("review: %+v err=%v", out, err)
	}
	m = &memStore{rec: model.IssueOpsRecord{ID: "io-cap", DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{
		Verdict: "revise", History: []reviewcontract.DevilsAdvocateRound{{Verdict: "revise"}, {Verdict: "revise"}, {Verdict: "revise"}, {Verdict: "revise"}},
	}}}
	_, err = m.planning().DevilsAdvocate("s", "io-cap", reviewcontract.DevilsAdvocateReviewRequest{Verdict: "revise", ReviewerContext: "inline", Findings: []string{"again"}})
	wantErr(t, err, "cycle io-cap already recorded 5 unwaived revise verdicts")
	wantErr(t, err, "issueops remote reflect-devils-advocate --id io-cap --confirm")
	wantErr(t, err, "issueops regress --id io-cap")
	if m.writes != 0 {
		t.Fatal("capped round must not be written")
	}
}

func TestPlanningRecorderRegressTransitionsUnderLockAndSurfacesActiveChildren(t *testing.T) {
	reflected := &model.IssueOpsDevilsAdvocateReview{Verdict: "stop", IssueReflectedAt: "t"}
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhasePlan, DevilsAdvocateReview: reflected}}
	out, err := m.planning().Regress("s", "io", "  scope moved  ")
	if err != nil || out.Phase != model.IssueOpsPhaseGrill || out.UpdatedAt != covNow || len(out.RegressEvents) != 1 || out.RegressEvents[0].Reason != "scope moved" || out.DevilsAdvocateReview != nil {
		t.Fatalf("regress: %+v err=%v", out, err)
	}

	m = &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhasePlan, DevilsAdvocateReview: reflected}}
	p := m.planning()
	p.ActiveChildren = func(string, model.IssueOpsRecord) ([]string, error) { return []string{"io-child"}, nil }
	_, err = p.Regress("s", "io", "scope moved")
	wantErr(t, err, "children_active: io-child")
	boom := errors.New("child lookup failed")
	p.ActiveChildren = func(string, model.IssueOpsRecord) ([]string, error) { return nil, boom }
	if _, err = p.Regress("s", "io", "scope moved"); !errors.Is(err, boom) || m.writes != 0 {
		t.Fatalf("err=%v writes=%d", err, m.writes)
	}
}

func TestPlanningRecorderPropagatesLockedReadFailure(t *testing.T) {
	boom := errors.New("read failed")
	m := &memStore{readErrAt: 1, readErr: boom}
	if _, err := m.planning().Regress("s", "io", "reason"); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if got := strings.Join(m.events, ","); got != "lock,read" {
		t.Fatalf("read failure must stop before authority: %s", got)
	}
}

// ---- Regress ----

func TestRegressRejectsBlankReasonBeforeReadingAndPropagatesStoreFailures(t *testing.T) {
	reads := 0
	boom := errors.New("read failed")
	store := reviewport.RegressStore{
		Read: func(string, string) (model.IssueOpsRecord, error) { reads++; return model.IssueOpsRecord{}, boom },
	}
	_, err := Regress(store, "s", "io", "   ")
	wantErr(t, err, "regression reason is required")
	if reads != 0 {
		t.Fatal("blank reason must not read")
	}
	if _, err = Regress(store, "s", "io", "reason"); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

func TestRegressRefusesIneligiblePhaseAndVerdictWithoutChildLookup(t *testing.T) {
	for name, tc := range map[string]struct {
		rec  model.IssueOpsRecord
		want string
	}{
		"wrong phase":     {model.IssueOpsRecord{Phase: model.IssueOpsPhaseImplement}, "only applies from plan or compatibility-review"},
		"no review":       {model.IssueOpsRecord{Phase: model.IssueOpsPhasePlan}, "requires a recorded devil's-advocate stop verdict"},
		"revise in place": {model.IssueOpsRecord{Phase: model.IssueOpsPhasePlan, DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{Verdict: "revise"}}, "must be resolved in place"},
		"not reflected":   {model.IssueOpsRecord{Phase: model.IssueOpsPhasePlan, DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{Verdict: "stop"}}, "reflect the devil's-advocate findings"},
	} {
		t.Run(name, func(t *testing.T) {
			store := reviewport.RegressStore{
				Read: func(string, string) (model.IssueOpsRecord, error) { return tc.rec, nil },
				ActiveChildren: func(string, model.IssueOpsRecord) ([]string, error) {
					t.Fatal("child lookup must follow persisted checks")
					return nil, nil
				},
				TouchWrite: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
					t.Fatal("must not write")
					return model.IssueOpsRecord{}, nil
				},
			}
			_, err := Regress(store, "s", "io", "reason")
			wantErr(t, err, tc.want)
		})
	}
}

// ---- feedback / mutateReviewRecord ----

func TestAddFeedbackRejectsInvalidInputBeforeSecondReadAndWrite(t *testing.T) {
	for name, tc := range map[string]struct {
		source, body, class string
		want                string
	}{
		"blank source":  {" ", "b", "defect", "feedback source is required"},
		"blank body":    {"s", " ", "defect", "feedback body is required"},
		"unknown class": {"s", "b", "nonsense", `unknown issueops feedback classification "nonsense"`},
	} {
		t.Run(name, func(t *testing.T) {
			m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseFeedback}}
			_, err := AddFeedback(m.mutation(), "s", "io", tc.source, tc.body, tc.class)
			wantErr(t, err, tc.want)
			if m.writes != 0 || m.reads != 1 {
				t.Fatalf("input validation must precede the second read: reads=%d writes=%d", m.reads, m.writes)
			}
		})
	}
}

func TestAddFeedbackRejectsDonePhaseAfterReread(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseDone}}
	_, err := AddFeedback(m.mutation(), "s", "io", "s", "b", "defect")
	wantErr(t, err, "cannot add feedback after done phase")
	if m.writes != 0 || m.reads != 2 {
		t.Fatalf("reads=%d writes=%d", m.reads, m.writes)
	}
}

func TestAddFeedbackRecordsNormalizedItemAndMovesCleanedCycleToFeedbackPhase(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhasePR, AISlopCleanAt: "clean"}}
	out, err := AddFeedback(m.mutation(), "s", "io", " bot ", " fix it ", " DEFECT ")
	if err != nil {
		t.Fatal(err)
	}
	item := out.Feedback[0]
	if item.Source != "bot" || item.Body != "fix it" || item.Classification != "defect" || item.CreatedAt != covNow || out.Phase != model.IssueOpsPhaseFeedback || out.UpdatedAt != covNow {
		t.Fatalf("feedback = %+v phase=%s", item, out.Phase)
	}
	m = &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement}}
	out, err = AddFeedback(m.mutation(), "s", "io", "bot", "note", " ")
	if err != nil || out.Phase != model.IssueOpsPhaseImplement || out.Feedback[0].Classification != "" {
		t.Fatalf("uncleaned cycle keeps its phase and unclassified feedback is accepted: %s %v", out.Phase, err)
	}
}

func TestMutateReviewRecordFailuresReturnNotOKResultAndNeverWrite(t *testing.T) {
	boom := errors.New("boom")
	for name, tc := range map[string]struct {
		setup func(*memStore)
		want  string
	}{
		"first read fails":   {func(m *memStore) { m.readErrAt, m.readErr = 1, boom }, "boom"},
		"authority rejected": {func(m *memStore) { m.validateErr = boom }, "boom"},
		"second read fails":  {func(m *memStore) { m.readErrAt, m.readErr = 2, boom }, "boom"},
		"write fails":        {func(m *memStore) { m.writeErr = boom }, "boom"},
	} {
		t.Run(name, func(t *testing.T) {
			m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseFeedback}}
			tc.setup(m)
			out, err := AddFeedback(m.mutation(), "s", "io", "bot", "body", "defect")
			wantErr(t, err, tc.want)
			if out.OK || out.ID != "" {
				t.Fatalf("failure must return an empty not-OK record: %+v", out)
			}
			if name != "write fails" && m.writes != 0 {
				t.Fatalf("writes=%d", m.writes)
			}
			if name == "first read fails" && slices.Contains(m.events, "authority") {
				t.Fatal("authority must not run when the record cannot be read")
			}
		})
	}
}

func TestResolveFeedbackValidatesResolutionAndIndex(t *testing.T) {
	seed := model.IssueOpsRecord{ID: "io", Feedback: []model.IssueOpsFeedbackItem{{Source: "a", Body: "b", Classification: "defect"}}}
	for name, tc := range map[string]struct {
		index      int
		resolution string
		want       string
	}{
		"unknown resolution": {0, "fixed", `unknown feedback resolution "fixed"`},
		"empty resolution":   {0, " ", "unknown feedback resolution"},
		"negative index":     {-1, "valid-defect", "feedback index -1 out of range (have 1 items)"},
		"index past end":     {1, "valid-defect", "feedback index 1 out of range (have 1 items)"},
	} {
		t.Run(name, func(t *testing.T) {
			m := &memStore{rec: seed}
			_, err := ResolveFeedback(m.mutation(), "s", "io", tc.index, tc.resolution)
			wantErr(t, err, tc.want)
			if m.writes != 0 {
				t.Fatal("rejected resolution must not write")
			}
		})
	}
	m := &memStore{rec: seed}
	out, err := ResolveFeedback(m.mutation(), "s", "io", 0, " Noise-Dismissed ")
	if err != nil || out.Feedback[0].Resolution != "noise-dismissed" || out.UpdatedAt != covNow {
		t.Fatalf("resolved: %+v %v", out, err)
	}
}

func TestMarkContractFeedbackIssueUpdatedStampsOnlyUnresolvedContractChanges(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Feedback: []model.IssueOpsFeedbackItem{
		{Classification: "contract_change"},
		{Classification: "contract_change", IssueUpdatedAt: "already"},
		{Classification: "defect"},
	}}}
	out, err := MarkContractFeedbackIssueUpdated(m.mutation(), "s", "io")
	if err != nil {
		t.Fatal(err)
	}
	got := []string{out.Feedback[0].IssueUpdatedAt, out.Feedback[1].IssueUpdatedAt, out.Feedback[2].IssueUpdatedAt}
	if !slices.Equal(got, []string{covNow, "already", ""}) {
		t.Fatalf("issue_updated_at stamps = %v", got)
	}
	_, err = MarkContractFeedbackIssueUpdated(m.mutation(), "s", "io")
	wantErr(t, err, "no unresolved contract_change feedback requires a remote issue update")
	if m.writes != 1 {
		t.Fatalf("second call has nothing to mark and must not write; writes=%d", m.writes)
	}
}

// ---- evidence recorders ----

func TestEvidenceRecordersRejectInvalidRequestsBeforeReadingState(t *testing.T) {
	for name, call := range map[string]func(*memStore) error{
		"implementation verdict": func(m *memStore) error {
			_, err := RecordImplementationReview(m.evidence(), "s", "io", model.IssueOpsImplementationReviewRequest{Verdict: "ok", Findings: []string{"f"}, Evidence: []string{"e"}})
			return err
		},
		"implementation evidence": func(m *memStore) error {
			_, err := RecordImplementationReview(m.evidence(), "s", "io", model.IssueOpsImplementationReviewRequest{Verdict: "pass", Findings: []string{"  "}, Evidence: []string{"e"}})
			return err
		},
		"schema without measurements": func(m *memStore) error {
			_, err := RecordSchemaEvidence(m.evidence(), "s", "io", model.IssueOpsSchemaEvidenceRequest{Sources: []string{"s"}})
			return err
		},
		"schema without sources": func(m *memStore) error {
			_, err := RecordSchemaEvidence(m.evidence(), "s", "io", model.IssueOpsSchemaEvidenceRequest{Measurements: []string{"m"}})
			return err
		},
		"schema waiver without rationale": func(m *memStore) error {
			_, err := RecordSchemaEvidence(m.evidence(), "s", "io", model.IssueOpsSchemaEvidenceRequest{Waive: true, WaiverRationale: " "})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement}}
			if err := call(m); err == nil || m.reads != 0 || m.writes != 0 {
				t.Fatalf("err=%v reads=%d writes=%d", err, m.reads, m.writes)
			}
		})
	}
}

func TestEvidenceRecordersRejectBeforeImplementPhaseAndNeverWrite(t *testing.T) {
	for name, call := range map[string]func(*memStore) error{
		"implementation review": func(m *memStore) error {
			_, err := RecordImplementationReview(m.evidence(), "s", "io", model.IssueOpsImplementationReviewRequest{Verdict: "pass", Findings: []string{"f"}, Evidence: []string{"e"}})
			return err
		},
		"schema evidence": func(m *memStore) error {
			_, err := RecordSchemaEvidence(m.evidence(), "s", "io", model.IssueOpsSchemaEvidenceRequest{Waive: true, WaiverRationale: "no schema"})
			return err
		},
		"project docs": func(m *memStore) error {
			_, err := RecordProjectDocsReview(docsStore(m), "s", "io", model.IssueOpsProjectDocsReviewRequest{Verdict: "no-change", ReviewedDocs: []string{"AGENTS.md"}, Evidence: []string{"e"}})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhasePlan}}
			err := call(m)
			wantErr(t, err, "can only be recorded from the implement phase onward (current: plan)")
			if m.writes != 0 {
				t.Fatal("must not write")
			}
		})
	}
}

func TestEvidenceRecordersPropagateStoreFailuresAsNotOK(t *testing.T) {
	boom := errors.New("boom")
	type recorder func(*memStore) (model.IssueOpsRecord, error)
	recorders := map[string]recorder{
		"implementation review": func(m *memStore) (model.IssueOpsRecord, error) {
			return RecordImplementationReview(m.evidence(), "s", "io", model.IssueOpsImplementationReviewRequest{Verdict: "pass", Findings: []string{"f"}, Evidence: []string{"e"}})
		},
		"schema evidence": func(m *memStore) (model.IssueOpsRecord, error) {
			return RecordSchemaEvidence(m.evidence(), "s", "io", model.IssueOpsSchemaEvidenceRequest{Measurements: []string{"m"}, Sources: []string{"s"}})
		},
		"project docs": func(m *memStore) (model.IssueOpsRecord, error) {
			return RecordProjectDocsReview(docsStore(m), "s", "io", model.IssueOpsProjectDocsReviewRequest{Verdict: "no-change", ReviewedDocs: []string{"AGENTS.md"}, Evidence: []string{"e"}})
		},
	}
	for name, rec := range recorders {
		for failure, setup := range map[string]func(*memStore){
			"observation read": func(m *memStore) { m.readErrAt, m.readErr = 1, boom },
			"locked read":      func(m *memStore) { m.readErrAt, m.readErr = 2, boom },
			"authority":        func(m *memStore) { m.validateErr = boom },
			"write":            func(m *memStore) { m.writeErr = boom },
		} {
			t.Run(name+"/"+failure, func(t *testing.T) {
				m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement}}
				setup(m)
				out, err := rec(m)
				if !errors.Is(err, boom) || out.OK || out.ID != "" {
					t.Fatalf("out=%+v err=%v", out, err)
				}
				if failure != "write" && m.writes != 0 {
					t.Fatal("must not write after a failed precondition")
				}
			})
		}
	}
}

func TestRecordImplementationReviewCleansEvidenceAndRecordsReviewer(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseAISlopClean}}
	out, err := RecordImplementationReview(m.evidence(), "s", "io", model.IssueOpsImplementationReviewRequest{
		Verdict: " Revise ", Findings: []string{" gap ", " "}, Evidence: []string{" go test "},
		ReviewerHost: " Claude ", ReviewerModel: " opus ", ReviewerEffort: " high ",
	})
	if err != nil {
		t.Fatal(err)
	}
	r := out.ImplementationReview
	if r == nil || r.Verdict != "revise" || !slices.Equal(r.Findings, []string{"gap"}) || !slices.Equal(r.Evidence, []string{"go test"}) ||
		r.ReviewerHost != "claude" || r.ReviewerModel != "opus" || r.ReviewerEffort != "high" || r.RecordedAt != covNow || r.ReviewedFingerprint != "fp" {
		t.Fatalf("review = %+v", r)
	}
}

func TestRecordSchemaEvidenceRecordsMeasurementsAndWaivers(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement}}
	out, err := RecordSchemaEvidence(m.evidence(), "s", "io", model.IssueOpsSchemaEvidenceRequest{Measurements: []string{" EXPLAIN ok "}, Sources: []string{" prod replica "}})
	e := out.SchemaEvidence
	if err != nil || e == nil || e.Waived || !slices.Equal(e.Measurements, []string{"EXPLAIN ok"}) || !slices.Equal(e.Sources, []string{"prod replica"}) || e.ReviewedFingerprint != "fp" || out.UpdatedAt != covNow {
		t.Fatalf("evidence=%+v err=%v", e, err)
	}
	out, err = RecordSchemaEvidence(m.evidence(), "s", "io", model.IssueOpsSchemaEvidenceRequest{Waive: true, WaiverRationale: " no schema change "})
	e = out.SchemaEvidence
	if err != nil || e == nil || !e.Waived || e.WaiverRationale != "no schema change" || len(e.Measurements) != 0 {
		t.Fatalf("waived evidence=%+v err=%v", e, err)
	}
}

func TestEvidenceRecordersRejectWorktreeOrBaseChangeDuringObservation(t *testing.T) {
	observed := model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/a", BranchPrepare: &model.IssueOpsBranchPrepare{BaseSHA: "1", BaseBranch: "main"}}
	for name, mutate := range map[string]func(*model.IssueOpsRecord){
		"base sha": func(r *model.IssueOpsRecord) {
			r.BranchPrepare = &model.IssueOpsBranchPrepare{BaseSHA: "2", BaseBranch: "main"}
		},
		"base branch": func(r *model.IssueOpsRecord) {
			r.BranchPrepare = &model.IssueOpsBranchPrepare{BaseSHA: "1", BaseBranch: "dev"}
		},
		"base dropped": func(r *model.IssueOpsRecord) { r.BranchPrepare = nil },
	} {
		t.Run(name, func(t *testing.T) {
			current := observed
			mutate(&current)
			m := &memStore{rec: observed}
			store := m.evidence()
			store.Read = func(string, string) (model.IssueOpsRecord, error) {
				m.reads++
				if m.reads == 1 {
					return observed, nil
				}
				return current, nil
			}
			_, err := RecordSchemaEvidence(store, "s", "io", model.IssueOpsSchemaEvidenceRequest{Waive: true, WaiverRationale: "r"})
			wantErr(t, err, "changed its worktree or base while the change set was observed")
			if m.writes != 0 {
				t.Fatal("must not write")
			}
		})
	}
}

func TestChangeObservationIdentityFallsBackToRepoAndIgnoresWhitespace(t *testing.T) {
	a := model.IssueOpsRecord{Repo: "/repo"}
	b := model.IssueOpsRecord{Repo: " /repo ", WorktreePath: " "}
	if validateCurrentChangeObservation(a, b) != nil {
		t.Fatal("repo root with padding must match")
	}
	b.WorktreePath = "/wt"
	if validateCurrentChangeObservation(a, b) == nil {
		t.Fatal("a newly assigned worktree is a different observation root")
	}
}

// ---- project docs ----

func docsStore(m *memStore) reviewport.ProjectDocsReviewStore {
	return reviewport.ProjectDocsReviewStore{
		EvidenceReviewStore: m.evidence(),
		ChangedPaths:        func(model.IssueOpsRecord) []string { return []string{"docs/a.md", ".issueops/ADR.md"} },
		Root:                func(model.IssueOpsRecord) string { return "/wt" },
		RelativePath: func(root, path string) string {
			rel, ok := strings.CutPrefix(path, root+"/")
			if !ok {
				if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "..") {
					return ""
				}
				return path
			}
			return rel
		},
		FileExists: func(_, rel string) bool { return rel != ".issueops/MISSING.md" },
	}
}

func TestRecordProjectDocsReviewRejectsInvalidRequestBeforeReading(t *testing.T) {
	for name, tc := range map[string]struct {
		req  model.IssueOpsProjectDocsReviewRequest
		want string
	}{
		"bad verdict":            {model.IssueOpsProjectDocsReviewRequest{Verdict: "done", Evidence: []string{"e"}}, "verdict must be updated|no-change"},
		"no evidence":            {model.IssueOpsProjectDocsReviewRequest{Verdict: "no-change", ReviewedDocs: []string{"AGENTS.md"}}, "requires at least one evidence entry"},
		"updated without docs":   {model.IssueOpsProjectDocsReviewRequest{Verdict: "updated", Evidence: []string{"e"}}, "requires at least one --doc"},
		"no-change lists docs":   {model.IssueOpsProjectDocsReviewRequest{Verdict: "no-change", Docs: []string{"a"}, Evidence: []string{"e"}}, "must not list updated docs"},
		"no-change reviewed nil": {model.IssueOpsProjectDocsReviewRequest{Verdict: "no-change", Evidence: []string{"e"}}, "requires at least one --reviewed-doc"},
	} {
		t.Run(name, func(t *testing.T) {
			m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement}}
			_, err := RecordProjectDocsReview(docsStore(m), "s", "io", tc.req)
			wantErr(t, err, tc.want)
			if m.reads != 0 {
				t.Fatal("invalid request must not read")
			}
		})
	}
}

func TestRecordProjectDocsReviewNormalizesUpdatedDocsAgainstChangeSet(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement}}
	out, err := RecordProjectDocsReview(docsStore(m), "s", "io", model.IssueOpsProjectDocsReviewRequest{
		Verdict: "UPDATED", Docs: []string{"/wt/.issueops/ADR.md"}, Evidence: []string{" diff reviewed "}, ReviewedDocs: []string{"AGENTS.md"},
	})
	r := out.ProjectDocsReview
	if err != nil || r == nil || r.Verdict != "updated" || !slices.Equal(r.Docs, []string{".issueops/ADR.md"}) || !slices.Equal(r.ReviewedDocs, []string{"AGENTS.md"}) || r.ReviewedFingerprint != "fp" || out.UpdatedAt != covNow {
		t.Fatalf("review=%+v err=%v", r, err)
	}
}

func TestRecordProjectDocsReviewDeferredPathErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		req  model.IssueOpsProjectDocsReviewRequest
		want string
	}{
		"updated doc outside worktree": {model.IssueOpsProjectDocsReviewRequest{Verdict: "updated", Docs: []string{"/elsewhere/x.md"}, Evidence: []string{"e"}}, "must be inside the worktree"},
		"updated doc not changed":      {model.IssueOpsProjectDocsReviewRequest{Verdict: "updated", Docs: []string{"docs/other.md"}, Evidence: []string{"e"}}, "not in the current change set"},
		"reviewed doc outside":         {model.IssueOpsProjectDocsReviewRequest{Verdict: "no-change", ReviewedDocs: []string{"/elsewhere/AGENTS.md"}, Evidence: []string{"e"}}, "reviewed path"},
		"reviewed doc not project doc": {model.IssueOpsProjectDocsReviewRequest{Verdict: "no-change", ReviewedDocs: []string{"README.md"}, Evidence: []string{"e"}}, "is not a project doc"},
		"reviewed doc missing":         {model.IssueOpsProjectDocsReviewRequest{Verdict: "no-change", ReviewedDocs: []string{".issueops/MISSING.md"}, Evidence: []string{"e"}}, "does not exist as a file"},
	} {
		t.Run(name, func(t *testing.T) {
			m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement}}
			_, err := RecordProjectDocsReview(docsStore(m), "s", "io", tc.req)
			wantErr(t, err, tc.want)
			if m.writes != 0 {
				t.Fatal("must not write")
			}
		})
	}
}

func TestRecordProjectDocsReviewCannotVerifyReviewedDocsWithoutRoot(t *testing.T) {
	m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement}}
	store := docsStore(m)
	store.Root = func(model.IssueOpsRecord) string { return "" }
	_, err := RecordProjectDocsReview(store, "s", "io", model.IssueOpsProjectDocsReviewRequest{Verdict: "no-change", ReviewedDocs: []string{"AGENTS.md"}, Evidence: []string{"e"}})
	wantErr(t, err, "cannot verify reviewed path AGENTS.md without a worktree or repo root")
}

func TestRecordProjectDocsReviewChangedObservationBeatsPathErrors(t *testing.T) {
	observed := model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/a"}
	current := observed
	current.WorktreePath = "/b"
	m := &memStore{rec: observed}
	store := docsStore(m)
	store.Read = func(string, string) (model.IssueOpsRecord, error) {
		m.reads++
		if m.reads == 1 {
			return observed, nil
		}
		return current, nil
	}
	_, err := RecordProjectDocsReview(store, "s", "io", model.IssueOpsProjectDocsReviewRequest{Verdict: "updated", Docs: []string{"docs/other.md"}, Evidence: []string{"e"}})
	wantErr(t, err, "changed its worktree or base")
}

// ---- AI slop clean ----

func TestRecordAISlopCleanEvidencePropagatesStoreFailures(t *testing.T) {
	boom := errors.New("boom")
	for name, setup := range map[string]func(*memStore){
		"first read":  func(m *memStore) { m.readErrAt, m.readErr = 1, boom },
		"authority":   func(m *memStore) { m.validateErr = boom },
		"second read": func(m *memStore) { m.readErrAt, m.readErr = 2, boom },
		"write":       func(m *memStore) { m.writeErr = boom },
	} {
		t.Run(name, func(t *testing.T) {
			m := &memStore{rec: model.IssueOpsRecord{ID: "io", Phase: model.IssueOpsPhaseImplement}}
			setup(m)
			store := reviewport.AISlopCleanStore{ReviewMutationStore: m.mutation(), Refresh: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
				t.Fatal("not a completed clean phase")
				return model.IssueOpsRecord{}, nil
			}}
			_, err := RecordAISlopCleanEvidence(store, "s", "io", []string{"dead code"}, []string{"go test"})
			if !errors.Is(err, boom) {
				t.Fatalf("err=%v", err)
			}
			if name != "write" && m.writes != 0 {
				t.Fatal("must not write")
			}
		})
	}
}
