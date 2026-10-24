package issueopsdelegation

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"time"

	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
)

type delegationStore struct {
	records  map[string]model.IssueOpsRecord
	locked   bool
	saves    int
	scanErr  error
	saveErr  error
	loadErrs map[string]error
}

func newDelegationStore(records ...model.IssueOpsRecord) *delegationStore {
	s := &delegationStore{records: map[string]model.IssueOpsRecord{}}
	for _, r := range records {
		s.records[r.ID] = r
	}
	return s
}

func (s *delegationStore) WithinLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	s.locked = true
	defer func() { s.locked = false }()
	return fn(ctx)
}
func (s *delegationStore) Load(id string) (model.IssueOpsRecord, error) {
	if !s.locked {
		panic("load outside lock")
	}
	if err := s.loadErrs[id]; err != nil {
		return model.IssueOpsRecord{}, err
	}
	r, ok := s.records[id]
	if !ok {
		return r, fs.ErrNotExist
	}
	return r, nil
}
func (s *delegationStore) Save(_ context.Context, r model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	if s.saveErr != nil {
		return r, s.saveErr
	}
	s.records[r.ID] = r
	s.saves++
	return r, nil
}
func (s *delegationStore) SavePair(_ context.Context, a, b model.IssueOpsRecord) (model.IssueOpsRecord, model.IssueOpsRecord, error) {
	if s.saveErr != nil {
		return a, b, s.saveErr
	}
	s.records[a.ID], s.records[b.ID] = a, b
	s.saves++
	return a, b, nil
}
func (s *delegationStore) Scan() ([]model.IssueOpsRecord, error) {
	if s.scanErr != nil {
		return nil, s.scanErr
	}
	var out []model.IssueOpsRecord
	for _, r := range s.records {
		out = append(out, r)
	}
	return out, nil
}

type fixedIdentity struct{}

func (fixedIdentity) CanonicalRepo(repo string) string { return strings.TrimSpace(repo) }
func (fixedIdentity) StableID(repo, branch string) string {
	return "io-" + strings.ReplaceAll(branch, "/", "-")
}

var fixedNow = func() time.Time { return time.Date(2026, 9, 28, 1, 2, 3, 0, time.UTC) }

const fixedStamp = "2026-09-28T01:02:03Z"

func authority() cycleapp.MutationAuthority { return cycleapp.NewMutationAuthority(nil, nil) }

func readyParent() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID: "parent", Repo: "/repo", Branch: "1-parent", Phase: model.IssueOpsPhaseImplement,
		ArchitectureReview:   &reviewcontract.ArchitectureReview{Approved: true},
		CompatibilityReview:  &reviewcontract.CompatibilityReview{Approved: true},
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{Verdict: "proceed", RecordedAt: "2026-09-27T00:00:00Z"},
	}
}

func startRequest() model.IssueOpsChildStartRequest {
	return model.IssueOpsChildStartRequest{
		ParentID: " parent ", Branch: " 2-child ", Title: "child", TaskScope: " do it ",
		AcceptanceCriteria: []string{" works ", " "},
	}
}

func newStarter(store *delegationStore) ChildStarter {
	return ChildStarter{
		Records: store, Identity: fixedIdentity{}, Authority: authority(), Now: fixedNow,
		Links: branchapp.Linker{Records: store, Authority: authority(), Now: fixedNow},
	}
}

