package issueopsremote

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type recoveryChildProvider struct {
	mu               sync.Mutex
	calls, attaches  int
	snapshots        map[string]port.ChildSnapshot
	fail             error
	entered, release chan struct{}
	search           port.IssueProviderFindIssueCreateCandidatesResult
	store            *testChildStore
}

func (p *recoveryChildProvider) CreateChild(req port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	if !p.store.mu.TryLock() {
		panic("provider called under record span")
	}
	p.store.mu.Unlock()
	if !req.Confirm {
		return port.IssueProviderCreateChildResult{OK: true, Preview: "preview"}, nil
	}
	p.mu.Lock()
	if e, ok := errors.AsType[*port.IssueProviderCreateError](p.fail); ok && !e.Invoked {
		p.mu.Unlock()
		return port.IssueProviderCreateChildResult{}, e
	}
	p.calls++
	url := strings.TrimSuffix(req.ParentIssueURL, "/1") + fmt.Sprintf("/%d", p.calls+1)
	snapshot := port.ChildSnapshot{URL: url, Title: req.Title, Body: req.Body, TypeVerified: true, HierarchyVerified: p.fail == nil, Labels: req.Labels, Assignees: req.Assignees}
	p.snapshots[url] = snapshot
	p.search.Candidates = []port.IssueProviderIssueCreateCandidate{{URL: url, Title: req.Title, Body: req.Body}}
	fail := p.fail
	p.mu.Unlock()
	if p.entered != nil {
		close(p.entered)
		<-p.release
	}
	return port.IssueProviderCreateChildResult{OK: fail == nil, ChildURL: url, HierarchyVerified: fail == nil}, fail
}
func (p *recoveryChildProvider) FindChildCreateCandidates(context.Context, port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error) {
	return p.search, nil
}
func (p *recoveryChildProvider) ReadChild(_ context.Context, _, _, url string) (port.ChildSnapshot, error) {
	return p.snapshots[url], nil
}
func (p *recoveryChildProvider) AttachChild(_ context.Context, _, _, url string) error {
	p.attaches++
	snapshot := p.snapshots[url]
	snapshot.HierarchyVerified = true
	p.snapshots[url] = snapshot
	return nil
}
func childRecoveryFixture(provider string) (*testChildStore, *recoveryChildProvider, ChildCreator, ChildReconciler, ChildCreateCommand) {
	parent := "https://github.com/acme/repo/issues/1"
	if provider == "gitlab" {
		parent = "https://gitlab.example.com/acme/repo/-/work_items/1"
	}
	store := &testChildStore{record: model.IssueOpsRecord{ID: "io-child", Repo: "/repo", Branch: "1-parent", IssueURL: parent, BranchPrepare: &model.IssueOpsBranchPrepare{Branch: "1-parent"}}}
	p := &recoveryChildProvider{store: store, snapshots: map[string]port.ChildSnapshot{}}
	intents := &ChildCreateIntents{Store: store, Authority: testChildAuthority{}, Now: time.Now}
	ids := 0
	creator := ChildCreator{Records: store, Intents: intents, NewOperationID: func() (string, error) { ids++; return fmt.Sprintf("%032x", ids), nil }, Resolve: func(string) (ChildProvider, error) { return p, nil }, Bodies: NewTemplateBodyResolver(nil)}
	reconciler := ChildReconciler{Records: store, Intents: intents, Resolve: func(string) (port.IssueProviderChildCreateRecovery, error) { return p, nil }}
	cmd := ChildCreateCommand{ID: "io-child", Title: "Child", Body: readableChildBody, Labels: []string{"bug"}, Assignees: []string{"owner"}, Confirm: true}
	return store, p, creator, reconciler, cmd
}

func TestChildImplicitIdentitySurvivesCompletedResponseLossAndExplicitDuplicate(t *testing.T) {
	for _, name := range []string{"github", "gitlab"} {
		t.Run(name, func(t *testing.T) {
			store, p, creator, _, cmd := childRecoveryFixture(name)
			first, err := creator.Create(context.Background(), cmd, nil)
			if err != nil {
				t.Fatal(err)
			}
			// Lose the response and reconstruct services from only the durable JSON record.
			creator.Records = &testChildStore{record: store.read()}
			creator.Intents.Store = creator.Records.(*testChildStore)
			replay, err := creator.Create(context.Background(), cmd, nil)
			if err != nil || replay.ChildURL != first.ChildURL || replay.OperationID != first.OperationID || p.calls != 1 {
				t.Fatalf("replay=%+v err=%v creates=%d", replay, err, p.calls)
			}
			cmd.OperationID = strings.Repeat("f", 32)
			second, err := creator.Create(context.Background(), cmd, nil)
			if err != nil || second.ChildURL == first.ChildURL {
				t.Fatalf("explicit=%+v err=%v", second, err)
			}
			restart := &testChildStore{record: creator.Records.(*testChildStore).read()}
			creator.Records = restart
			creator.Intents.Store = restart
			cmd.OperationID = ""
			replay, err = creator.Create(context.Background(), cmd, nil)
			if err != nil || replay.ChildURL != first.ChildURL || replay.OperationID != first.OperationID || p.calls != 2 {
				t.Fatalf("lost implicit identity: %+v err=%v creates=%d", replay, err, p.calls)
			}
			if restart.read().IssueURL != store.read().IssueURL || len(restart.read().IssueLinks) != 2 {
				t.Fatalf("parent/links changed: %+v", restart.read())
			}
			cmd.OperationID = first.OperationID
			cmd.Title = "Different"
			if _, err = creator.Create(context.Background(), cmd, nil); err == nil || p.calls != 2 {
				t.Fatalf("payload drift accepted: %v", err)
			}
		})
	}
}

