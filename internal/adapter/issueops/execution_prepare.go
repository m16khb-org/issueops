package issueops

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"issueops/internal/adapter/issueops/pathutil"
	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func newExecutionOperationID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func executionWorkspaceRequest(record issueops.IssueOpsRecord, confirm bool) (port.ExecutionWorkspaceRequest, error) {
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseSHA) == "" {
		return port.ExecutionWorkspaceRequest{}, fmt.Errorf("verified branch preparation with base_sha is required")
	}
	branch := strings.TrimSpace(record.Branch)
	leaf := strings.ReplaceAll(branch, "/", "-")
	if leaf == "" || leaf == "." || leaf == ".." {
		return port.ExecutionWorkspaceRequest{}, fmt.Errorf("execution branch is invalid")
	}
	parentWorktree := strings.TrimSpace(record.BranchPrepare.ParentWorktree)
	hasDelegatedParent := record.Delegation != nil && strings.TrimSpace(record.Delegation.ParentCycleID) != ""
	if parentWorktree != "" || hasDelegatedParent {
		parentLeaf := strings.ReplaceAll(strings.TrimSpace(record.BranchPrepare.BaseBranch), "/", "-")
		if parentLeaf == "" || parentLeaf == "." || parentLeaf == ".." {
			return port.ExecutionWorkspaceRequest{}, fmt.Errorf("parent execution base branch is invalid")
		}
		expectedParent := filepath.Join(record.Repo+".worktrees", parentLeaf)
		if parentWorktree == "" {
			parentWorktree = expectedParent
		} else {
			parentWorktree = filepath.Clean(parentWorktree)
			if !samePath(parentWorktree, expectedParent) {
				return port.ExecutionWorkspaceRequest{}, fmt.Errorf(
					"parent_worktree %q does not match canonical parent worktree %q",
					parentWorktree, expectedParent,
				)
			}
		}
	}
	return port.ExecutionWorkspaceRequest{
		LifecycleID: record.ID, SourceRoot: record.Repo, Root: filepath.Join(record.Repo+".worktrees", leaf),
		Branch: branch, BaseBranch: strings.TrimSpace(record.BranchPrepare.BaseBranch),
		BaseHead: strings.TrimSpace(record.BranchPrepare.BaseSHA), ParentWorktree: parentWorktree, Confirm: confirm,
	}, nil
}

// ensureExecutionRootUnclaimed는 다른 lifecycle 레코드가 이미 주장한 canonical
// worktree root를 fail-closed로 거부한다. leaf 파생(strings.ReplaceAll(branch,
// "/", "-"))이 비단사라 "72/fix"와 "72-fix"가 같은 root로 접히지만 lifecycle ID는
// 브랜치로 해시되어 서로 다른 레코드가 된다 — 두 사이클이 같은 워크트리를
// 소유하는 불변식 위반을 prepare 입구에서 막는다.
//
// 스캔은 phase·lease 상태와 무관한 전 레코드다: cleanup finish가 워크트리를
// 제거한 뒤에야 레코드를 삭제하므로 레코드의 존재가 곧 root 소유권이다.
// WorktreePath와 Execution.Workspace.Root의 합집합을 보는 이유는 linking 경로가
// Execution 없이 WorktreePath만 채울 수 있고 그 경로에는 레코드 간 유일성 검증이
// 없기 때문이다. 읽기 오류는 통과가 아니라 거부다 — 손상된 레코드가 소유권
// 주장을 조용히 잃어서는 안 된다.
func ensureExecutionRootUnclaimed(stateRoot, selfID, root string) error {
	target := pathutil.CleanAbsPath(root)
	if target == "" {
		return fmt.Errorf("canonical worktree root is required")
	}
	self := strings.TrimSpace(selfID)
	records, err := ScanIssueOps(stateRoot)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.ID == self {
			continue
		}
		for _, claimed := range []string{record.WorktreePath, executionRecordWorkspaceRoot(record)} {
			if strings.TrimSpace(claimed) == "" || pathutil.CleanAbsPath(claimed) != target {
				continue
			}
			return fmt.Errorf(
				"canonical worktree %s는 이미 lifecycle %s(브랜치 %s)가 선점했다; 먼저 그 사이클을 정리하라: issueops cleanup finish --id %s --preview --json",
				target, record.ID, strings.TrimSpace(record.Branch), record.ID,
			)
		}
	}
	return nil
}

func executionRecordWorkspaceRoot(record issueops.IssueOpsRecord) string {
	if record.Execution == nil {
		return ""
	}
	return record.Execution.Workspace.Root
}

var issueOpsOwnerReportLabels = []string{
	"Status",
	"Lifecycle",
	"Mode/host/model",
	"Worktree/branch/final HEAD",
	"Lease generation/completion",
	"Issue/packet digests",
	"Commits",
	"Changed files",
	"Acceptance evidence",
	"Verification",
	"AI-slop clean",
	"Draft PR/MR",
	"Deviations",
	"Blockers",
}

func renderExecutionOwnerReportContract(record issueops.IssueOpsRecord, req ExecutionPrepareRequest) string {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if record.Execution != nil {
		mode = string(record.Execution.Mode)
	}
	values := []string{
		"completed | blocked",
		record.ID,
		mode + " / " + strings.ToLower(strings.TrimSpace(req.OwnerHost)) + " / " + strings.TrimSpace(req.OwnerModel) + " (" + strings.TrimSpace(req.OwnerEffort) + ")",
		"<exact values>",
		"<generation + receipt or blocker>",
		"<verified | drift, 원문 secret 없음>",
		"<ordered SHA + subject>",
		"<exact paths>",
		"<AC-ID → test/command/result mapping>",
		"<every command + PASS/FAIL>",
		"<removed duplication/legacy/noise or none>",
		"<URL or none>",
		"<issue-vs-code mismatch with file:line evidence or none>",
		"<exact state/error/next command or none>",
	}
	lines := []string{"## IssueOps v1 Owner Report"}
	for index, label := range issueOpsOwnerReportLabels {
		lines = append(lines, "- "+label+": "+values[index])
	}
	return strings.Join(lines, "\n")
}

func executionNow(now func() time.Time) string {
	if now == nil {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return now().UTC().Format(time.RFC3339Nano)
}