func TestChildStartCreatesDelegatedChildAndIndexesItOnParent(t *testing.T) {
	store := newDelegationStore(readyParent())
	got, err := newStarter(store).Start(context.Background(), startRequest(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.ParentID != "parent" || got.Child.ID != "io-2-child" || got.Child.Repo != "/repo" || got.Child.Branch != "2-child" {
		t.Fatalf("result=%+v", got)
	}
	if d := got.Child.Delegation; d == nil || d.ParentCycleID != "parent" || d.TaskScope != "do it" || !reflect.DeepEqual(d.AcceptanceCriteria, []string{"works"}) {
		t.Fatalf("delegation=%+v", got.Child.Delegation)
	}
	if got.Child.DevilsAdvocateReview == nil || got.Child.UpdatedAt != fixedStamp {
		t.Fatalf("child review/updated: %+v", got.Child)
	}
	if !strings.Contains(got.Guidance, "base_branch=1-parent") || !strings.Contains(got.Guidance, "2-child") {
		t.Fatalf("guidance=%q", got.Guidance)
	}
	parent := store.records["parent"]
	if len(parent.ChildCycles) != 1 || parent.ChildCycles[0].CycleID != "io-2-child" || parent.UpdatedAt != fixedStamp {
		t.Fatalf("parent index=%+v updated=%q", parent.ChildCycles, parent.UpdatedAt)
	}
	if got.ParentRef.CycleID != "io-2-child" || store.records["io-2-child"].ID == "" {
		t.Fatalf("ref=%+v", got.ParentRef)
	}
}

func TestChildStartIsIdempotentForExistingChild(t *testing.T) {
	store := newDelegationStore(readyParent())
	starter := newStarter(store)
	if _, err := starter.Start(context.Background(), startRequest(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := starter.Start(context.Background(), startRequest(), nil); err != nil {
		t.Fatal(err)
	}
	if n := len(store.records["parent"].ChildCycles); n != 1 {
		t.Fatalf("duplicate child refs: %d", n)
	}
}

func TestChildStartRejectionsLeaveStoreUntouched(t *testing.T) {
	notReady := readyParent()
	notReady.Phase = model.IssueOpsPhasePlan
	notReady.ArchitectureReview = nil
	badRepo := readyParent()
	badRepo.Repo = "  "
	cases := []struct {
		name   string
		parent model.IssueOpsRecord
		mutate func(*model.IssueOpsChildStartRequest)
		want   string
	}{
		{"missing parent id", readyParent(), func(r *model.IssueOpsChildStartRequest) { r.ParentID = " " }, "parent_id is required"},
		{"missing branch", readyParent(), func(r *model.IssueOpsChildStartRequest) { r.Branch = "" }, "branch is required"},
		{"missing scope", readyParent(), func(r *model.IssueOpsChildStartRequest) { r.TaskScope = "" }, "task_scope is required"},
		{"missing acceptance", readyParent(), func(r *model.IssueOpsChildStartRequest) { r.AcceptanceCriteria = []string{" "} }, "acceptance_criteria"},
		{"unready parent", notReady, nil, "parent_phase_not_implement, parent_architecture_review_unapproved"},
		{"same branch", readyParent(), func(r *model.IssueOpsChildStartRequest) { r.Branch = "1-parent" }, "child_branch_equals_parent"},
		{"empty repo", badRepo, nil, "repo is required"},
		{"invalid branch", readyParent(), func(r *model.IssueOpsChildStartRequest) { r.Branch = "no-number" }, "issue number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newDelegationStore(tc.parent)
			req := startRequest()
			if tc.mutate != nil {
				tc.mutate(&req)
			}
			got, err := newStarter(store).Start(context.Background(), req, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) || got.OK || store.saves != 0 {
				t.Fatalf("got=%+v err=%v saves=%d", got, err, store.saves)
			}
		})
	}
}

func TestChildStartPropagatesStoreErrors(t *testing.T) {
	boom := errors.New("disk")
	store := newDelegationStore()
	if _, err := newStarter(store).Start(context.Background(), startRequest(), nil); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing parent err=%v", err)
	}
	store = newDelegationStore(readyParent())
	store.loadErrs = map[string]error{"io-2-child": boom}
	if _, err := newStarter(store).Start(context.Background(), startRequest(), nil); !errors.Is(err, boom) {
		t.Fatalf("child load err=%v", err)
	}
	store = newDelegationStore(readyParent())
	store.saveErr = boom
	if got, err := newStarter(store).Start(context.Background(), startRequest(), nil); !errors.Is(err, boom) || got.OK {
		t.Fatalf("save err=%v got=%+v", err, got)
	}
}

func TestChildStartRejectsChildWithCleanupInProgress(t *testing.T) {
	store := newDelegationStore(readyParent(), model.IssueOpsRecord{
		ID: "io-2-child", Repo: "/repo", Branch: "2-child",
		CleanupAbandonFailure: &model.IssueOpsCleanupAbandonFailure{Step: "applying"},
	})
	if _, err := newStarter(store).Start(context.Background(), startRequest(), nil); err == nil || !strings.Contains(err.Error(), "cleanup abandon apply is in progress") {
		t.Fatalf("err=%v", err)
	}
}

func TestChildStartLinkFailureBecomesWarning(t *testing.T) {
	store := newDelegationStore(readyParent())
	req := startRequest()
	req.ChildIssueURL = "not-a-url"
	got, err := newStarter(store).Start(context.Background(), req, nil)
	if err != nil || !got.OK || !strings.Contains(got.ChildLinkWarning, "http(s) URL") {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func doneChild(id, parent string) model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: id, Repo: "/repo", Branch: id, Phase: model.IssueOpsPhaseDone, Delegation: &model.IssueOpsDelegationContract{ParentCycleID: parent}}
}

func indexedParent(ids ...string) model.IssueOpsRecord {
	p := readyParent()
	for _, id := range ids {
		p.ChildCycles = append(p.ChildCycles, model.IssueOpsChildCycleRef{CycleID: id, Branch: id})
	}
	return p
}

func TestValidatorAcceptRecordsVerdictOnParentIndex(t *testing.T) {
	store := newDelegationStore(indexedParent("c1"), doneChild("c1", "parent"))
	v := Validator{Records: store, Authority: authority(), Now: fixedNow}
	got, err := v.Accept(context.Background(), " parent ", " c1 ", []string{" ok ", ""}, nil)
	if err != nil || !got.OK || got.ParentID != "parent" || got.ChildID != "c1" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	ref := store.records["parent"].ChildCycles[0]
	if ref.ValidationVerdict != "accepted" || !reflect.DeepEqual(ref.ValidationEvidence, []string{"ok"}) || ref.ValidatedAt != fixedStamp {
		t.Fatalf("ref=%+v", ref)
	}
	if got.ParentRef.ValidationVerdict != "accepted" || store.records["parent"].UpdatedAt != fixedStamp {
		t.Fatalf("result ref=%+v", got.ParentRef)
	}
}

func TestValidatorRejectAndDropRequireReasonAndOnlyRejectKeepsEvidence(t *testing.T) {
	child := doneChild("c1", "parent")
	child.Phase = model.IssueOpsPhaseImplement
	store := newDelegationStore(indexedParent("c1"), child)
	v := Validator{Records: store, Authority: authority(), Now: fixedNow}
	if _, err := v.Reject(context.Background(), "parent", "c1", "short", nil, nil); err == nil || !strings.Contains(err.Error(), "at least 10") {
		t.Fatalf("err=%v", err)
	}
	got, err := v.Reject(context.Background(), "parent", "c1", " does not meet criteria ", []string{"log"}, nil)
	if err != nil || got.ParentRef.ValidationVerdict != "rejected" || got.ParentRef.ValidationReason != "does not meet criteria" || !reflect.DeepEqual(got.ParentRef.ValidationEvidence, []string{"log"}) {
		t.Fatalf("reject got=%+v err=%v", got, err)
	}
	got, err = v.Drop(context.Background(), "parent", "c1", "no longer needed", nil)
	if err != nil || got.ParentRef.ValidationVerdict != "dropped" || len(got.ParentRef.ValidationEvidence) != 0 {
		t.Fatalf("drop got=%+v err=%v", got, err)
	}
}

func TestValidatorRefusesInvalidInputsAndMismatches(t *testing.T) {
	notDone := doneChild("c2", "parent")
	notDone.Phase = model.IssueOpsPhaseImplement
	store := newDelegationStore(indexedParent("c1", "c2"), doneChild("c1", "other"), notDone)
	v := Validator{Records: store, Authority: authority(), Now: fixedNow}
	cases := []struct {
		name, parent, child string
		evidence            []string
		want                string
	}{
		{"no evidence", "parent", "c1", nil, "validation_evidence is required"},
		{"no parent", "", "c1", []string{"e"}, "parent_id is required"},
		{"no child", "parent", " ", []string{"e"}, "child_id is required"},
		{"foreign child", "parent", "c1", []string{"e"}, "child_parent_mismatch: c1"},
		{"child not done", "parent", "c2", []string{"e"}, "child_not_done: c2"},
		{"unknown child unindexed", "parent", "ghost", []string{"e"}, "child_not_indexed: ghost"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := v.Accept(context.Background(), tc.parent, tc.child, tc.evidence, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) || got.OK {
				t.Fatalf("got=%+v err=%v", got, err)
			}
		})
	}
	if store.saves != 0 {
		t.Fatalf("saves=%d", store.saves)
	}
}

func TestValidatorAcceptsArchivedChildFromIndexButRejectNeedsLiveChild(t *testing.T) {
	store := newDelegationStore(indexedParent("gone"))
	v := Validator{Records: store, Authority: authority(), Now: fixedNow}
	got, err := v.Drop(context.Background(), "parent", "gone", "archived already", nil)
	if err != nil || got.ParentRef.CycleID != "gone" || got.ParentRef.ValidationVerdict != "dropped" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if _, err := v.Reject(context.Background(), "parent", "gone", "cannot reject archived", nil, nil); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v", err)
	}
}

