package issueopslease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"issueops/internal/adapter/outbound/gitraw"
	"issueops/internal/adapter/outbound/workspacefingerprint"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type ReseedInventory struct {
	inspect leaseapp.ProcessInspector
}

func NewReseedInventory(inspect leaseapp.ProcessInspector) *ReseedInventory {
	return &ReseedInventory{inspect: inspect}
}

func (a *ReseedInventory) Observe(ctx context.Context, record leasecontract.Record, requester leasedomain.Actor) (string, error) {
	if a == nil || a.inspect == nil || record.Execution == nil {
		return "", fmt.Errorf("reseed inventory requires an execution and process inspector")
	}
	snapshot, err := reseedWorkspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", err
	}
	processStatus := "none"
	if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
		processStatus, _, err = a.inspect(ctx, leasedomain.ProcessReceipt{PID: holder.SessionProcess.PID, StartedAt: holder.SessionProcess.StartedAt, Executable: holder.SessionProcess.Executable})
		if err != nil {
			return "", err
		}
	}
	payload := struct {
		ID         string               `json:"id"`
		Generation uint64               `json:"generation"`
		Status     string               `json:"status"`
		Holder     *leasecontract.Actor `json:"holder,omitempty"`
		Requester  leasecontract.Actor  `json:"requester"`
		Process    string               `json:"process_status"`
		Snapshot   string               `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, record.Execution.Lease.Status, record.Execution.Lease.Holder, reseedInventoryActor(requester), processStatus, snapshot}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func reseedInventoryActor(actor leasedomain.Actor) leasecontract.Actor {
	result := leasecontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.Process != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{PID: actor.Process.PID, StartedAt: actor.Process.StartedAt, Executable: actor.Process.Executable}
	}
	return result
}

func reseedWorkspaceSnapshot(workspace leasecontract.Workspace) (string, error) {
	if err := reseedRequireCanonicalWorktree(workspace); err != nil {
		return "", err
	}
	head, err := reseedGitOutput(workspace.Root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commonDir, err := reseedGitOutput(workspace.Root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	indexBytes, err := reseedIndexBytes(workspace.Root)
	if err != nil {
		return "", err
	}
	tracked, staged, untracked, err := workspacefingerprint.ReadDiffs(gitraw.Run, workspace.Root)
	if err != nil {
		return "", err
	}
	return workspacefingerprint.Digest(workspacefingerprint.Inputs{
		Root: workspace.Root, CommonDir: commonDir, Branch: workspace.Branch, Head: head,
		Index: indexBytes, Tracked: tracked, Staged: staged, Untracked: untracked,
	})
}

// reseedRequireCanonicalWorktree는 작업공간 루트가 실제 디렉터리이고 Git top-level 및 현재 브랜치와 일치하는지 확인한다.
func reseedRequireCanonicalWorktree(workspace leasecontract.Workspace) error {
	info, err := os.Lstat(workspace.Root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("canonical worktree must be a real directory")
	}
	top, err := reseedGitOutput(workspace.Root, "rev-parse", "--show-toplevel")
	if err != nil || !(FilesystemPathMatcher{}).Matches(top, workspace.Root) {
		return fmt.Errorf("canonical worktree root does not match Git top-level")
	}
	branch, err := reseedGitOutput(workspace.Root, "branch", "--show-current")
	if err != nil || branch != workspace.Branch {
		return fmt.Errorf("canonical worktree branch mismatch")
	}
	return nil
}

// reseedIndexBytes는 Git index 파일 내용을 읽는다. 파일이 없으면 빈 값을 돌려준다.
func reseedIndexBytes(root string) ([]byte, error) {
	indexPath, err := reseedGitOutput(root, "rev-parse", "--git-path", "index")
	if err != nil {
		return nil, err
	}
	return workspacefingerprint.ReadIndex(root, indexPath)
}

func reseedGitOutput(root string, args ...string) (string, error) {
	code, stdout, stderr := gitraw.Run(root, args...)
	if code != 0 {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), stderr)
	}
	return strings.TrimSpace(stdout), nil
}
