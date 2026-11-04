package issueopscleanup_test

import (
	"context"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"
	"strings"
	"sync"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestCloseChildrenRequiresMergeEvidence(t *testing.T) {
	store := newCloseChildrenStoreForTest(model.Record{
		ID:       "io-close-children",
		IssueURL: "https://github.com/acme/repo/issues/1",
		IssueLinks: []model.IssueLink{{
			Type:     "child",
			URL:      "https://github.com/acme/repo/issues/2",
			Provider: "github",
		}},
	})

	result, err := ByID(store.store(), t.TempDir(), "io-close-children", model.CloseChildrenRequest{Merged: false})
	if err == nil || !strings.Contains(err.Error(), "merge evidence") {
		t.Fatalf("expected merge evidence failure, got result=%+v err=%v", result, err)
	}
	if len(store.provider.calls) != 0 {
		t.Fatalf("provider must not be called without merge evidence: %+v", store.provider.calls)
	}
}

func TestCloseChildrenDryRunDoesNotMutateState(t *testing.T) {
	record := model.Record{
		RemoteArtifact: &model.RemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/100"},
		ID:             "io-close-children",
		Repo:           "/repo",
		IssueURL:       "https://github.com/acme/repo/issues/1",
		IssueLinks: []model.IssueLink{{
			Type:     "child",
			URL:      "https://github.com/acme/repo/issues/2",
			Provider: "github",
		}},
	}
	store := newCloseChildrenStoreForTest(record)

	result, err := ByID(store.store(), t.TempDir(), record.ID, model.CloseChildrenRequest{Merged: true, Confirm: false})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.DryRun || result.ClosedCount != 0 || len(result.Children) != 1 {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if len(store.provider.calls) != 1 || store.provider.calls[0].Confirm {
		t.Fatalf("dry-run should call provider preview only: %+v", store.provider.calls)
	}
	if got := store.records[record.ID].IssueLinks[0].CloseVerifiedAt; got != "" {
		t.Fatalf("dry-run mutated close evidence: %+v", store.records[record.ID].IssueLinks[0])
	}
}

func TestCloseChildrenConfirmRecordsVerifiedEvidence(t *testing.T) {
	record := model.Record{
		RemoteArtifact: &model.RemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/100"},
		ID:             "io-close-children",
		Repo:           "/repo",
		IssueURL:       "https://github.com/acme/repo/issues/1",
		IssueLinks: []model.IssueLink{{
			Type:      "child",
			URL:       "https://github.com/acme/repo/issues/2",
			Provider:  "github",
			CreatedAt: "2026-06-17T00:00:00Z",
		}},
	}
	store := newCloseChildrenStoreForTest(record)
	store.provider.result = port.IssueProviderCloseChildResult{
		OK:                true,
		Provider:          "github",
		ChildURL:          "https://github.com/acme/repo/issues/2",
		HierarchyVerified: true,
		Closed:            true,
		State:             "closed",
	}

	result, err := ByID(store.store(), t.TempDir(), record.ID, model.CloseChildrenRequest{Merged: true, Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.DryRun || result.ClosedCount != 1 {
		t.Fatalf("unexpected confirm result: %+v", result)
	}
	link := store.records[record.ID].IssueLinks[0]
	if link.ClosedAt == "" || link.CloseVerifiedAt == "" || link.CloseReason != "completed" {
		t.Fatalf("verified close evidence not recorded: %+v", link)
	}
}

func TestCloseChildrenRunsIndependentProviderCallsConcurrently(t *testing.T) {
	record := model.Record{
		RemoteArtifact: &model.RemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/100"},
		ID:             "io-close-children",
		Repo:           "/repo",
		IssueURL:       "https://github.com/acme/repo/issues/1",
		IssueLinks: []model.IssueLink{
			{Type: "child", URL: "https://github.com/acme/repo/issues/2", Provider: "github"},
			{Type: "child", URL: "https://github.com/acme/repo/issues/3", Provider: "github"},
		},
	}
	provider := &blockingCloseChildProvider{
		fakeCloseChildProvider: &fakeCloseChildProvider{},
		started:                make(chan string, 2),
		release:                make(chan struct{}),
	}
	released := false
	defer func() {
		if !released {
			close(provider.release)
		}
	}()
	store := Store{
		Read: func(string, string) (model.Record, error) {
			return record, nil
		},
		TouchWrite: func(_ string, record model.Record) (model.Record, error) {
			return record, nil
		},
		Provider: func(string) (port.IssueProvider, error) {
			return provider, nil
		},
	}
	result := make(chan error, 1)
	go func() {
		_, err := ByID(store, t.TempDir(), record.ID, model.CloseChildrenRequest{Merged: true, Confirm: true})
		result <- err
	}()

	<-provider.started
	select {
	case <-provider.started:
	case <-time.After(250 * time.Millisecond):
		close(provider.release)
		released = true
		t.Fatal("second child close did not start while the first was in flight")
	}
	close(provider.release)
	released = true
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

type closeChildrenStoreForTest struct {
	records  map[string]model.Record
	provider *fakeCloseChildProvider
}

func newCloseChildrenStoreForTest(records ...model.Record) *closeChildrenStoreForTest {
	store := &closeChildrenStoreForTest{
		records:  map[string]model.Record{},
		provider: &fakeCloseChildProvider{result: port.IssueProviderCloseChildResult{OK: true, Provider: "github", HierarchyVerified: true, Closed: true, State: "closed"}},
	}
	for _, record := range records {
		store.records[record.ID] = record
	}
	return store
}

func (s *closeChildrenStoreForTest) store() Store {
	return Store{
		Read:       s.read,
		TouchWrite: s.touchWrite,
		Provider: func(string) (port.IssueProvider, error) {
			return s.provider, nil
		},
	}
}

func (s *closeChildrenStoreForTest) read(_ string, id string) (model.Record, error) {
	record, ok := s.records[id]
	if !ok {
		return model.Record{}, fmt.Errorf("missing record")
	}
	return record, nil
}

func (s *closeChildrenStoreForTest) touchWrite(_ string, record model.Record) (model.Record, error) {
	s.records[record.ID] = record
	return record, nil
}

type fakeCloseChildProvider struct {
	mu     sync.Mutex
	result port.IssueProviderCloseChildResult
	calls  []port.IssueProviderCloseChildRequest
	// previewState는 preview 경로가 관측한 자식의 원격 상태다. 빈 값은 관측
	// 실패를 뜻한다(gh/glab 부재 등).
	previewState      string
	previewStateByURL map[string]string
}

func (p *fakeCloseChildProvider) Name() string { return "github" }

func (p *fakeCloseChildProvider) CreateIssue(context.Context, port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, nil
}

func (p *fakeCloseChildProvider) CreatePullRequest(context.Context, port.IssueProviderCreatePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error) {
	return executionissue.IssueProviderCreatePullRequestResult{}, nil
}

func (p *fakeCloseChildProvider) CreateChild(context.Context, port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, nil
}

func (p *fakeCloseChildProvider) UpdateIssueBodySection(context.Context, port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{}, nil
}

func (p *fakeCloseChildProvider) CloseIssue(context.Context, port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	return port.IssueProviderCloseIssueResult{}, nil
}

func (p *fakeCloseChildProvider) CloseChild(_ context.Context, req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, req)
	result := p.result
	result.ChildURL = req.ChildURL
	if !req.Confirm {
		result.Closed = false
		result.Preview = "[dry-run] close " + req.ChildURL
		state := p.previewState
		if override, ok := p.previewStateByURL[req.ChildURL]; ok {
			state = override
		}
		result.State = state
		result.AlreadyClosed = strings.EqualFold(state, "closed")
		result.HierarchyVerified = state != ""
	}
	return result, nil
}

type blockingCloseChildProvider struct {
	*fakeCloseChildProvider
	started chan string
	release chan struct{}
}

func (provider *blockingCloseChildProvider) CloseChild(_ context.Context, req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	provider.started <- req.ChildURL
	<-provider.release
	return port.IssueProviderCloseChildResult{
		OK:                true,
		Provider:          "github",
		ChildURL:          req.ChildURL,
		HierarchyVerified: true,
		Closed:            true,
		State:             "closed",
	}, nil
}