func TestValidatorDetectsChildReappearingDuringArchivedVerdict(t *testing.T) {
	store := newDelegationStore(indexedParent("c1"))
	reappear := &reappearingStore{delegationStore: store, child: doneChild("c1", "parent")}
	v := Validator{Records: reappear, Authority: authority(), Now: fixedNow}
	if _, err := v.Drop(context.Background(), "parent", "c1", "archived already", nil); err == nil || !strings.Contains(err.Error(), "child_reappeared: c1") {
		t.Fatalf("err=%v", err)
	}
	if store.saves != 0 {
		t.Fatalf("saves=%d", store.saves)
	}
}

// reappearingStore hides the child on its first load and exposes it afterwards.
type reappearingStore struct {
	*delegationStore
	child model.IssueOpsRecord
	loads int
}

func (s *reappearingStore) Load(id string) (model.IssueOpsRecord, error) {
	if id != s.child.ID {
		return s.delegationStore.Load(id)
	}
	s.loads++
	if s.loads == 1 {
		return model.IssueOpsRecord{}, fs.ErrNotExist
	}
	return s.child, nil
}

func TestStatusReportsChildrenAndOrphansWithoutWriting(t *testing.T) {
	parent := indexedParent("c1", "lost")
	store := newDelegationStore(parent, doneChild("c1", "parent"), doneChild("unindexed", "parent"), doneChild("foreign", "other"))
	svc := StatusService{Records: store, Authority: authority(), Now: fixedNow}
	got, err := svc.Status(context.Background(), " parent ", false, nil)
	if err != nil || !got.OK || got.ParentID != "parent" || got.Repaired || store.saves != 0 {
		t.Fatalf("got=%+v err=%v saves=%d", got, err, store.saves)
	}
	if !reflect.DeepEqual(got.Orphaned, []string{"lost"}) || len(got.Children) != 3 {
		t.Fatalf("orphaned=%v children=%+v", got.Orphaned, got.Children)
	}
}

