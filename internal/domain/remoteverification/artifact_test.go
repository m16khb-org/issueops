package remoteverification

import (
	model "issueops/internal/contract/remoteverification"
	"strings"
	"testing"
)

func TestArtifactEvidencePreservesMissingOrderAndRefusalPrecedence(t *testing.T) {
	req := struct {
		URL               string
		Labels, Assignees []string
	}{URL: " url/ ", Labels: []string{" Bug ", "", "Bug", " feature "}, Assignees: []string{"alice"}}
	live := model.Artifact{URL: "url", Labels: []string{"feature"}}
	err := ValidateArtifact(req.URL, req.Labels, req.Assignees, live)
	if err == nil || err.Error() != "remote artifact missing verified label(s):  Bug , Bug" {
		t.Fatalf("missing=%v", err)
	}
	live.Labels = append(live.Labels, "BUG")
	if err := ValidateArtifact(req.URL, req.Labels, req.Assignees, live); err == nil || err.Error() != "remote artifact missing verified assignee(s): alice" {
		t.Fatalf("assignee=%v", err)
	}
	live.Assignees = []string{"ALICE"}
	if err := ValidateArtifact(req.URL, req.Labels, req.Assignees, live); err != nil {
		t.Fatal(err)
	}
	live.URL = "wrong?token=secret-value" + strings.Repeat("x", 4000)
	if err := ValidateArtifact(req.URL, req.Labels, req.Assignees, live); err == nil || strings.Contains(err.Error(), "secret-value") || len(err.Error()) > 2150 {
		t.Fatalf("URL error was missing, unbounded or unredacted")
	}
}
func TestArtifactTargetsRestrictProviderKindPairs(t *testing.T) {
	for _, tc := range []struct {
		provider, kind string
		mergeOnly, ok  bool
		wantKind       string
	}{
		{" GitHub ", " PULL_REQUEST ", true, true, "pr"},
		{"gitlab", "merge_request", true, true, "mr"},
		{"github", "issue", false, true, "issue"},
		{"gitlab", "issue", true, false, ""},
		{"github", "mr", false, false, ""},
		{"gitlab", "pr", true, false, ""},
	} {
		target, err := ArtifactTarget(tc.provider, tc.kind, " url ", tc.mergeOnly)
		if (err == nil) != tc.ok || (tc.ok && (target.Kind != tc.wantKind || target.URL != "url")) {
			t.Fatalf("%+v target=%+v err=%v", tc, target, err)
		}
	}
}
