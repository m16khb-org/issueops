package issueops

import (
	"context"
	model "issueops/internal/contract/issueops"
	linkedbranch "issueops/internal/domain/issueopslinkedbranch"
	"reflect"
	"testing"
)

func TestCleanupLinkedBranchRejectsUnknownRemoteRef(t *testing.T) {
	t.Parallel()

	for _, apply := range []bool{false, true} {
		name := "preview"
		if apply {
			name = "apply"
		}
		t.Run(name, func(t *testing.T) {
			root, record := lbFixture(t)
			d := &lbDeps{nodes: []linkedbranch.Node{{ID: lbOrphanID}}}
			deps := d.build()
			preview, err := CleanupLinkedBranch(context.Background(), root, model.CleanupLinkedBranchRequest{ID: record.ID}, deps)
			if err != nil {
				t.Fatal(err)
			}
			before, err := ReadIssueOps(root, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			deps.Git = func(context.Context, string, ...string) (int, string) { return 128, "remote unavailable" }
			result, err := CleanupLinkedBranch(context.Background(), root, model.CleanupLinkedBranchRequest{ID: record.ID, Apply: apply, Confirm: apply, Fingerprint: preview.Fingerprint}, deps)
			if err == nil || result.OK || result.Fingerprint != "" || result.Deleted || len(d.deleted) != 0 {
				t.Fatalf("unknown remote accepted: result=%+v err=%v deletes=%v", result, err, d.deleted)
			}
			after, readErr := ReadIssueOps(root, record.ID)
			if readErr != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("failed observation mutated record: before=%+v after=%+v err=%v", before, after, readErr)
			}
		})
	}
}

func TestAwaitBranchLinkRetriesUnknownRemoteRef(t *testing.T) {
	t.Parallel()

	root := awaitFixture(t, false)
	d := &awaitDeps{rounds: [][]linkedbranch.Node{healthyNodes(), healthyNodes()}, remote: []string{lbSealedBase, lbSealedBase}}
	deps := d.build()
	git := deps.Git
	calls := 0
	deps.Git = func(ctx context.Context, dir string, args ...string) (int, string) {
		calls++
		if calls == 1 {
			return 128, "remote unavailable"
		}
		return git(ctx, dir, args...)
	}
	result, err := AwaitBranchLink(context.Background(), root, model.AwaitBranchLinkRequest{ID: "io-await1", Timeout: "1m"}, deps)
	if err != nil || !result.Linked || result.Attempts != 2 || len(d.slept) != 1 {
		t.Fatalf("unknown remote treated as mismatch: result=%+v err=%v sleeps=%v", result, err, d.slept)
	}
}