func TestChildPartialFailurePreviewAndRestartReconcileSameURL(t *testing.T) {
	for _, name := range []string{"github", "gitlab"} {
		for _, failure := range []string{"attach", "receipt", "unknown"} {
			t.Run(name+"/"+failure, func(t *testing.T) {
				store, p, creator, reconciler, cmd := childRecoveryFixture(name)
				if failure == "receipt" {
					store.before = func(r model.IssueOpsRecord) error {
						if r.ChildCreateOperations[0].Status == model.IssueCreateIntentCompleted {
							return errors.New("disk full")
						}
						return nil
					}
				} else {
					p.fail = errors.New("connection lost")
				}
				first, err := creator.Create(context.Background(), cmd, nil)
				if err == nil {
					t.Fatal("expected partial failure")
				}
				if first.ChildURL == "" || first.OperationID == "" || first.RecoveryCommand == "" {
					t.Fatalf("lost recovery: %+v", first)
				}
				if failure == "unknown" {
					store.mu.Lock()
					store.record.ChildCreateOperations[0].Status = model.IssueCreateIntentInvokedUnknown
					store.record.ChildCreateOperations[0].CanonicalURL = ""
					store.mu.Unlock()
				}
				store.before = nil
				retry, err := creator.Create(context.Background(), cmd, nil)
				if err == nil || p.calls != 1 || retry.OperationID != first.OperationID {
					t.Fatalf("retry=%+v err=%v count=%d", retry, err, p.calls)
				}
				cmd.OperationID = strings.Repeat("e", 32)
				if _, err = creator.Create(context.Background(), cmd, nil); err == nil || p.calls != 1 {
					t.Fatal("fresh identity bypassed pending")
				}
				snapshot := store.read()
				preview, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID}, nil)
				if err != nil || !preview.WouldAdopt || p.attaches != 0 || !reflect.DeepEqual(snapshot, store.read()) {
					t.Fatalf("preview wrote or failed %+v %v", preview, err)
				}
				adopted, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID, Confirm: true}, nil)
				if err != nil || adopted.ChildURL != first.ChildURL || p.calls != 1 || len(store.read().IssueLinks) != 1 {
					t.Fatalf("adopt=%+v err=%v count=%d", adopted, err, p.calls)
				}
			})
		}
	}
}

func TestChildConcurrentImplicitCreateIsCASBound(t *testing.T) {
	store, p, creator, _, cmd := childRecoveryFixture("github")
	p.entered, p.release = make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() { _, err := creator.Create(context.Background(), cmd, nil); done <- err }()
	<-p.entered
	second := creator
	second.NewOperationID = func() (string, error) { return strings.Repeat("b", 32), nil }
	_, err := second.Create(context.Background(), cmd, nil)
	if err == nil {
		t.Fatal("concurrent pending create accepted")
	}
	close(p.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if p.calls != 1 || len(store.read().ChildCreateOperations) != 1 {
		t.Fatalf("creates=%d ops=%d", p.calls, len(store.read().ChildCreateOperations))
	}
}

func TestChildReconcileRejectsCandidateAmbiguityAndMetadataDrift(t *testing.T) {
	for _, problem := range []string{"zero", "many", "truncated", "title", "body", "labels", "assignees", "type", "project", "parent"} {
		t.Run(problem, func(t *testing.T) {
			store, p, creator, reconciler, cmd := childRecoveryFixture("github")
			p.fail = errors.New("lost response")
			result, _ := creator.Create(context.Background(), cmd, nil)
			op := store.read().ChildCreateOperations[0]
			snapshot := p.snapshots[result.ChildURL]
			switch problem {
			case "zero", "many", "truncated":
				store.mu.Lock()
				store.record.ChildCreateOperations[0].CanonicalURL = ""
				store.record.ChildCreateOperations[0].Status = model.IssueCreateIntentInvokedUnknown
				store.mu.Unlock()
				if problem == "zero" {
					p.search.Candidates = nil
				}
				if problem == "many" {
					p.search.Candidates = append(p.search.Candidates, p.search.Candidates[0])
				}
				if problem == "truncated" {
					p.search.Truncated = true
				}
			case "title":
				snapshot.Title = "Different"
			case "body":
				snapshot.Body += "edited"
			case "labels":
				snapshot.Labels = nil
			case "assignees":
				snapshot.Assignees = nil
			case "type":
				snapshot.TypeVerified = false
			case "project":
				snapshot.URL = "https://github.com/foreign/repo/issues/2"
			case "parent":
				store.mu.Lock()
				store.record.IssueURL = "https://github.com/acme/repo/issues/9"
				store.mu.Unlock()
			}
			p.snapshots[result.ChildURL] = snapshot
			if _, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: op.OperationID, Confirm: true}, nil); err == nil {
				t.Fatal("unsafe adoption accepted")
			}
			if p.calls != 1 || p.attaches != 0 || len(store.read().IssueLinks) != 0 {
				t.Fatal("unsafe side effect")
			}
		})
	}
}

