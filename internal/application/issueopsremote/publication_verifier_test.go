package issueopsremote

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	publicationdomain "issueops/internal/domain/issueopspublication"
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

type knownURLReader struct {
	record   model.IssueOpsRecord
	knownURL string
}

func (r knownURLReader) DecodeSnapshot(contract.Intent) (model.IssueOpsRecord, contract.IntentPayload, error) {
	return r.record, contract.IntentPayload{Provider: "github", Kind: "pr", KnownURL: r.knownURL, Request: contract.ProviderCreateRequest{
		Labels: []string{"bug"}, Assignees: []string{"octocat"}, BaseBranch: "main", Title: "제목", HeadBranch: "7-demo", Body: "본문"}}, nil
}
func (r knownURLReader) Read(context.Context, string) (model.IssueOpsRecord, error) {
	return r.record, nil
}

func matchingCandidate(url string) contract.Candidate {
	return contract.Candidate{URL: url, Title: "제목", HeadBranch: "7-demo", BaseBranch: "main", BodySHA256: publicationdomain.BodySHA256("본문"),
		Labels: []string{"bug"}, Assignees: []string{"octocat"}}
}

func TestPublicationVerifierCandidateMustMatchIntentAndLinkedProject(t *testing.T) {
	const inProject = "https://github.com/acme/app/pull/3"
	codeElsewhere := journalRecord()
	codeElsewhere.BranchPrepare = &model.IssueOpsBranchPrepare{CodeProjectKey: "github.com/fork/app"}
	for _, tc := range []struct {
		name      string
		record    model.IssueOpsRecord
		known     string
		candidate func() contract.Candidate
		wantErr   string
	}{
		{"exact match", journalRecord(), "", func() contract.Candidate { return matchingCandidate(inProject) }, ""},
		{"known URL equal", journalRecord(), inProject, func() contract.Candidate { return matchingCandidate(inProject) }, ""},
		{"known URL differs", journalRecord(), "https://github.com/acme/app/pull/9", func() contract.Candidate { return matchingCandidate(inProject) }, "differs from the durable known URL"},
		{"title drift", journalRecord(), "", func() contract.Candidate { c := matchingCandidate(inProject); c.Title = "다른 제목"; return c }, "does not match the exact durable intent"},
		{"body drift", journalRecord(), "", func() contract.Candidate {
			c := matchingCandidate(inProject)
			c.BodySHA256 = publicationdomain.BodySHA256("수정")
			return c
		}, "does not match the exact durable intent"},
		{"label drift", journalRecord(), "", func() contract.Candidate { c := matchingCandidate(inProject); c.Labels = nil; return c }, "does not match the exact durable intent"},
		{"non-canonical URL", journalRecord(), "", func() contract.Candidate { return matchingCandidate("http://github.com/acme/app/pull/3") }, "canonical HTTPS URL"},
		{"issue URL is not a PR", journalRecord(), "", func() contract.Candidate { return matchingCandidate("https://github.com/acme/app/issues/3") }, "pull request URL"},
		{"foreign project", journalRecord(), "", func() contract.Candidate { return matchingCandidate("https://github.com/other/repo/pull/3") }, "match linked issue project"},
		{"code project overrides issue project", codeElsewhere, "", func() contract.Candidate { return matchingCandidate("https://github.com/fork/app/pull/3") }, ""},
		{"issue project rejected when code lives elsewhere", codeElsewhere, "", func() contract.Candidate { return matchingCandidate(inProject) }, "match linked issue project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := NewPublicationVerifier(knownURLReader{record: tc.record, knownURL: tc.known}, nil).VerifyCandidate(t.Context(), contract.Intent{}, tc.candidate())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
