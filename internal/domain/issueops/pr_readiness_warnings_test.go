package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestPRReadinessWarningsPreservesDecisionAndLinkSignals(t *testing.T) {
	if got := PRReadinessWarnings(model.IssueOpsRecord{}); !reflect.DeepEqual(got, []string{"no_decision_records", "no_issue_graph_links"}) {
		t.Fatalf("empty record warnings=%v", got)
	}
	record := model.IssueOpsRecord{IssueLinks: []model.IssueOpsIssueLink{{Type: "child"}}}
	if got := PRReadinessWarnings(record); !reflect.DeepEqual(got, []string{"no_decision_records"}) {
		t.Fatalf("child link warnings=%v", got)
	}
	record.Decisions = []model.IssueOpsDecision{{Title: "decision"}}
	if got := PRReadinessWarnings(record); len(got) != 0 {
		t.Fatalf("decision and link warnings=%v", got)
	}
}
