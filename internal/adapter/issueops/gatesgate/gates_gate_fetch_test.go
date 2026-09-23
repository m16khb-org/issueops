package gatesgate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
)

// pr 진입은 upstream을 한 번만 fetch한다. core가 span 밖에서 fetch한 결과로 판정하므로
// guard가 core readiness를 다시 계산하면 같은 fetch가 한 번 더 일어난다.
func TestAdvancePhaseToPRFetchesUpstreamOnce(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := readyGatesGateRecord(t)
	regressed := record
	regressed.Phase = issueopscontract.IssueOpsPhaseImplement
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), regressed); err != nil {
		t.Fatal(err)
	}
	writeGatesLedger(t, record.Repo, "- [x] G1: done\n  EVIDENCE: measured\n")
	log := installFetchCountingGit(t)

	if _, err := AdvancePhaseWithActor(issueops.IssueOpsStateRoot(), record.ID, "pr", issueops.IssueOpsActor{Host: "codex"}); err != nil {
		t.Fatalf("pr entry with met gates must pass: %v", err)
	}
	data, err := os.ReadFile(log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if fetches := strings.Count(string(data), "fetch\n"); fetches != 1 {
		t.Fatalf("pr entry must fetch upstream exactly once, fetched %d times", fetches)
	}
}

// installFetchCountingGit은 PATH 앞에 fetch 호출만 기록하는 git을 둔다.
func installFetchCountingGit(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "fetch.log")
	script := "#!/bin/sh\ncase \"$1\" in fetch) echo fetch >> '" + log + "';; esac\nexec '" + realGit + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}
