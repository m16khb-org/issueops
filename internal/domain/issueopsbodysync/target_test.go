package issueopsbodysync

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopsbodysync"
)

func TestResolveTargetBindsPublicationToVerifiedArtifact(t *testing.T) {
	snapshot := TargetSnapshot{IssueURL: "https://gitlab.com/group/repo/-/issues/4", ArtifactURL: "https://gitlab.com/group/repo/-/merge_requests/8", ArtifactKind: contract.KindMR}
	kind, url, err := ResolveTarget(snapshot, contract.Command{Kind: contract.KindPR, URL: snapshot.ArtifactURL + "/"})
	if err != nil || kind != contract.KindMR || url != snapshot.ArtifactURL {
		t.Fatalf("verified artifact rejected: kind=%q url=%q err=%v", kind, url, err)
	}
	if _, _, err := ResolveTarget(snapshot, contract.Command{Kind: contract.KindPR, URL: "https://gitlab.com/other/repo/-/merge_requests/8"}); err == nil {
		t.Fatal("unrelated publication URL accepted")
	}
	kind, url, err = ResolveTarget(snapshot, contract.Command{Kind: contract.KindIssue, URL: snapshot.IssueURL + "/"})
	if err != nil || kind != contract.KindIssue || url != snapshot.IssueURL {
		t.Fatalf("linked issue rejected: kind=%q url=%q err=%v", kind, url, err)
	}
	kind, _, err = ResolveTarget(snapshot, contract.Command{Kind: contract.KindIssue, URL: "https://gitlab.com/group/repo/-/issues/5"})
	if err != nil || kind != contract.KindChild {
		t.Fatalf("child issue rejected: kind=%q err=%v", kind, err)
	}
}

func TestBodySyncPublicationGatesKeepFailureReasons(t *testing.T) {
	if err := ValidateGeneration(false, 0, 1); err == nil || !strings.Contains(err.Error(), "without an execution lease") {
		t.Fatalf("missing lease accepted: %v", err)
	}
	if err := ValidateGeneration(true, 3, 2); err == nil || !strings.Contains(err.Error(), "stale lease generation") {
		t.Fatalf("stale generation accepted: %v", err)
	}
	if err := ValidateGeneration(true, 3, 3); err != nil {
		t.Fatalf("matching generation rejected: %v", err)
	}
	if err := RejectClosedPublication(contract.KindPR, "merged"); err == nil {
		t.Fatal("merged publication accepted")
	}
	if err := RejectClosedPublication(contract.KindMR, ""); err == nil || !strings.Contains(err.Error(), "without an observed artifact state") {
		t.Fatalf("unknown publication state accepted: %v", err)
	}
	if err := RejectClosedPublication(contract.KindChild, "closed"); err != nil {
		t.Fatalf("child state rejected: %v", err)
	}
}
