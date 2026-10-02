package issueopscleanup_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type childCloseFunction struct {
	port.IssueProvider
	close func(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error)
}

func (p childCloseFunction) CloseChild(req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	return p.close(req)
}

type lockedChildCleanupRecords struct {
	t       *testing.T
	record  model.IssueOpsRecord
	locked  bool
	saves   int
	saveErr error
}

func (r *lockedChildCleanupRecords) WithinLock(ctx context.Context, id string, fn func(context.Context) error) error {
	if r.locked || id != r.record.ID {
		r.t.Fatal("incorrect lock")
	}
	r.locked = true
	defer func() { r.locked = false }()
	return fn(ctx)
}
func (r *lockedChildCleanupRecords) Load(id string) (model.IssueOpsRecord, error) {
	if id != r.record.ID {
		r.t.Fatal("unexpected cycle identity")
	}
	return r.record, nil
}
func (r *lockedChildCleanupRecords) Save(_ context.Context, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	if !r.locked {
		r.t.Fatal("save outside the cycle lock")
	}
	r.saves++
	if r.saveErr != nil {
		return record, r.saveErr
	}
	r.record = record
	return record, nil
}

func TestChildrenCloserDoesNotPersistPartialOrUnverifiedReceipts(t *testing.T) {
	for _, failure := range []string{"provider", "hierarchy", "closed", "save"} {
		t.Run(failure, func(t *testing.T) {
			record := model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/100"}, ID: "parent", IssueURL: "https://github.com/acme/repo/issues/1", IssueLinks: []model.IssueOpsIssueLink{{Type: "child", URL: "https://github.com/acme/repo/issues/2"}, {Type: "child", URL: "https://github.com/acme/repo/issues/3"}}}
			original := append([]model.IssueOpsIssueLink(nil), record.IssueLinks...)
			records := &lockedChildCleanupRecords{t: t, record: record}
			if failure == "save" {
				records.saveErr = errors.New("save refused")
			}
			provider := childCloseFunction{close: func(req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
				result := port.IssueProviderCloseChildResult{Closed: true, HierarchyVerified: true}
				if strings.HasSuffix(req.ChildURL, "/3") {
					switch failure {
					case "provider":
						return result, errors.New("close refused")
					case "hierarchy":
						result.HierarchyVerified = false
					case "closed":
						result.Closed = false
					}
				}
				return result, nil
			}}
			closer := app.ChildrenCloser{VerifyMerged: func(model.IssueOpsRemoteArtifactVerification) error { return nil }, Records: records, Provider: func(string) (port.IssueProvider, error) { return provider, nil }, Now: func() time.Time { return time.Unix(123, 0) }}
			result, err := closer.Close(context.Background(), record.ID, true, true)
			if err == nil || len(result.Children) != 2 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !reflect.DeepEqual(records.record.IssueLinks, original) || !reflect.DeepEqual(record.IssueLinks, original) {
				t.Fatal("failed closure mutated stored or observed record")
			}
			wantSaves := 0
			if failure == "save" {
				wantSaves = 1
			}
			if records.saves != wantSaves {
				t.Fatalf("saves=%d", records.saves)
			}
		})
	}
}

func TestChildrenCloserLimitsConcurrencyAndPreservesResultOrder(t *testing.T) {
	record := model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/100"}, ID: "parent", IssueURL: "https://github.com/acme/repo/issues/1"}
	for _, suffix := range []string{"2", "3", "4", "5", "6"} {
		record.IssueLinks = append(record.IssueLinks, model.IssueOpsIssueLink{Type: "child", URL: "https://github.com/acme/repo/issues/" + suffix})
	}
	records := &lockedChildCleanupRecords{t: t, record: record}
	started := make(chan string, 5)
	release := make(chan struct{})
	defer close(release)
	provider := childCloseFunction{close: func(req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
		started <- req.ChildURL
		<-release
		return port.IssueProviderCloseChildResult{ChildURL: req.ChildURL, HierarchyVerified: true, Closed: true}, nil
	}}
	closer := app.ChildrenCloser{VerifyMerged: func(model.IssueOpsRemoteArtifactVerification) error { return nil }, Records: records, Provider: func(string) (port.IssueProvider, error) { return provider, nil }, Now: func() time.Time { return time.Unix(123, 0) }}
	done := make(chan model.IssueOpsCloseChildrenResult, 1)
	errs := make(chan error, 1)
	go func() {
		result, err := closer.Close(context.Background(), record.ID, true, true)
		done <- result
		errs <- err
	}()
	for i := 0; i < 4; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("four independent provider requests did not start")
		}
	}
	select {
	case <-started:
		t.Fatal("more than four requests in flight")
	case <-time.After(50 * time.Millisecond):
	}
	// Release one slot, then let the final request start. All original workers
	// remain bounded by the same semaphore.
	release <- struct{}{}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("fifth request did not start after a slot was released")
	}
	for i := 0; i < 4; i++ {
		release <- struct{}{}
	}
	result := <-done
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if records.saves != 1 || result.ClosedCount != 5 {
		t.Fatalf("saves=%d result=%+v", records.saves, result)
	}
	for index, child := range result.Children {
		if child.URL != record.IssueLinks[index].URL {
			t.Fatalf("unstable result order: %+v", result.Children)
		}
	}
}
