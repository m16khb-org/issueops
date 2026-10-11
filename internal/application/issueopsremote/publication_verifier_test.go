package issueopsremote

import (
	"context"
	"errors"
	"testing"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

type verifierReader struct {
	record model.IssueOpsRecord
	err    error
}

func (r verifierReader) DecodeSnapshot(contract.Intent) (model.IssueOpsRecord, contract.IntentPayload, error) {
	return r.record, contract.IntentPayload{Provider: "github", Kind: "pr", Request: contract.ProviderCreateRequest{
		Labels: []string{"bug"}, Assignees: []string{"octocat"}, BaseBranch: "main"}}, r.err
}
func (r verifierReader) Read(context.Context, string) (model.IssueOpsRecord, error) {
	return r.record, nil
}

func TestPublicationVerifierLiveChecksProjectThenProviderVerification(t *testing.T) {
	record := journalRecord()
	var seen model.IssueOpsRemoteArtifactVerificationRequest
	verifier := NewPublicationVerifier(verifierReader{record: record}, func(req model.IssueOpsRemoteArtifactVerificationRequest) error {
		seen = req
		return nil
	})
	if err := verifier.VerifyLive(t.Context(), contract.Intent{}, " https://github.com/acme/app/pull/3 "); err != nil {
		t.Fatal(err)
	}
	if seen.URL != "https://github.com/acme/app/pull/3" || seen.TargetBranch != "main" || seen.Provider != "github" || seen.Kind != "pr" || len(seen.Labels) != 1 || len(seen.Assignees) != 1 {
		t.Fatalf("provider verification must see the trimmed URL and sealed metadata: %+v", seen)
	}

	called := false
	verifier = NewPublicationVerifier(verifierReader{record: record}, func(model.IssueOpsRemoteArtifactVerificationRequest) error {
		called = true
		return nil
	})
	if err := verifier.VerifyLive(t.Context(), contract.Intent{}, "https://github.com/other/repo/pull/3"); err == nil || called {
		t.Fatalf("a PR outside the linked project must be rejected before the provider is asked: err=%v called=%v", err, called)
	}

	boom := errors.New("provider says no")
	verifier = NewPublicationVerifier(verifierReader{record: record}, func(model.IssueOpsRemoteArtifactVerificationRequest) error { return boom })
	if err := verifier.VerifyLive(t.Context(), contract.Intent{}, "https://github.com/acme/app/pull/3"); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if err := NewPublicationVerifier(verifierReader{record: record}, nil).VerifyLive(t.Context(), contract.Intent{}, "https://github.com/acme/app/pull/3"); err != nil {
		t.Fatalf("without a provider verifier the local checks decide: %v", err)
	}
}

func TestPublicationVerifierPropagatesSnapshotDecodeFailure(t *testing.T) {
	boom := errors.New("corrupt snapshot")
	verifier := NewPublicationVerifier(verifierReader{err: boom}, nil)
	if err := verifier.VerifyLive(t.Context(), contract.Intent{}, "https://github.com/acme/app/pull/3"); !errors.Is(err, boom) {
		t.Fatalf("live err=%v", err)
	}
	if err := verifier.VerifyCandidate(t.Context(), contract.Intent{}, contract.Candidate{}); !errors.Is(err, boom) {
		t.Fatalf("candidate err=%v", err)
	}
}
