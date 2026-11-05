package issueopsremote

import "testing"

func TestRenderScoreSummaryKeepsGroupsAndNamePriority(t *testing.T) {
	result := ScoringResult{
		Threshold: 0.7,
		SelectedRelatedIssues: []ScoredItem{
			{Name: " name ", ID: "id", Title: "title", URL: "url", Score: 0.91},
			{Name: " ", ID: " id ", Title: "title", Score: 0.82},
			{Title: " title ", URL: "url", Score: 0.73},
			{URL: " url ", Score: 0.64},
			{Score: 0.55},
		},
		RejectedRelatedIssues: []ScoredItem{{ID: "rejected", Score: 0.2}},
		SelectedLabels:        []ScoredItem{{Name: "bug", Score: 0.9}},
		RejectedLabels:        []ScoredItem{{Name: "docs", Score: 0.1}},
	}
	const want = "threshold 0.70\n선택 관련 이슈: name(0.91), id(0.82), title(0.73), url(0.64), unknown(0.55)\n거절 관련 이슈: rejected(0.20)\n선택 라벨: bug(0.90)\n거절 라벨: docs(0.10)"
	if got := RenderScoreSummary(result); got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
	if got := RenderScoreSummary(ScoringResult{}); got != "threshold 0.00" {
		t.Fatalf("empty summary = %q", got)
	}
}
