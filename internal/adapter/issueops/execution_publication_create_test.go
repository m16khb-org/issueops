package issueops

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	cycleapp "issueops/internal/application/issueopscycle"
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
	if err != nil || record.Execution.Pending == nil || record.Execution.Pending.Kind != contract.RemoteIntentKind {
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
	t.Parallel()

	for _, scenario := range []string{"success", "wrong holder", "wrong cwd", "wrong branch", "placeholder assignee", "stale review", "operation collision"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			fixture := newClaimableExecutionFixture(t, root, "197-create-preparation")
			actor := executionActor("codex", "publication-creator")
			record := fixture.record
			record.Phase = model.IssueOpsPhasePR
			record.IssueURL = "https://github.com/example/issueops/issues/69"
			record.Execution.Lease = model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &actor, ClaimedAt: "2026-09-28T00:00:00Z"}
			record.ImplementationReview = &model.IssueOpsImplementationReview{Verdict: "pass", ReviewedFingerprint: testChangeReader().ChangeFingerprint(record)}
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
			case "wrong holder":
				command.Actor.SessionID = "other-session"
				wantError = "holder"
			case "wrong cwd":
				command.CWD = t.TempDir()
				wantError = "canonical"

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
			if _, err := writeIssueOps(context.Background(), root, record); err != nil {
				t.Fatal(err)
			}
			var operationID func() (string, error)
			var collisionDB *sqlstore.DB
			const collisionID = "0123456789abcdef0123456789abcdef"
			if scenario == "operation collision" {
				wantError = "already exists"
				operationID = func() (string, error) { return collisionID, nil }
				var openErr error
				collisionDB, openErr = sqlstore.Open(root)
				if openErr != nil {
					t.Fatal(openErr)
				}
				if err := collisionDB.Put(externalIntentBucket, collisionID, []byte("existing intent")); err != nil {
					t.Fatal(err)
				}
			}
			repository := newPublicationJournalForTest(root, nil, operationID)
			before, err := repository.Latest(context.Background(), record.ID)
			if err != nil {
				t.Fatal(err)
			}
			provider := &publicationCreateProvider{t: t, root: root, id: record.ID}
			verified := 0
			verifier := remoteapp.NewPublicationVerifier(RemotePublicationStore{StateRoot: root}, func(req model.IssueOpsRemoteArtifactVerificationRequest) error {
				verified++
				if req.URL != "https://github.com/example/issueops/pull/197" {
					t.Fatalf("wrong verification: %+v", req)
				}
				return nil
			})
			service := publicationapp.NewCreateService(remoteapp.NewCreatePreparation(RemotePublicationObserver{CurrentFingerprint: testChangeReader().ChangeFingerprint, CurrentHead: testReadinessGit().Head, StateRoot: root}, cycleapp.NewMutationAuthority(samePath, NativeActorVerifier()), NativeActorVerifier()), repository, provider, verifier)
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
					t.Fatal("rejected publication wrote cycle state")
				}
				if collisionDB != nil {
					raw, exists, readErr := collisionDB.Get(externalIntentBucket, collisionID)
					if readErr != nil || !exists || string(raw) != "existing intent" {
						t.Fatalf("collision damaged existing intent: %q exists=%v err=%v", raw, exists, readErr)
					}
					_, exists, readErr = collisionDB.Get(leaseHolderBucket, leaseHolderIndexKey(actor))
					if readErr != nil || exists {
						t.Fatalf("failed intent inserted holder index: exists=%v err=%v", exists, readErr)
					}
				}
				return
			}
			if err != nil || provider.calls != 1 || verified != 1 || after.RemoteArtifact == nil || after.RemoteArtifact.URL != result.URL || after.Execution.Pending != nil || after.Execution.Completion != nil || after.Execution.Lease.Status != model.LeaseStatusActive {
				t.Fatalf("publication: result=%+v execution=%+v calls=%d verify=%d err=%v", result, after.Execution, provider.calls, verified, err)
			}
		})
	}
}