func TestStatusRepairAppendsUnindexedChildrenOnce(t *testing.T) {
	store := newDelegationStore(indexedParent("c1"), doneChild("c1", "parent"), doneChild("b", "parent"), doneChild("a", "parent"))
	svc := StatusService{Records: store, Authority: authority(), Now: fixedNow}
	got, err := svc.Status(context.Background(), "parent", true, nil)
	if err != nil || !got.Repaired || !reflect.DeepEqual(got.RepairAppended, []string{"a", "b"}) {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	saved := store.records["parent"]
	if len(saved.ChildCycles) != 3 || saved.UpdatedAt != fixedStamp || store.saves != 1 {
		t.Fatalf("saved=%+v saves=%d", saved.ChildCycles, store.saves)
	}
	got, err = svc.Status(context.Background(), "parent", true, nil)
	if err != nil || got.Repaired || len(got.RepairAppended) != 0 || store.saves != 1 {
		t.Fatalf("second repair got=%+v err=%v saves=%d", got, err, store.saves)
	}
}

func TestStatusErrors(t *testing.T) {
	svc := StatusService{Records: newDelegationStore(), Authority: authority(), Now: fixedNow}
	if got, err := svc.Status(context.Background(), "  ", false, nil); err == nil || got.OK || !strings.Contains(err.Error(), "parent_id is required") {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if got, err := svc.Status(context.Background(), "nope", false, nil); !errors.Is(err, fs.ErrNotExist) || got.ParentID != "nope" || got.OK {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	store := newDelegationStore(readyParent())
	store.scanErr = errors.New("scan broke")
	svc.Records = store
	if got, err := svc.Status(context.Background(), "parent", false, nil); err == nil || got.OK || got.ParentID != "parent" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	store = newDelegationStore(readyParent(), doneChild("x", "parent"))
	store.saveErr = errors.New("write broke")
	svc.Records = store
	got, err := svc.Status(context.Background(), "parent", true, nil)
	if err == nil || !strings.Contains(err.Error(), "write broke") || got.Repaired || len(got.RepairAppended) != 0 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

// create-child links the child issue first; child start with the same URL
// finds the link in place, which is success, not a warning.
func TestChildStartAlreadyLinkedChildIsNotAWarning(t *testing.T) {
	parent := readyParent()
	parent.IssueURL = "https://github.com/o/r/issues/1"
	parent.IssueLinks = []model.IssueOpsIssueLink{{Type: "child", URL: "https://github.com/o/r/issues/2"}}
	req := startRequest()
	req.ChildIssueURL = "https://github.com/o/r/issues/2"
	got, err := newStarter(newDelegationStore(parent)).Start(context.Background(), req, nil)
	if err != nil || !got.OK || got.ChildLinkWarning != "" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
