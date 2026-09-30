package issueopspreparation

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestIntentAuthorityRequiresMatchingPendingAndLease(t *testing.T) {
	intent := preparationcontract.Intent{
		Purpose: preparationcontract.PurposePrepare, LifecycleID: "io-1", OperationID: "op-1",
		Generation: 1, Stage: preparationcontract.IntentStageWorktree,
	}
	record := leasecontract.Record{ID: "io-1", Execution: &leasecontract.Execution{
		Lease:   leasecontract.Lease{Generation: 1, Status: "released"},
		Pending: &leasecontract.ExternalIntent{OperationID: "op-1", Marker: "marker", Kind: "worktree_create"},
	}}
	if err := ValidateIntentRecordAuthority(record, intent); err == nil || !strings.Contains(err.Error(), "authority changed") {
		t.Fatalf("missing marker should reject authority: %v", err)
	}
	intent.Marker = "marker"
	if err := ValidateIntentRecordAuthority(record, intent); err != nil {
		t.Fatalf("matching authority rejected: %v", err)
	}
	record.Execution.Lease.Generation = 2
	if err := ValidateIntentRecordAuthority(record, intent); err == nil {
		t.Fatal("stale lease generation accepted")
	}
}

func TestCanonicalizeIntentPreservesBytesAndRejectsStaleAuthority(t *testing.T) {
	const operationID = "0123456789abcdef0123456789abcdef"
	codec := preparationcontract.IntentCodec{}
	marker, err := codec.RenderMarker(preparationcontract.MarkerIdentity{
		Purpose: preparationcontract.PurposePrepare, LifecycleID: "io-1", Generation: 1,
		OperationID: operationID, Provider: "github", Issue: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	intent := preparationcontract.Intent{
		SchemaVersion: 1, Purpose: preparationcontract.PurposePrepare,
		OperationID: operationID, LifecycleID: "io-1", Generation: 1,
		Stage: preparationcontract.IntentStageWorktree, Marker: marker,
		StartedAt: "2026-09-24T00:00:00Z", InvocationState: preparationcontract.InvocationNotInvoked,
		Workspace:       preparationcontract.WorkspaceRequest{LifecycleID: "io-1", SourceRoot: "/repo", Root: "/repo.wt", Branch: "work", BaseHead: strings.Repeat("a", 40)},
		Probe:           preparationcontract.ProbeRequest{Repo: "/repo", Host: "codex", Model: "m", Provider: "github", Issue: 1, Marker: marker},
		IssueBodySHA256: strings.Repeat("b", 64),
	}
	raw, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	record := leasecontract.Record{ID: "io-1", Execution: &leasecontract.Execution{
		Lease:   leasecontract.Lease{Generation: 1, Status: "released"},
		Pending: &leasecontract.ExternalIntent{OperationID: operationID, Marker: marker, Kind: "worktree_create"},
	}}
	_, preserved, err := CanonicalizeIntent(record, raw)
	if err != nil || !bytes.Equal(preserved, raw) {
		t.Fatalf("canonical bytes changed: %v", err)
	}
	record.Execution.Lease.Generation = 2
	_, rejected, err := CanonicalizeIntent(record, raw)
	if err == nil || rejected != nil {
		t.Fatalf("stale authority accepted: %v", err)
	}
}

func TestSealIntentBindsIssueIdentity(t *testing.T) {
	const operationID = "0123456789abcdef0123456789abcdef"
	intent := preparationcontract.Intent{
		SchemaVersion: 1, Purpose: preparationcontract.PurposePrepare,
		OperationID: operationID, LifecycleID: "io-1", Generation: 1,
		Stage:     preparationcontract.IntentStageWorktree,
		StartedAt: "2026-09-24T00:00:00Z", InvocationState: preparationcontract.InvocationNotInvoked,
		Workspace:       preparationcontract.WorkspaceRequest{LifecycleID: "io-1", SourceRoot: "/repo", Root: "/repo.wt", Branch: "work", BaseHead: strings.Repeat("a", 40)},
		Probe:           preparationcontract.ProbeRequest{Repo: "/repo", Host: "codex", Model: "m"},
		IssueBodySHA256: strings.Repeat("b", 64),
	}
	sealed, err := SealIntent(intent, preparationcontract.IssueIdentity{Provider: "github", Issue: 199})
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Probe.Provider != "github" || sealed.Probe.Issue != 199 || sealed.Marker == "" || sealed.Probe.Marker != sealed.Marker {
		t.Fatalf("issue identity was not sealed: %+v", sealed)
	}
	if _, err := SealIntent(intent, preparationcontract.IssueIdentity{Provider: "gitlab", Issue: 0}); err == nil || !strings.Contains(err.Error(), "intent_identity_mismatch") {
		t.Fatalf("invalid issue accepted: %v", err)
	}
}

func TestPrepareIssueIdentityRequiresVerifiedGitLabLink(t *testing.T) {
	record := leasecontract.Record{
		IssueURL:      "https://gitlab.com/example/repo/-/work_items/199",
		BranchPrepare: []byte(`{"provider":"gitlab","issue_url":"https://gitlab.com/example/repo/-/work_items/199","link_verified":false}`),
	}
	if _, err := PrepareIssueIdentity(record.IssueURL, preparationcontract.DecodeIssueLinkEvidence(record.BranchPrepare)); err == nil {
		t.Fatal("unverified GitLab link accepted")
	}
	record.BranchPrepare = []byte(`{"provider":"gitlab","issue_url":"https://gitlab.com/example/repo/-/work_items/199","link_verified":true}`)
	issue, err := PrepareIssueIdentity(record.IssueURL, preparationcontract.DecodeIssueLinkEvidence(record.BranchPrepare))
	if err != nil || issue.Provider != "gitlab" || issue.Issue != 199 {
		t.Fatalf("verified GitLab link rejected: %+v %v", issue, err)
	}
}

func TestPrepareIssueIdentityPreservesUnverifiedGitHubException(t *testing.T) {
	record := leasecontract.Record{
		IssueURL:      "https://github.com/example/repo/issues/199",
		BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/example/repo/issues/199","link_verified":false}`),
	}
	issue, err := PrepareIssueIdentity(record.IssueURL, preparationcontract.DecodeIssueLinkEvidence(record.BranchPrepare))
	if err != nil || issue.Provider != "github" || issue.Issue != 199 {
		t.Fatalf("GitHub preparation exception changed: %+v %v", issue, err)
	}
}

func TestPrepareIssueIdentityRejectsTypedURLMismatch(t *testing.T) {
	link := &preparationcontract.IssueLinkEvidence{
		Provider: "gitlab", IssueURL: "https://gitlab.com/example/repo/-/work_items/199", LinkVerified: true,
	}
	if _, err := PrepareIssueIdentity("https://gitlab.com/other/repo/-/work_items/199", link); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched issue URL accepted: %v", err)
	}
	if _, err := PrepareIssueIdentity("", nil); err == nil || !strings.Contains(err.Error(), "requires verified") {
		t.Fatalf("missing sidecar changed error: %v", err)
	}
}

func TestResumeIntentAuthorityRejectsBindingDrift(t *testing.T) {
	lease := leasecontract.Lease{Generation: 2, Status: "claimable", ClaimTokenSHA256: strings.Repeat("a", 64)}
	prior := preparationcontract.ResumeBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", LeaseGeneration: 1}
	intent := preparationcontract.Intent{
		Purpose: preparationcontract.PurposeResume, LifecycleID: "io-1", OperationID: "op-2",
		Generation: 2, Stage: preparationcontract.IntentStageTerminal,
		Marker: "marker", ResumeLease: &lease, PriorBinding: &prior,
	}
	record := leasecontract.Record{ID: "io-1", Execution: &leasecontract.Execution{
		Lease:   lease,
		Pending: &leasecontract.ExternalIntent{OperationID: "op-2", Marker: "marker", Kind: "owner_launch"},
		Orca:    &leasecontract.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", LeaseGeneration: 1},
	}}
	if err := ValidateIntentRecordAuthority(record, intent); err != nil {
		t.Fatalf("matching resume authority rejected: %v", err)
	}
	record.Execution.Orca.RuntimeID = "other-runtime"
	if err := ValidateIntentRecordAuthority(record, intent); err == nil {
		t.Fatal("stale prior binding accepted")
	}
}

func TestPrepareIssueIdentityAcceptsVerifiedSelfHostedGitLab(t *testing.T) {
	for _, path := range []string{"issues", "work_items"} {
		issueURL := "https://code.example.org/group/repo/-/" + path + "/69"
		identity, err := PrepareIssueIdentity(issueURL, &preparationcontract.IssueLinkEvidence{Provider: "gitlab", IssueURL: issueURL, LinkVerified: true})
		if err != nil || identity.Provider != "gitlab" || identity.Issue != 69 {
			t.Fatalf("self-hosted GitLab %s: %+v %v", path, identity, err)
		}
	}
}

func TestPrepareIssueIdentityRejectsUntrustedOrMalformedLinks(t *testing.T) {
	for _, tc := range []struct {
		name, provider, issueURL string
		verified                 bool
	}{
		{"unverified self hosted", "gitlab", "https://code.example.org/group/repo/-/issues/69", false},
		{"provider mismatch", "github", "https://code.example.org/group/repo/-/issues/69", true},
		{"unknown host without gitlab route", "gitlab", "https://code.example.org/group/repo/issues/69", true},
		{"no hostname", "gitlab", "/group/repo/-/issues/69", true},
		{"nonnumeric issue", "gitlab", "https://code.example.org/group/repo/-/issues/no", true},
		{"zero issue", "gitlab", "https://code.example.org/group/repo/-/issues/0", true},
		{"extra path", "gitlab", "https://code.example.org/group/repo/-/issues/69/edit", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PrepareIssueIdentity(tc.issueURL, &preparationcontract.IssueLinkEvidence{Provider: tc.provider, IssueURL: tc.issueURL, LinkVerified: tc.verified})
			if err == nil {
				t.Fatal("untrusted or malformed issue identity accepted")
			}
		})
	}
}
