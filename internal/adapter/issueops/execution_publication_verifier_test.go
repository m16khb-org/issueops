package issueops

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	remoteapp "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	publicationdomain "issueops/internal/domain/issueopspublication"
)

func TestPublicationVerifierUsesLatestProjectAndPhaseBeforeLiveReadback(t *testing.T) {
	root := t.TempDir()
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "66-verification"})
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = model.IssueOpsPhasePR
	record.IssueURL = "https://github.com/stale/repo/issues/1"
	rawRecord, _ := json.Marshal(record)
	record.IssueURL = "https://github.com/acme/repo/issues/1"
	if _, err := WriteIssueOps(context.Background(), root, record); err != nil {
		t.Fatal(err)
	}
	payload := contract.IntentPayload{SchemaVersion: 1, OperationID: "operation", Generation: 1, Provider: "github", Kind: "pr", Request: contract.ProviderCreateRequest{ProjectKey: "acme/repo", Title: "Fix", Body: "body", HeadBranch: "work", BaseBranch: "main", ExpectedHeadSHA: strings.Repeat("a", 40), Labels: []string{"backend"}, Assignees: []string{"owner"}, Draft: true}}
	rawPayload, _ := json.Marshal(payload)
	intent := contract.Intent{OperationID: "operation", Record: contract.RecordSnapshot{ID: record.ID, Raw: rawRecord}, Raw: rawPayload}
	candidate := contract.Candidate{URL: "https://github.com/acme/repo/pull/1", ProjectKey: "acme/repo", SourceProjectKey: "acme/repo", HeadBranch: "work", BaseBranch: "main", HeadSHA: strings.Repeat("a", 40), Title: "Draft: Fix", BodySHA256: publicationdomain.BodySHA256("body"), Labels: []string{"backend"}, Assignees: []string{"owner"}, Draft: true, State: "opened"}
	calls := 0
	verifier := remoteapp.NewPublicationVerifier(RemotePublicationStore{StateRoot: root}, func(req model.IssueOpsRemoteArtifactVerificationRequest) error {
		calls++
		if req.URL != candidate.URL || req.TargetBranch != "main" {
			t.Fatalf("readback=%+v", req)
		}
		return nil
	})
	if err := verifier.VerifyCandidate(context.Background(), intent, candidate); err != nil {
		t.Fatal(err)
	}
	foreign := candidate
	foreign.URL = "https://github.com/foreign/repo/pull/1"
	if err := verifier.VerifyCandidate(context.Background(), intent, foreign); err == nil {
		t.Fatal("foreign project accepted")
	}
	wrongBody := candidate
	wrongBody.BodySHA256 = strings.Repeat("b", 64)
	if err := verifier.VerifyCandidate(context.Background(), intent, wrongBody); err == nil {
		t.Fatal("wrong body accepted")
	}
	payload.KnownURL = "https://github.com/acme/repo/pull/2"
	intent.Raw, _ = json.Marshal(payload)
	if err := verifier.VerifyCandidate(context.Background(), intent, candidate); err == nil || !strings.Contains(err.Error(), "durable known URL") {
		t.Fatalf("known URL mismatch=%v", err)
	}
	payload.KnownURL = candidate.URL
	intent.Raw, _ = json.Marshal(payload)
	if err := verifier.VerifyCandidate(context.Background(), intent, candidate); err != nil {
		t.Fatal(err)
	}
	record.Phase = model.IssueOpsPhasePlan
	if _, err := WriteIssueOps(context.Background(), root, record); err != nil {
		t.Fatal(err)
	}
	if err := verifier.VerifyLive(context.Background(), intent, candidate.URL); err == nil || calls != 0 {
		t.Fatalf("premature readback calls=%d err=%v", calls, err)
	}
	record.Phase = model.IssueOpsPhasePR
	if _, err := WriteIssueOps(context.Background(), root, record); err != nil {
		t.Fatal(err)
	}
	if err := verifier.VerifyLive(context.Background(), intent, " "+candidate.URL+" "); err != nil || calls != 1 {
		t.Fatalf("valid readback calls=%d err=%v", calls, err)
	}
}
