package issueops

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"issueops/internal/adapter/issueops/implementation"
	publicationapp "issueops/internal/application/issueopspublication"
	remoteapp "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

type publicationCreateProvider struct {
	t        *testing.T
	root, id string
	calls    int
}

func (p *publicationCreateProvider) Create(_ context.Context, provider string, request contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
	p.t.Helper()
	p.calls++
	record, err := ReadIssueOps(p.root, p.id)
	if err != nil || record.Execution.Pending == nil || record.Execution.Pending.Kind != externalIntentRemotePR {
		p.t.Fatalf("provider invoked before durable intent: record=%+v err=%v", record.Execution, err)
	}
	if provider != "github" || request.BaseBranch != "main" || request.ProjectKey != "github.com/example/issueops" || !request.Draft || !strings.Contains(request.Body, record.Execution.Pending.Marker) {
		p.t.Fatalf("wrong prepared request: %+v", request)
	}
	return contract.ProviderCreateResult{OK: true, URL: "https://github.com/example/issueops/pull/197"}, contract.InvocationUnknown, nil
}
func (p *publicationCreateProvider) Inspect(context.Context, contract.Intent) (contract.Inventory, bool, error) {
	p.t.Fatal("unexpected reconcile")
	return contract.Inventory{}, false, nil
}

func TestPublicationCreateUsesPreparedDomainRulesBeforePersistence(t *testing.T) {
	for _, scenario := range []string{"success", "wrong branch", "placeholder assignee", "stale review"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			fixture := newClaimableExecutionFixture(t, root, "197-create-preparation")
			actor := executionActor("codex", "publication-creator")
			record := fixture.record
			record.Phase = model.IssueOpsPhasePR
			record.IssueURL = "https://github.com/example/issueops/issues/69"
			record.Execution.Lease = model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &actor, ClaimedAt: "2026-09-28T00:00:00Z"}
			record.ImplementationReview = &model.IssueOpsImplementationReview{Verdict: "pass", ReviewedFingerprint: implementation.ChangeFingerprint(record)}
			if record.ImplementationReview.ReviewedFingerprint == "" {
				t.Fatal("fixture fingerprint is missing")
			}
			command := contract.CreateCommand{ID: record.ID, Provider: "github", Title: "Prepared publication", Body: "Verified change.", Head: record.Branch, Base: "main", Labels: []string{"enhancement"}, Assignees: []string{"maintainer"}, ExpectedGeneration: 1, CWD: fixture.worktree, Confirm: true,
				Actor: contract.Actor{Host: actor.Host, SessionID: actor.SessionID, SessionProcess: &contract.ProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}},
			}
			for _, receipt := range actor.ProcessAncestry {
				command.Actor.ProcessAncestry = append(command.Actor.ProcessAncestry, contract.ProcessReceipt(receipt))
			}
			wantError := ""
			switch scenario {
			case "wrong branch":
				command.Base = "other"
				wantError = "linked issue authority"
			case "placeholder assignee":
				command.Assignees = []string{"@me"}
				wantError = "placeholder"
			case "stale review":
				record.ImplementationReview.ReviewedFingerprint = "stale"
				wantError = "implementation_review_stale"
			}
			if _, err := writeIssueOps(root, record); err != nil {
				t.Fatal(err)
			}
			repository := NewRemotePublicationRepository(root, nil, nil)
			before, err := repository.Latest(context.Background(), record.ID)
			if err != nil {
				t.Fatal(err)
			}
			provider := &publicationCreateProvider{t: t, root: root, id: record.ID}
			verified := 0
			verifier := RemotePublicationVerifier{StateRoot: root, Verify: func(req model.IssueOpsRemoteArtifactVerificationRequest) error {
				verified++
				if req.URL != "https://github.com/example/issueops/pull/197" {
					t.Fatalf("wrong verification: %+v", req)
				}
				return nil
			}}
			service := publicationapp.NewCreateService(remoteapp.NewCreatePreparation(RemotePublicationObserver{StateRoot: root}), repository, provider, verifier)
			result, err := service.Create(context.Background(), command)
			after, readErr := ReadIssueOps(root, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if wantError != "" {
				if err == nil || !strings.Contains(err.Error(), wantError) || provider.calls != 0 || verified != 0 {
					t.Fatalf("rejection: result=%+v calls=%d verify=%d err=%v", result, provider.calls, verified, err)
				}
				stored, readErr := repository.Latest(context.Background(), record.ID)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if !bytes.Equal(before.Raw, stored.Raw) {
					t.Fatal("preparation failure wrote cycle state")
				}
				return
			}
			if err != nil || provider.calls != 1 || verified != 1 || after.RemoteArtifact == nil || after.RemoteArtifact.URL != result.URL || after.Execution.Pending != nil || after.Execution.Completion != nil || after.Execution.Lease.Status != model.LeaseStatusActive {
				t.Fatalf("publication: result=%+v execution=%+v calls=%d verify=%d err=%v", result, after.Execution, provider.calls, verified, err)
			}
		})
	}
}
