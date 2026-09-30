package remote

import "testing"

func TestRenderScoreSummaryKeepsGroupsAndNamePriority(t *testing.T) {
	result := IssueOpsRemoteScoringResult{
		Threshold: 0.7,
		SelectedRelatedIssues: []IssueOpsRemoteScoredItem{
			{Name: " name ", ID: "id", Title: "title", URL: "url", Score: 0.91},
			{Name: " ", ID: " id ", Title: "title", Score: 0.82},
			{Title: " title ", URL: "url", Score: 0.73},
			{URL: " url ", Score: 0.64},
			{Score: 0.55},
		},
		RejectedRelatedIssues: []IssueOpsRemoteScoredItem{{ID: "rejected", Score: 0.2}},
		SelectedLabels:        []IssueOpsRemoteScoredItem{{Name: "bug", Score: 0.9}},
		RejectedLabels:        []IssueOpsRemoteScoredItem{{Name: "docs", Score: 0.1}},
	}
	const want = "threshold 0.70\n선택 관련 이슈: name(0.91), id(0.82), title(0.73), url(0.64), unknown(0.55)\n거절 관련 이슈: rejected(0.20)\n선택 라벨: bug(0.90)\n거절 라벨: docs(0.10)"
	if got := RenderScoreSummary(result); got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
	if got := RenderScoreSummary(IssueOpsRemoteScoringResult{}); got != "threshold 0.00" {
		t.Fatalf("empty summary = %q", got)
	}
}
