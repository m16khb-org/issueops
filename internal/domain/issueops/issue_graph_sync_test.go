package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestIssueGraphSyncEligibilityBeforePublication(t *testing.T) {
	for _, tc := range []struct {
		name, url, provider, want string
		confirm, links            bool
	}{
		{name: "preview"},
		{name: "missing issue before empty graph", confirm: true, want: "no issue_url"},
		{name: "empty graph before provider", url: "https://example.test/issue/1", confirm: true},
		{name: "unknown provider", url: "https://example.test/issue/1", confirm: true, links: true, want: "cannot determine provider"},
		{name: "unsupported provider", url: "https://example.test/issue/1", provider: "other", confirm: true, links: true, want: "not supported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := model.IssueOpsRecord{IssueURL: tc.url}
			if tc.provider != "" {
				record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: tc.provider}
			}
			if tc.links {
				record.IssueLinks = []model.IssueOpsIssueLink{{Type: "blocks", URL: "https://example.test/issue/2"}}
			}
			plan, err := PlanIssueGraphSync(record, tc.confirm)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("plan=%+v err=%v want=%s", plan, err, tc.want)
				}
				return
			}
			if err != nil || (!tc.confirm && !plan.Preview) || (tc.confirm && !plan.Noop) || plan.Body != "" {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
		})
	}
}

func TestIssueGraphSyncRendersRelationships(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-graph", IssueURL: " https://github.com/acme/repo/issues/1 "}
	for _, kind := range []string{"depends-on", "blocks", "supersedes", "follows-up", "duplicates", "splits-from", "implements", "custom"} {
		record.IssueLinks = append(record.IssueLinks, model.IssueOpsIssueLink{Type: kind, URL: "https://example.test/2"})
	}
	plan, err := PlanIssueGraphSync(record, true)
	want := "## 관련 이슈\n\n- **선행 이슈**: https://example.test/2\n- **이 이슈가 막는 이슈**: https://example.test/2\n- **대체하는 이슈**: https://example.test/2\n- **후속 이슈**: https://example.test/2\n- **중복 이슈**: https://example.test/2\n- **나뉘어 나온 원래 이슈**: https://example.test/2\n- **구현하는 이슈**: https://example.test/2\n- **custom**: https://example.test/2\n"
	if err != nil || plan.Body != want || plan.LinkCount != 8 || plan.Provider != "github" || plan.URL != "https://github.com/acme/repo/issues/1" {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}
