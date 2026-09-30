package issueops

import (
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

func TestOrcaIntentIssueIdentityAllowsOnlyUnverifiedGitHubForResumeLaunch(t *testing.T) {
	record := issueops.IssueOpsRecord{
		ID:       "io-aaaaaaaaaaaa",
		IssueURL: "https://github.com/acme/repo/issues/194",
		BranchPrepare: &issueops.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: "https://github.com/acme/repo/issues/194",
		},
	}
	payload := preparationcontract.Intent{
		Purpose: preparationcontract.PurposeResume, LifecycleID: record.ID,
		Probe: intentContractProbeRequest(port.ExecutionOrcaProbeRequest{Provider: "github", Issue: 194}),
	}
	if err := preparationdomain.ValidateIntentIssueIdentity(preparationRecordForTest(t, record), payload); err != nil {
		t.Fatalf("미검증 GitHub branch identity가 owner의 post-claim 검증 전에 resume launch를 막았다: %v", err)
	}

	gitlab := record
	gitlab.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/194"
	prepared := *record.BranchPrepare
	prepared.Provider = "gitlab"
	prepared.IssueURL = gitlab.IssueURL
	gitlab.BranchPrepare = &prepared
	payload.Probe = intentContractProbeRequest(port.ExecutionOrcaProbeRequest{Provider: "gitlab", Issue: 194})
	if err := preparationdomain.ValidateIntentIssueIdentity(preparationRecordForTest(t, gitlab), payload); err == nil {
		t.Fatal("미검증 GitLab branch identity가 resume launch에 허용됐다")
	}
}

func TestSealExternalOrcaIntentPayloadUsesTheVerifiedRecordIdentity(t *testing.T) {
	_, record := executionPrepareRecord(t)
	workspace, err := executionWorkspaceRequest(record, true)
	if err != nil {
		t.Fatal(err)
	}
	payload := preparationcontract.Intent{
		SchemaVersion:   issueops.IssueOpsSchemaVersion,
		Purpose:         preparationcontract.PurposePrepare,
		OperationID:     strings.Repeat("b", 32),
		LifecycleID:     record.ID,
		Generation:      1,
		Stage:           intentContractStage(port.ExecutionOrcaIntentWorktree),
		StartedAt:       "2026-07-30T00:00:00Z",
		InvocationState: preparationcontract.InvocationNotInvoked,
		Workspace:       intentContractWorkspaceRequest(workspace),
		Probe: intentContractProbeRequest(port.ExecutionOrcaProbeRequest{
			Repo: record.Repo, Host: "codex", Model: "gpt-6-astra", Effort: "xhigh",
			Provider: "gitlab", Issue: 2646,
		}),
		IssueBodySHA256: strings.Repeat("a", 64),
	}

	identity, err := preparationdomain.PrepareIssueIdentity(record.IssueURL, &preparationcontract.IssueLinkEvidence{Provider: record.BranchPrepare.Provider, IssueURL: record.BranchPrepare.IssueURL, LinkVerified: record.BranchPrepare.LinkVerified})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := preparationdomain.SealIntent(payload, identity)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Probe.Provider != "github" || sealed.Probe.Issue != 16 {
		t.Fatalf("sealed issue identity = %s#%d", sealed.Probe.Provider, sealed.Probe.Issue)
	}
	wantSuffix := " provider=github issue=16"
	if !strings.HasSuffix(sealed.Marker, wantSuffix) || sealed.Probe.Marker != sealed.Marker {
		t.Fatalf("sealed markers = payload:%q probe:%q", sealed.Marker, sealed.Probe.Marker)
	}
}

func TestPreparationRepositoryRejectsRecordIdentityDriftBeforePersistence(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	workspace, err := executionWorkspaceRequest(record, true)
	if err != nil {
		t.Fatal(err)
	}
	passed := record
	record.IssueURL = "https://github.com/acme/repo/issues/17"
	record.BranchPrepare.IssueURL = record.IssueURL
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	probe := port.ExecutionOrcaProbeRequest{
		Repo: record.Repo, Host: "codex", Model: "gpt-6-astra", Effort: "xhigh",
		Provider: "github", Issue: 16,
	}
	snapshot := executionOwnerSnapshot{issue: executionOwnerIssue{BodySHA256: strings.Repeat("a", 64)}}

	_, _, err = beginOrcaIntentViaRepository(stateRoot, passed, workspace, probe, issueops.ExecutionPrepareRequest{
		OwnerHost: "codex", OwnerModel: "gpt-6-astra", OwnerEffort: "xhigh",
	}, snapshot, nil)
	if err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("identity drift error = %v", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.Execution != nil {
		t.Fatalf("identity drift persisted execution: %#v", persisted.Execution)
	}
	rows, rowsErr := sqlstore.GetAllExisting(stateRoot, externalIntentBucket)
	if rowsErr != nil {
		t.Fatal(rowsErr)
	}
	if len(rows) != 0 {
		t.Fatalf("identity drift persisted external intents: %#v", rows)
	}
}
