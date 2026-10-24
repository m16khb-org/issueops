package remote

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// githubDefaultLabelCandidates는 gh label list의 기본 라벨 설명을 그대로 쓴다.
func githubDefaultLabelCandidates() []IssueOpsRemoteLabelCandidate {
	return []IssueOpsRemoteLabelCandidate{
		{Name: "bug", Description: "Something isn't working"},
		{Name: "enhancement", Description: "New feature or request"},
		{Name: "documentation", Description: "Improvements or additions to documentation"},
		{Name: "question", Description: "Further information is requested"},
	}
}

func TestScoreIssueOpsRemoteLabelQualityTable(t *testing.T) {
	type qualityCase struct {
		Issue        int      `json:"issue"`
		Note         string   `json:"note"`
		Title        string   `json:"title"`
		Body         string   `json:"body"`
		WantSelected []string `json:"want_selected"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "label_quality_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []qualityCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("quality table is empty")
	}
	for _, tc := range cases {
		if tc.Issue >= 232 {
			t.Fatalf("case #%d is outside the development set (numbers below 232)", tc.Issue)
		}
		res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
			Issue:           IssueOpsRemoteArtifact{Title: tc.Title, Body: tc.Body},
			LabelCandidates: githubDefaultLabelCandidates(),
		})
		if err != nil {
			t.Fatalf("#%d: %v", tc.Issue, err)
		}
		got := []string{}
		for _, item := range res.SelectedLabels {
			got = append(got, item.Name)
		}
		if !reflect.DeepEqual(got, tc.WantSelected) {
			t.Errorf("#%d selected labels = %v, want %v (%s): %+v", tc.Issue, got, tc.WantSelected, tc.Note, res.SelectedLabels)
		}
	}
}

func TestScoreIssueOpsRemoteLabelWithoutKeywordRuleKeepsOverlapScore(t *testing.T) {
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Issue:           IssueOpsRemoteArtifact{Title: "Further information is requested about login", Body: "오류가 재현되지 않는다 오류 결함 버그"},
		LabelCandidates: githubDefaultLabelCandidates(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var question *IssueOpsRemoteScoredItem
	for i := range res.RejectedLabels {
		if res.RejectedLabels[i].Name == "question" {
			question = &res.RejectedLabels[i]
		}
	}
	if question == nil {
		t.Fatalf("question label was not rejected: selected=%+v rejected=%+v", res.SelectedLabels, res.RejectedLabels)
	}
	if want := []string{"token_overlap=0.75", "type_heuristic=0.00"}; !reflect.DeepEqual(question.Evidence, want) {
		t.Fatalf("question evidence = %v, want %v", question.Evidence, want)
	}
	if math.Abs(question.Score-0.65*0.75) > 1e-9 {
		t.Fatalf("question score = %v, want 0.65*overlap", question.Score)
	}
}

// One matched type token is below the evidence floor, so the scorer abstains;
// the rejection says so instead of reading like a failed score.
func TestScoreIssueOpsRemoteLabelNamesAbstentionOnThinEvidence(t *testing.T) {
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Issue:           IssueOpsRemoteArtifact{Title: "hello.sh에 --upper 옵션 추가", Body: "--upper 옵션을 주면 HELLO를 대문자로 출력한다."},
		LabelCandidates: githubDefaultLabelCandidates(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range res.RejectedLabels {
		if item.Name == "enhancement" {
			if item.Score != 0 || item.RejectReason != "abstained: 1 type-evidence token(s), scoring needs 2; choose the label manually" {
				t.Fatalf("enhancement = %+v", item)
			}
			return
		}
	}
	t.Fatalf("enhancement not rejected: %+v", res)
}
