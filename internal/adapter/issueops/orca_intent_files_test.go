package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	app "issueops/internal/application/issueopspreparation"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspreparation"
)

func sealedIntentFilesFixture(t *testing.T) (model.IssueOpsRecord, contract.Intent) {
	t.Helper()
	_, record, intent := resumeIntentFixture(t, "github", 16)
	token, _, err := createClaimToken(record)
	if err != nil {
		t.Fatal(err)
	}
	digest := tokenSHA256(token)
	record.Execution.Lease.ClaimTokenSHA256 = digest
	intent.ClaimTokenSHA256 = digest
	intent.ResumeLease.ClaimTokenSHA256 = digest
	prompt, packet := []byte("sealed owner prompt\n"), []byte("sealed context packet\n")
	for path, data := range map[string][]byte{intent.Launch.PromptPath: prompt, intent.Launch.ContextPacketPath: packet} {
		if err := writeExecutionOwnerArtifact(record.Execution.Workspace.Root, path, data); err != nil {
			t.Fatal(err)
		}
	}
	intent.Launch.PromptSHA256 = digestExecutionOwnerBytes(prompt)
	intent.Launch.ContextPacketSHA256 = digestExecutionOwnerBytes(packet)
	return record, intent
}

func TestIntentInspectionDoesNotRequireArtifactsButInvocationDoes(t *testing.T) {
	t.Parallel()

	record, intent := sealedIntentFilesFixture(t)
	builder := app.IntentRequestBuilder{Files: OrcaIntentFiles{}}
	request, err := builder.Build(preparationRecordForTest(t, record), intent)
	if err != nil || request.Launch == nil || request.Launch.Prompt != "sealed owner prompt\n" {
		t.Fatalf("sealed invocation failed: %v", err)
	}
	if err := os.RemoveAll(record.Execution.Workspace.Root); err != nil {
		t.Fatal(err)
	}
	inspected, err := builder.Inspect(preparationRecordForTest(t, record), intent)
	if err != nil || inspected.Launch == nil || inspected.Launch.Prompt != "" || inspected.Generation != 2 {
		t.Fatalf("metadata inspection depended on missing files: %+v %v", inspected, err)
	}
	if _, err := builder.Build(preparationRecordForTest(t, record), intent); err == nil || !strings.Contains(err.Error(), "sealed claim token identity changed") {
		t.Fatalf("invocation accepted missing token: %v", err)
	}
}

func TestIntentBuilderRejectsArtifactAndWorkspaceDrift(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"token", "prompt", "packet", "prompt path", "workspace root", "prepared record root", "parent", "generation"} {
		t.Run(kind, func(t *testing.T) {
			record, intent := sealedIntentFilesFixture(t)
			builder := app.IntentRequestBuilder{Files: OrcaIntentFiles{}}
			if _, err := builder.Build(preparationRecordForTest(t, record), intent); err != nil {
				t.Fatal(err)
			}
			want := ""
			switch kind {
			case "token":
				if err := os.WriteFile(claimTokenPath(record), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
				want = "sealed claim token identity changed"
			case "prompt", "packet":
				path := intent.Launch.PromptPath
				want = "sealed owner prompt identity changed"
				if kind == "packet" {
					path = intent.Launch.ContextPacketPath
					want = "sealed context packet identity changed"
				}
				if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "prompt path":
				intent.Launch.PromptPath = filepath.Join(t.TempDir(), "foreign-prompt")
				want = "sealed owner artifact path changed"
			case "workspace root":
				record.Execution.Workspace.Root = t.TempDir()
				want = "Orca intent record identity changed"
			case "prepared record root":
				record.WorktreePath = t.TempDir()
				want = "Orca prepared workspace identity changed"
			case "parent":
				record.Execution.Workspace.ParentWorktree = t.TempDir()
				want = "Orca intent record identity changed"
			case "generation":
				record.Execution.Lease.Generation++
				want = "Orca intent authority changed before CAS"
			}
			if _, err := builder.Build(preparationRecordForTest(t, record), intent); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("%s accepted or wrong refusal: %v", kind, err)
			}
		})
	}
}

func TestLaunchHydratorUsesCurrentIdentityAndPreservesRequest(t *testing.T) {
	t.Parallel()

	record, intent := sealedIntentFilesFixture(t)
	request := contract.IntentRequest{Stage: intent.Stage, Marker: intent.Marker, Launch: &contract.LaunchRequest{
		PromptPath: intent.Launch.PromptPath, PromptSHA256: intent.Launch.PromptSHA256,
		ContextPacketPath: intent.Launch.ContextPacketPath, ContextPacketSHA256: intent.Launch.ContextPacketSHA256,
	}}
	hydrator := app.LaunchHydrator{
		ReadRecord: func(id string) (contract.Record, error) {
			if id != record.ID {
				t.Fatal("wrong record ID")
			}
			return preparationRecordForTest(t, record), nil
		},
		ReadIntent: func(operationID string) (contract.Intent, error) {
			if operationID != record.Execution.Pending.OperationID {
				t.Fatal("wrong operation")
			}
			return intent, nil
		},
		Builder: app.IntentRequestBuilder{Files: OrcaIntentFiles{}},
	}
	hydrated, err := hydrator.Hydrate(record.ID, request)
	if err != nil || hydrated.Launch.Prompt != "sealed owner prompt\n" || request.Launch.Prompt != "" {
		t.Fatalf("hydrate=%+v err=%v original prompt=%q", hydrated, err, request.Launch.Prompt)
	}
	request.Launch.ContextPacketSHA256 = strings.Repeat("0", 64)
	if _, err := hydrator.Hydrate(record.ID, request); err == nil || !strings.Contains(err.Error(), "sealed owner launch identity changed") {
		t.Fatalf("changed launch accepted: %v", err)
	}
}

func preparationRecordForTest(t *testing.T, record model.IssueOpsRecord) contract.Record {
	t.Helper()
	projected, err := PreparationIntentRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	return projected
}
