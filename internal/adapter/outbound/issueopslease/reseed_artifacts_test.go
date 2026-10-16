package issueopslease

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func TestReseedArtifactsPrepareAndRollbackOnlyTargetGeneration(t *testing.T) {
	root := t.TempDir()
	record := reseedArtifactRecord(root, 2)
	artifacts := NewReseedArtifacts()
	receipt, err := artifacts.Prepare(context.Background(), record)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	info, err := os.Stat(receipt.Receipt.ClaimTokenPath)
	if err != nil || info.Mode().Perm() != 0o600 || !info.Mode().IsRegular() {
		t.Fatalf("token info=%v err=%v", info, err)
	}
	if receipt.TokenSHA256 == "" {
		t.Fatal("token hash is empty")
	}
	if err := artifacts.Rollback(context.Background(), receipt); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := os.Stat(receipt.Receipt.ClaimTokenPath); !os.IsNotExist(err) {
		t.Fatalf("target token remained after rollback: %v", err)
	}
}

func TestReseedArtifactsReclaimsExistingTargetToken(t *testing.T) {
	root := t.TempDir()
	record := reseedArtifactRecord(root, 2)
	path := reseedTokenPath(record)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt, err := NewReseedArtifacts().Prepare(context.Background(), record)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	data, err := os.ReadFile(receipt.Receipt.ClaimTokenPath)
	if err != nil || string(data) == "existing\n" {
		t.Fatalf("token=%q err=%v", data, err)
	}
}

func TestReseedArtifactsRollbackRejectsOutsideReceiptPathAndCleansOwnedTargets(t *testing.T) {
	root := t.TempDir()
	record := reseedArtifactRecord(root, 2)
	artifacts := NewReseedArtifacts()
	receipt, err := artifacts.Prepare(context.Background(), record)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("must survive"), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt.Receipt.ClaimTokenPath = outside
	if err := artifacts.Rollback(context.Background(), receipt); err == nil || !strings.Contains(err.Error(), "outside the target generation") {
		t.Fatalf("rollback error=%v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("rollback deleted outside receipt path: %v", err)
	}
	if _, err := os.Stat(reseedTokenPath(record)); !os.IsNotExist(err) {
		t.Fatalf("rollback did not clean owned target token: %v", err)
	}
}

func TestReseedArtifactsRollbackRejectsAncestorSymlinkWithoutDeletingOutsideTarget(t *testing.T) {
	root := t.TempDir()
	record := reseedArtifactRecord(root, 2)
	artifacts := NewReseedArtifacts()
	receipt, err := artifacts.Prepare(context.Background(), record)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	outside := t.TempDir()
	outsideTargets := reseedReplaceArtifactDirectoryWithSymlink(t, root, outside, receipt.TargetPaths)
	if err := artifacts.Rollback(context.Background(), receipt); err == nil || !strings.Contains(err.Error(), "real directories") {
		t.Fatalf("rollback error=%v", err)
	}
	for _, path := range outsideTargets {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("rollback deleted symlinked outside target %s: %v", path, err)
		}
	}
}

func TestReseedArtifactsCleanupSupersededRejectsAncestorSymlink(t *testing.T) {
	root := t.TempDir()
	record := reseedArtifactRecord(root, 2)
	artifacts := NewReseedArtifacts()
	if _, err := artifacts.Prepare(context.Background(), record); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	outsideTargets := reseedReplaceArtifactDirectoryWithSymlink(t, root, t.TempDir(), []string{reseedTokenPath(record)})
	if err := artifacts.CleanupSuperseded(context.Background(), record); err == nil || !strings.Contains(err.Error(), "real directories") {
		t.Fatalf("cleanup superseded error=%v", err)
	}
	if _, err := os.Stat(outsideTargets[0]); err != nil {
		t.Fatalf("cleanup superseded deleted symlinked outside target: %v", err)
	}
}

func reseedReplaceArtifactDirectoryWithSymlink(t *testing.T, root, outside string, targets []string) []string {
	t.Helper()
	artifactDir := filepath.Join(root, ".issueops")
	if err := os.RemoveAll(artifactDir); err != nil {
		t.Fatalf("remove artifact directory: %v", err)
	}
	if err := os.Symlink(outside, artifactDir); err != nil {
		t.Fatalf("symlink artifact directory: %v", err)
	}
	outsideTargets := make([]string, 0, len(targets))
	for _, target := range targets {
		rel, err := filepath.Rel(artifactDir, target)
		if err != nil {
			t.Fatalf("relative target: %v", err)
		}
		outsideTarget := filepath.Join(outside, rel)
		if err := os.MkdirAll(filepath.Dir(outsideTarget), 0o700); err != nil {
			t.Fatalf("create outside target directory: %v", err)
		}
		if err := os.WriteFile(outsideTarget, []byte("must survive"), 0o600); err != nil {
			t.Fatalf("create outside target: %v", err)
		}
		outsideTargets = append(outsideTargets, outsideTarget)
	}
	return outsideTargets
}

func reseedArtifactRecord(root string, generation uint64) leasecontract.Record {
	return leasecontract.Record{SchemaVersion: leasecontract.SchemaVersion, ID: "io-reseed-artifact", Execution: &leasecontract.Execution{Workspace: leasecontract.Workspace{SourceRoot: root + "/source", Root: root, Branch: "branch", BaseHead: "base", LinkedAt: "2026-07-30T09:00:00Z"}, Lease: leasecontract.Lease{Generation: generation, Status: "claimable"}}}
}
