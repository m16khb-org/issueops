package remote

import (
	"strings"
	"testing"
)

func TestValidateCreateAuthorityConfirmRequiresMatchingNonZeroGeneration(t *testing.T) {
	base := CreateAuthority{Provider: "github", PhasePR: true, Confirm: true, Execution: true}

	zero := base
	if _, _, err := ValidateCreateAuthority(zero); err == nil || !strings.Contains(err.Error(), "stale lease generation: current=0 expected=0") {
		t.Fatalf("zero generations must be stale, err=%v", err)
	}

	matching := base
	matching.Generation, matching.ExpectedGeneration = 3, 3
	if _, _, err := ValidateCreateAuthority(matching); err != nil {
		t.Fatalf("matching generation rejected: %v", err)
	}
}

func TestPrepareCreateRequestSizeLimitsAreInclusive(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider string
		title    string
		body     string
		wantErr  bool
	}{
		{"title at limit", "github", strings.Repeat("가", 255), "Body", false},
		{"title over limit", "github", strings.Repeat("t", 256), "Body", true},
		{"github body at limit", "github", "Title", strings.Repeat("가", 65536-128), false},
		{"github body over limit", "github", "Title", strings.Repeat("가", 65536-127), true},
		{"gitlab body at limit", "gitlab", "Title", strings.Repeat("b", 1<<20-128), false},
		{"gitlab body over limit", "gitlab", "Title", strings.Repeat("b", 1<<20-127), true},
		{"body over byte cap", "gitlab", "Title", strings.Repeat("가", 400000), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authority, request := createRequestFixture()
			if tc.provider == "gitlab" {
				authority.Provider, authority.IssueURL, request.Provider = "gitlab", "https://gitlab.com/team/repo/-/issues/42", "gitlab"
			}
			request.Title, request.Body = tc.title, tc.body
			_, err := PrepareCreateRequest(authority, request, false)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestIssueNumberAcceptsEveryDigit(t *testing.T) {
	if got := IssueNumber("https://github.com/a/b/issues/0123456789"); got != "0123456789" {
		t.Fatalf("IssueNumber=%q", got)
	}
}

func TestValidProjectKeyAcceptsHostWithSingleSegment(t *testing.T) {
	if !ValidProjectKey("github.com/acme") {
		t.Fatal("host plus one path segment must be a valid project key")
	}
}

func TestProviderFromRemoteURLsHostExtractionShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
		want string
	}{
		{"ssh url with userinfo and port", "ssh://git@github.com:22/acme/repo", "github"},
		{"https url with port", "https://github.com:8443/acme/repo", "github"},
		{"https url with credentials", "https://user:token@gitlab.com/acme/repo.git", "gitlab"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ProviderFromRemoteURLs([]string{tc.url})
			if err != nil || got != tc.want {
				t.Fatalf("provider=%q err=%v want %q", got, err, tc.want)
			}
		})
	}
}

func TestScoreIssueOpsRemoteCandidatesThresholdOneIsAccepted(t *testing.T) {
	one := 1.0
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Threshold:       1,
		Issue:           IssueOpsRemoteArtifact{Title: "login crash"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{{ID: "a", Score: &one}},
	})
	if err != nil || len(res.SelectedRelatedIssues) != 1 {
		t.Fatalf("threshold 1 must be valid: res=%+v err=%v", res, err)
	}
}

func TestScoreIssueOpsRemoteCandidatesScoreEqualToThresholdIsSelected(t *testing.T) {
	half := 0.5
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Threshold:       0.5,
		Issue:           IssueOpsRemoteArtifact{Title: "login crash"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{{ID: "a", Score: &half}},
		LabelCandidates: []IssueOpsRemoteLabelCandidate{{Name: "bug", Score: &half}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.SelectedRelatedIssues) != 1 || len(res.SelectedLabels) != 1 || len(res.RejectedRelatedIssues) != 0 || len(res.RejectedLabels) != 0 {
		t.Fatalf("score == threshold must be selected: %+v", res)
	}
}

func TestScoreIssueOpsRemoteCandidatesReportsReadOnlyJoinContract(t *testing.T) {
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{Issue: IssueOpsRemoteArtifact{Title: "login crash"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ReadOnly || res.ExecutionClass != "background_join" || res.JoinBefore != "remote_artifact_write" {
		t.Fatalf("join contract=%+v", res)
	}
}

func TestScoreIssueOpsRemoteCandidatesLabelOverlapAddsToIssueScore(t *testing.T) {
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Threshold: 0.95,
		Issue:     IssueOpsRemoteArtifact{Title: "alpha beta gamma delta"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{
			{ID: "full", Title: "alpha", Labels: []string{"beta"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// overlap=1, title_overlap=1, label_overlap=1: 0.55+0.30+0.15 reaches 1.
	if len(res.SelectedRelatedIssues) != 1 || res.SelectedRelatedIssues[0].Score != 1 {
		t.Fatalf("selected=%+v rejected=%+v", res.SelectedRelatedIssues, res.RejectedRelatedIssues)
	}
}

func TestScoreIssueOpsRemoteCandidatesTiedItemsKeepInputOrder(t *testing.T) {
	score := 0.8
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Threshold: 0.5,
		Issue:     IssueOpsRemoteArtifact{Title: "login crash"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{
			{ID: "same", Title: "first", Score: &score},
			{ID: "same", Title: "second", Score: &score},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := res.SelectedRelatedIssues
	if len(got) != 2 || got[0].Title != "first" || got[1].Title != "second" {
		t.Fatalf("equal-key items must keep input order: %+v", got)
	}
}
