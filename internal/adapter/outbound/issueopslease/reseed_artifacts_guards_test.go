package issueopslease

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func orcaReseedArtifactRecord(root string, generation uint64) leasecontract.Record {
	record := reseedArtifactRecord(root, generation)
	record.Execution.Mode = "orca"
	record.Execution.Selection = leaseSelectionFixture("orca")
	return record
}

func TestReseedArtifactsRequireAnExecution(t *testing.T) {
	artifacts := NewReseedArtifacts(nil)
	if _, err := artifacts.Prepare(t.Context(), leasecontract.Record{}); err == nil || !strings.Contains(err.Error(), "without an execution") {
		t.Fatalf("prepare err=%v", err)
	}
	if err := artifacts.CleanupSuperseded(t.Context(), leasecontract.Record{}); err == nil || !strings.Contains(err.Error(), "without an execution") {
		t.Fatalf("cleanup err=%v", err)
	}
	if err := cleanupReseedTarget(leasecontract.Record{}); err == nil || !strings.Contains(err.Error(), "without an execution") {
		t.Fatalf("cleanup target err=%v", err)
	}
}

func TestReseedArtifactsPrepareWritesTokenWhoseHashMatchesReceipt(t *testing.T) {
	root := t.TempDir()
	record := reseedArtifactRecord(root, 4)
	receipt, err := NewReseedArtifacts(nil).Prepare(t.Context(), record)
	if err != nil {
		t.Fatal(err)
	}
	token, err := readCurrentClaimToken(record, receipt.Receipt.ClaimTokenPath)
	if err != nil {
		t.Fatalf("prepared token must satisfy the claim reader contract: %v", err)
	}
	if claimTokenSHA256(token) != receipt.TokenSHA256 || len(token) != 64 {
		t.Fatalf("token=%q hash=%q", token, receipt.TokenSHA256)
	}
	if receipt.Receipt.Execution.Lease.Generation != 4 || len(receipt.TargetPaths) != 1 || receipt.TargetPaths[0] != receipt.Receipt.ClaimTokenPath {
		t.Fatalf("receipt=%+v", receipt)
	}
	second, err := NewReseedArtifacts(nil).Prepare(t.Context(), record)
	if err != nil || second.TokenSHA256 == receipt.TokenSHA256 {
		t.Fatalf("re-prepare must mint a fresh token: err=%v", err)
	}
}

func TestReseedArtifactsPrepareRemovesTokenWhenOrcaOwnerWriterIsMissingOrFails(t *testing.T) {
	root := t.TempDir()
	record := orcaReseedArtifactRecord(root, 2)
	if _, err := NewReseedArtifacts(nil).Prepare(t.Context(), record); err == nil || !strings.Contains(err.Error(), "owner artifact writer is required") {
		t.Fatalf("missing writer err=%v", err)
	}
	if _, err := os.Lstat(reseedTokenPath(record)); !os.IsNotExist(err) {
		t.Fatalf("token left behind after missing writer: %v", err)
	}

	foreign := filepath.Join(root, "elsewhere.md")
	outside := NewReseedArtifacts(func(context.Context, leasecontract.Record) (leasecontract.ReseedReceipt, error) {
		return leasecontract.ReseedReceipt{OwnerPromptPath: foreign}, nil
	})
	if _, err := outside.Prepare(t.Context(), record); err == nil || !strings.Contains(err.Error(), "outside the target generation") {
		t.Fatalf("outside receipt err=%v", err)
	}
	if _, err := os.Lstat(reseedTokenPath(record)); !os.IsNotExist(err) {
		t.Fatalf("token left behind after rejected receipt: %v", err)
	}
}

func TestReseedArtifactsCleanupSupersededRemovesOnlyTheTokenOfThatGeneration(t *testing.T) {
	root := t.TempDir()
	old, current := reseedArtifactRecord(root, 1), reseedArtifactRecord(root, 2)
	artifacts := NewReseedArtifacts(nil)
	oldReceipt, err := artifacts.Prepare(t.Context(), old)
	if err != nil {
		t.Fatal(err)
	}
	currentReceipt, err := artifacts.Prepare(t.Context(), current)
	if err != nil {
		t.Fatal(err)
	}
	if err := artifacts.CleanupSuperseded(t.Context(), old); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(oldReceipt.Receipt.ClaimTokenPath); !os.IsNotExist(err) {
		t.Fatalf("superseded token remained: %v", err)
	}
	if _, err := os.Lstat(currentReceipt.Receipt.ClaimTokenPath); err != nil {
		t.Fatalf("current generation token must survive: %v", err)
	}
	if err := artifacts.CleanupSuperseded(t.Context(), old); err != nil {
		t.Fatalf("cleanup of already-removed token must be idempotent: %v", err)
	}
}

func TestValidateReseedReceiptTargetsRejectsEmptyAndForeignPaths(t *testing.T) {
	if err := validateReseedReceiptTargets(leasecontract.ReseedReceipt{}, nil); err == nil || !strings.Contains(err.Error(), "no target generation paths") {
		t.Fatalf("empty targets err=%v", err)
	}
	targets := []string{"/a/token", "/a/context", "/a/prompt"}
	if err := validateReseedReceiptTargets(leasecontract.ReseedReceipt{ClaimTokenPath: "/a/token", ContextPacketPath: "/a/context"}, targets); err != nil {
		t.Fatalf("in-target receipt rejected: %v", err)
	}
	for name, receipt := range map[string]leasecontract.ReseedReceipt{
		"token":   {ClaimTokenPath: "/b/token"},
		"context": {ContextPacketPath: "/b/context"},
		"prompt":  {OwnerPromptPath: "/b/prompt"},
	} {
		if err := validateReseedReceiptTargets(receipt, targets); err == nil || !strings.Contains(err.Error(), "outside the target generation") {
			t.Fatalf("%s err=%v", name, err)
		}
	}
}
