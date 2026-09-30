package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestPublicationDefaultsPreserveExplicitBaseAndProviderPrecedence(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-defaults", Branch: " local ", IssueURL: "https://github.com/acme/repo/issues/1", BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "gitlab", BaseBranch: "release"}}
	got, err := ResolvePublicationDefaults(record, model.RemotePullRequestRequest{})
	if err != nil || got.ID != record.ID || got.Provider != "gitlab" || got.Head != "local" || got.Base != "release" {
		t.Fatalf("defaults=%+v err=%v", got, err)
	}
	got, err = ResolvePublicationDefaults(record, model.RemotePullRequestRequest{Provider: " github ", Head: " feature ", Base: " "})
	if err != nil || got.Provider != "github" || got.Head != "feature" || got.Base != " " {
		t.Fatalf("explicit values=%+v err=%v", got, err)
	}
	if _, err := ResolvePublicationDefaults(model.IssueOpsRecord{}, model.RemotePullRequestRequest{}); err == nil || !strings.Contains(err.Error(), "ensure issue_url is set") {
		t.Fatalf("missing provider=%v", err)
	}
	if got, err := ResolveBodySyncProvider(record, " github "); err != nil || got != "github" {
		t.Fatalf("body sync override=%s err=%v", got, err)
	}
	if _, err := ResolveBodySyncProvider(model.IssueOpsRecord{}, ""); err == nil || !strings.Contains(err.Error(), "or pass --provider github|gitlab") {
		t.Fatalf("body sync missing provider=%v", err)
	}
}