func TestChildReceiptFencesObservedGeneration(t *testing.T) {
	store, p, creator, _, cmd := childRecoveryFixture("github")
	p.fail = errors.New("lost response")
	result, _ := creator.Create(context.Background(), cmd, nil)
	observed := store.read()
	// The same native actor can hold a later generation; the old read still cannot commit.
	store.mu.Lock()
	store.record.Execution = &model.Execution{Lease: model.WriteLease{Generation: 2}}
	store.mu.Unlock()
	err := creator.Intents.Complete(context.Background(), observed, observed.ChildCreateOperations[0], result.ChildURL, model.IssueOpsActor{}, true)
	if err == nil || len(store.read().IssueLinks) != 0 {
		t.Fatalf("stale observation committed: %v", err)
	}
}

func TestChildFailedRecoveryPersistsDiscoveredURL(t *testing.T) {
	store, p, creator, reconciler, cmd := childRecoveryFixture("github")
	p.fail = errors.New("lost response")
	first, _ := creator.Create(context.Background(), cmd, nil)
	store.mu.Lock()
	store.record.ChildCreateOperations[0].CanonicalURL = ""
	store.record.ChildCreateOperations[0].Status = model.IssueCreateIntentInvokedUnknown
	store.mu.Unlock()
	snapshot := p.snapshots[first.ChildURL]
	snapshot.Labels = nil
	p.snapshots[first.ChildURL] = snapshot
	_, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID, Confirm: true}, nil)
	op := store.read().ChildCreateOperations[0]
	if err == nil || op.CanonicalURL != first.ChildURL || op.Status != model.IssueCreateIntentVerificationFailed || p.calls != 1 {
		t.Fatalf("lost discovered URL: %+v %v", op, err)
	}
}

func TestChildConcurrentNotInvokedRetryUsesSelectedOperationBody(t *testing.T) {
	store, p, first, reconcile, cmd := childRecoveryFixture("github")
	second := first
	entered, release := make(chan struct{}), make(chan struct{})
	second.NewOperationID = func() (string, error) { close(entered); <-release; return strings.Repeat("b", 32), nil }
	type outcome struct {
		result ChildCreateResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() { r, e := second.Create(context.Background(), cmd, nil); done <- outcome{r, e} }()
	<-entered
	p.fail = &port.IssueProviderCreateError{Invoked: false, Err: errors.New("capability unavailable")}
	a, e := first.Create(context.Background(), cmd, nil)
	if e == nil || p.calls != 0 || store.read().ChildCreateOperations[0].Status != model.IssueCreateIntentNotInvoked {
		t.Fatalf("first=%+v err=%v creates=%d", a, e, p.calls)
	}
	p.fail = errors.New("response lost after create")
	close(release)
	b := <-done
	op := store.read().ChildCreateOperations[0]
	snapshot := p.snapshots[b.result.ChildURL]
	if b.err == nil || b.result.OperationID != a.OperationID || p.calls != 1 || !strings.Contains(snapshot.Body, op.Marker) || fmt.Sprintf("%x", sha256.Sum256([]byte(snapshot.Body))) != op.BodySHA256 {
		t.Fatalf("selected=%s marker=%s body=%s result=%+v err=%v creates=%d", op.OperationID, op.Marker, snapshot.Body, b.result, b.err, p.calls)
	}
	adopted, e := reconcile.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: op.OperationID, Confirm: true}, nil)
	if e != nil || adopted.ChildURL != b.result.ChildURL || p.calls != 1 {
		t.Fatalf("adopt=%+v err=%v creates=%d", adopted, e, p.calls)
	}
}
