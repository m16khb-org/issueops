package issueopsapp

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"

	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
)

func TestIssueLinkCompositionPreservesConcurrentRelationsAndRefusedState(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := core.IssueOpsStateRoot()
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: makeGitRepoForContract(t), Branch: "63-links"})
	if err != nil {
		t.Fatal(err)
	}
	service := newIssueLinker(root)
	record, err = service.Issue(context.Background(), record.ID, " https://github.com/acme/repo/issues/63 ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.IssueURL != "https://github.com/acme/repo/issues/63" || record.Phase != model.IssueOpsPhaseProblem {
		t.Fatalf("unexpected link result: %+v", record)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for n := 0; n < 12; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := service.Related(context.Background(), record.ID, "blocks", fmt.Sprintf("https://github.com/acme/repo/issues/%d", 100+n), "related", nil)
			errs <- err
		}(n)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	stored, err := core.ReadIssueOps(root, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, link := range stored.IssueLinks {
		if seen[link.URL] {
			t.Fatal("duplicate relation")
		}
		seen[link.URL] = true
	}
	if len(seen) != 12 {
		t.Fatalf("concurrent links lost: %d", len(seen))
	}
	for _, call := range []func() (model.IssueOpsRecord, error){
		func() (model.IssueOpsRecord, error) {
			return service.Related(context.Background(), record.ID, "blocks", "https://github.com/acme/repo/issues/100", "duplicate", nil)
		},
		func() (model.IssueOpsRecord, error) {
			return service.Child(context.Background(), record.ID, "https://github.com/acme/other/issues/1", "wrong project", nil)
		},
		func() (model.IssueOpsRecord, error) {
			return service.Issue(context.Background(), record.ID, "not-a-url", nil)
		},
	} {
		if _, err := call(); err == nil {
			t.Fatal("invalid link accepted")
		}
		after, err := core.ReadIssueOps(root, record.ID)
		if err != nil || !reflect.DeepEqual(after, stored) {
			t.Fatalf("refused link changed persistent state: %v", err)
		}
	}
}
