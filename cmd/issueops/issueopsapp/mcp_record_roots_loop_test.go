package issueopsapp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"issueops/cmd/issueops/mcpcli"
	statestore "issueops/internal/adapter/outbound/state"
	loopcontract "issueops/internal/contract/looprun"
)

func TestMCPRecordRootsLoopLookupReadsExistingRecordOnly(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	repo := gitRepoForHTTPTest(t)
	started, err := newLoopService().Start(context.Background(), loopcontract.StartLoopRequest{Repo: repo, Name: "record-roots", Goal: "lookup"})
	if err != nil {
		t.Fatalf("seed loop: %v", err)
	}
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(gone)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Getwd(); err == nil {
		t.Skip("platform still resolves a removed cwd")
	}
	roots := issueOpsMCPRecordRoots(issueOpsStateRoot())
	got, err := roots(t.Context(), mcpcli.RecordLoop, started.ID)
	if err != nil || len(got) != 1 || got[0] != started.Repo {
		t.Fatalf("loop roots = %v err=%v, want [%s]", got, err, started.Repo)
	}
}

func TestMCPRecordRootsUnknownLoopDoesNotCreateLoopStore(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	roots := issueOpsMCPRecordRoots(issueOpsStateRoot())
	_, err := roots(t.Context(), mcpcli.RecordLoop, "loop-000000000000")
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unknown loop err = %v, want not-exist", err)
	}
	if _, statErr := os.Stat(filepath.Join(statestore.StateDir(), "loop")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("record lookup created the loop store: %v", statErr)
	}
}
