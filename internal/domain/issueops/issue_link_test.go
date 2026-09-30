package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestAppendIssueRelationDoesNotMutatePreviousRecordStorage(t *testing.T) {
	backing := make([]model.IssueOpsIssueLink, 2, 4)
	backing[0] = model.IssueOpsIssueLink{Type: "blocks", URL: "https://github.com/acme/repo/issues/1"}
	backing[1] = model.IssueOpsIssueLink{Title: "other snapshot"}
	before := model.IssueOpsRecord{IssueLinks: backing[:1], UpdatedAt: "before"}
	got, err := AppendIssueRelation(before, model.IssueOpsIssueLink{Type: "depends-on", URL: backing[0].URL, Title: " dependency "}, "now")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.IssueLinks) != 2 || got.IssueLinks[1].Title != "dependency" || got.IssueLinks[1].CreatedAt != "now" || got.UpdatedAt != "now" {
		t.Fatalf("unexpected projection: %+v", got)
	}
	if backing[1].Title != "other snapshot" || before.UpdatedAt != "before" {
		t.Fatal("projection mutated input")
	}
}

func TestRelatedLinkTypeRejectsTopologyEdges(t *testing.T) {
	for _, kind := range []string{"depends-on", "blocks", "supersedes", "follows-up", "duplicates", "splits-from", "implements"} {
		if err := ValidateRelatedLinkType(kind); err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"", "parent", "child", "related", "unknown"} {
		if err := ValidateRelatedLinkType(kind); err == nil {
			t.Fatalf("accepted %q", kind)
		}
	}
}
