package issueopslease

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func TestReseedArtifactsRequireAnExecution(t *testing.T) {
	artifacts := NewReseedArtifacts()
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
	receipt, err := NewReseedArtifacts().Prepare(t.Context(), record)
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
	second, err := NewReseedArtifacts().Prepare(t.Context(), record)
	if err != nil || second.TokenSHA256 == receipt.TokenSHA256 {
		t.Fatalf("re-prepare must mint a fresh token: err=%v", err)
	}
}

func TestReseedArtifactsCleanupSupersededRemovesOnlyTheTokenOfThatGeneration(t *testing.T) {
	root := t.TempDir()
	old, current := reseedArtifactRecord(root, 1), reseedArtifactRecord(root, 2)
	artifacts := NewReseedArtifacts()
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
	targets := []string{"/a/token"}
	if err := validateReseedReceiptTargets(leasecontract.ReseedReceipt{ClaimTokenPath: "/a/token"}, targets); err != nil {
		t.Fatalf("in-target receipt rejected: %v", err)
	}
	if err := validateReseedReceiptTargets(leasecontract.ReseedReceipt{ClaimTokenPath: "/b/token"}, targets); err == nil || !strings.Contains(err.Error(), "outside the target generation") {
		t.Fatalf("foreign token err=%v", err)
	}
}

func TestReseedArtifactsCleanupSupersededRejectsMissingOrSymlinkedRoot(t *testing.T) {
	artifacts := NewReseedArtifacts()

	missing := reseedArtifactRecord(filepath.Join(t.TempDir(), "absent"), 1)
	if err := artifacts.CleanupSuperseded(t.Context(), missing); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing root err=%v, want not-exist", err)
	}

	real := t.TempDir()
	linked := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, linked); err != nil {
		t.Fatal(err)
	}
	record := reseedArtifactRecord(linked, 1)
	if err := os.MkdirAll(filepath.Dir(reseedTokenPath(reseedArtifactRecord(real, 1))), 0o700); err != nil {
		t.Fatal(err)
	}
	target := reseedTokenPath(reseedArtifactRecord(real, 1))
	if err := os.WriteFile(target, []byte("token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifacts.CleanupSuperseded(t.Context(), record); err == nil || !strings.Contains(err.Error(), "only real directories") {
		t.Fatalf("symlinked root err=%v", err)
	}
	if _, err := os.Lstat(target); err != nil {
		t.Fatalf("token behind symlinked root was deleted: %v", err)
	}
}
