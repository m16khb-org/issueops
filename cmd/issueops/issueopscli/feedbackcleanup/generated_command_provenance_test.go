package feedbackcleanup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	issueopscore "issueops/internal/adapter/issueops"
	commandparsecontract "issueops/internal/contract/commandparse"
	executionissue "issueops/internal/contract/executionissue"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/domain/commandparse"
	shelltoken "issueops/internal/domain/shelltoken"
	"issueops/internal/port"
	provenanceport "issueops/internal/port/issueopsprovenance"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cleanupProvenanceObserverStub struct {
	evidence provenanceport.Receipt
}

func TestCleanupFinishPreviewEmitsBoundFinishCommand(t *testing.T) {
	command := testCleanupCommand()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, true, true)
	var printed any
	deps := cleanupStatusDeps(nil)
	deps.Provenance = cleanupProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/issueops", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	deps.PrintJSON = func(value any) error {
		printed = value
		return nil
	}
	deps.Provider = func(string) (port.IssueProvider, error) {
		return &cleanupStatusProvider{snapshot: executionissue.ExecutionIssueSnapshot{
			URL: record.IssueURL, Body: issueopscontract.IssueBodyCompletionStartMarker, State: "closed",
		}}, nil
	}
	deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
		return issueopscontract.CleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: "abc123", BaseRefName: "main"}, nil
	}
	if err := command.RunCleanup([]string{"finish", "--id", record.ID, "--preview", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	result, ok := printed.(issueopscontract.CleanupFinishResult)
	if !ok || !strings.Contains(result.NextCommand, "cleanup finish") || !strings.Contains(result.NextCommand, "--generated-for-generation 1") {
		t.Fatalf("cleanup finish preview result = %#v", printed)
	}
}

func (s cleanupProvenanceObserverStub) Observe(context.Context) (provenanceport.Receipt, error) {
	return s.evidence, nil
}

func TestBindCleanupNextCommandCoversPreviewAndFinish(t *testing.T) {
	observer := cleanupProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/issueops", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	commands := map[string]string{
		"preview": "issueops cleanup finish --id io-1 --preview --json",
		"finish":  "issueops cleanup finish --id io-1 --apply --confirm --fingerprint abc --json",
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			bound, err := bindCleanupNextCommand(command, 9, observer)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"--generated-by-executable", "--generated-by-sha256", "--generated-for-generation 9"} {
				if !strings.Contains(bound, want) {
					t.Fatalf("bound cleanup command %q does not contain %q", bound, want)
				}
			}
		})
	}
}

func TestBindCleanupNextCommandMissingObserverHasNoFallback(t *testing.T) {
	bound, err := bindCleanupNextCommand("issueops cleanup finish --id io-1 --preview --json", 9, nil)
	if err == nil || bound != "" {
		t.Fatalf("missing observer bound=%q err=%v", bound, err)
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok || fields.IssueOpsErrorFields()["code"] != "generated_command_provenance_observation_failed" {
		t.Fatalf("missing observer failure is not structured: %T %v", err, err)
	}
}

func TestCurrentRelayCleanupGeneratedCommandDogfood(t *testing.T) {
	command := testCleanupCommand()
	binary := os.Getenv("ISSUEOPS_CURRENT_RELAY_DOGFOOD_BINARY")
	if binary == "" {
		t.Skip("live current-relay dogfood requires an exact binary")
	}
	binary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	binaryHash := sha256.Sum256(binaryBytes)

	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, true, true)
	if _, err := (issueopscore.CycleRecordStore{StateRoot: issueOpsStateRootForTest()}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	observer := cleanupProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: binary, ExecutableSHA256: hex.EncodeToString(binaryHash[:]),
	}}
	provider := &liveCleanupProvider{cleanupStatusProvider: cleanupStatusProvider{snapshot: executionissue.ExecutionIssueSnapshot{
		URL: record.IssueURL, Body: issueopscontract.IssueBodyCompletionStartMarker, State: "closed",
	}}}
	var printed any
	deps := cleanupStatusDeps(nil)
	deps.Provenance = observer
	deps.PrintJSON = func(value any) error { printed = value; return nil }
	deps.CleanupFinishGit = func(_ string, args ...string) (int, string) {
		switch args[0] {
		case "status", "ls-remote", "worktree", "update-ref":
			return 0, ""
		case "rev-parse":
			return 0, "abc123\n"
		default:
			return 1, "unexpected git call"
		}
	}
	deps.Provider = func(string) (port.IssueProvider, error) { return provider, nil }
	deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
		return issueopscontract.CleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: "abc123", BaseRefName: "main"}, nil
	}
	if err := command.RunCleanup([]string{"finish", "--id", record.ID, "--preview", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	preview, ok := printed.(issueopscontract.CleanupFinishResult)
	if !ok || preview.NextCommand == "" {
		t.Fatalf("cleanup current-relay preview = %#v", printed)
	}
	tokens := shelltoken.SplitCommandTokens(preview.NextCommand)
	if len(tokens) < 3 || tokens[0] != binary || tokens[1] != "cleanup" {
		t.Fatalf("cleanup generated command = %q", preview.NextCommand)
	}
	clean, provenance, present, err := commandparsecontract.ConsumeGeneratedCommandProvenance(tokens[1:])
	if err != nil || !present {
		t.Fatalf("cleanup generated provenance err=%v present=%t", err, present)
	}
	observed := commandparsecontract.GeneratedCommandProvenance{
		ExecutablePath: observer.evidence.ExecutablePath, ExecutableSHA256: observer.evidence.ExecutableSHA256, LeaseGeneration: 1,
	}
	if err := commandparse.ValidateGeneratedCommandInvocation(provenance, observed, 1); err != nil {
		t.Fatal(err)
	}
	if err := command.RunCleanup(clean[1:], deps); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.ReadIssueOps(issueOpsStateRootForTest(), record.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cleanup dogfood record was not deleted: %v", err)
	}
}

type liveCleanupProvider struct {
	cleanupStatusProvider
}

func (p *liveCleanupProvider) UpdateIssueBodySection(ctx context.Context, req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{OK: true, URL: req.IssueURL, Updated: true}, nil
}
