package issueops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
)

// CleanupAbandonFailureEvidence contains path identity and seal checks performed
// outside the domain; resolving symlinks and encoding evidence are not policy.
type CleanupAbandonFailureEvidence struct {
	SameWorktree bool
	SealMatches  bool
}

// CleanupAbandonReasonLimit는 --reason 상한이다. 감사 문자열의 표현력 한계가
// 아니라, lease 가드의 exact-command 파싱(commandparse)이 명령 전체를 토큰
// 단위로 재구성한다는 사실에서 오는 보수적 여유값이다.
const CleanupAbandonReasonLimit = 512

// cleanupAbandonReasonForbidden은 exact-command 파서가 "활성 셸 구성"으로
// 판정하는 문자 집합이다. 이 문자가 --reason에 들어가면 명령이 가드 단계에서
// 거부되므로, core가 먼저 거부해 진단을 앞당긴다(사유 없는 unsafe_mutation
// 거부보다 reason_required가 훨씬 읽기 쉽다).
const cleanupAbandonReasonForbidden = "\"'`$\\|&;<>()*?~"

func CleanupAbandonRemoteFlags(req issueops.CleanupAbandonRequest) string {
	flags := ""
	if req.ClosePR {
		flags += " --close-pr"
	}
	if req.CloseIssue {
		flags += " --close-issue"
	}
	if req.DeleteRemoteBranch {
		flags += " --delete-remote-branch"
	}
	return flags
}

func CleanupAbandonPreviewCommand(id, reason string, req issueops.CleanupAbandonRequest) string {
	return fmt.Sprintf("issueops cleanup abandon --id %s --reason %q%s --preview --json",
		id, reason, CleanupAbandonRemoteFlags(req))
}

func CleanupAbandonPartialBranchRetry(record issueops.IssueOpsRecord, inventory issueops.CleanupAbandonInventory, evidence CleanupAbandonFailureEvidence) bool {
	branchPresent := inventory.BranchOID != ""
	failure := record.CleanupAbandonFailure
	return !inventory.WorktreePresent && branchPresent && failure != nil &&
		(failure.Step == "applying" || failure.Step == "branch_delete") &&
		evidence.SameWorktree &&
		failure.Branch == inventory.Branch && failure.BranchOID == inventory.BranchOID &&
		failure.WorktreeHead == inventory.BranchOID
}

func CleanupAbandonFailureInventoryMatches(record issueops.IssueOpsRecord, inventory issueops.CleanupAbandonInventory, evidence CleanupAbandonFailureEvidence) bool {
	failure := record.CleanupAbandonFailure
	if failure == nil {
		return true
	}
	if !cleanupAbandonReceiptSealed(failure, inventory, evidence) {
		return false
	}
	m := newCleanupAbandonReceiptMatcher(failure, inventory)
	if !m.pairedOIDsAgree() {
		return false
	}
	switch failure.Step {
	case "applying":
		return m.applyingMatches()
	case "close_pr", "close_issue", "remote_branch_delete":
		return m.remoteStepMatches()
	case "workspace_processes_stop", "worktree_remove":
		return m.worktreeStepMatches()
	case "branch_delete":
		return m.branchDeleteMatches()
	case "record_delete":
		return m.recordDeleteMatches()
	}
	return false
}

// cleanupAbandonReceiptSealed는 receipt의 지문들이 형식에 맞고 봉인·worktree
// 증거와 branch 이름이 현재 inventory와 일치하는지 확인한다.
func cleanupAbandonReceiptSealed(failure *issueops.IssueOpsCleanupAbandonFailure, inventory issueops.CleanupAbandonInventory, evidence CleanupAbandonFailureEvidence) bool {
	return cleanupAbandonValidFingerprint(failure.Fingerprint) &&
		cleanupAbandonValidFingerprint(failure.RecordSHA) &&
		cleanupAbandonValidFingerprint(failure.InventorySHA256) &&
		evidence.SealMatches && failure.Branch == inventory.Branch &&
		evidence.SameWorktree
}

// cleanupAbandonReceiptMatcher는 receipt가 기록한 원래 자원 모양과 현재
// inventory를 비교한다.
type cleanupAbandonReceiptMatcher struct {
	failure       *issueops.IssueOpsCleanupAbandonFailure
	inventory     issueops.CleanupAbandonInventory
	origWorktree  bool
	origBranch    bool
	branchPresent bool
}

func newCleanupAbandonReceiptMatcher(failure *issueops.IssueOpsCleanupAbandonFailure, inventory issueops.CleanupAbandonInventory) cleanupAbandonReceiptMatcher {
	return cleanupAbandonReceiptMatcher{
		failure: failure, inventory: inventory,
		origWorktree:  failure.WorktreeHead != "",
		origBranch:    failure.BranchOID != "",
		branchPresent: inventory.BranchOID != "",
	}
}

// 남아 있는 자원은 receipt의 OID와 같아야 하고, 사라진 자원은 원래 없었거나
// apply가 그 단계까지 갔을 때만 사라질 수 있다. 삭제 순서는 worktree → branch다.
func (m cleanupAbandonReceiptMatcher) worktreeMatches(mayBeRemoved bool) bool {
	if m.inventory.WorktreePresent {
		return m.origWorktree && m.inventory.WorktreeHead == m.failure.WorktreeHead
	}
	return !m.origWorktree || mayBeRemoved
}

func (m cleanupAbandonReceiptMatcher) branchMatches(mayBeRemoved bool) bool {
	if m.branchPresent {
		return m.origBranch && m.inventory.BranchOID == m.failure.BranchOID
	}
	return !m.origBranch || mayBeRemoved
}

// receipt는 arm 시점의 자원 모양을 담는다: paired(둘 다), worktree-only,
// branch-only(#433 비대칭 잔여), absent. 둘 다 있었다면 같은 OID여야 한다.
func (m cleanupAbandonReceiptMatcher) pairedOIDsAgree() bool {
	return !(m.origWorktree && m.origBranch && m.failure.WorktreeHead != m.failure.BranchOID)
}

func (m cleanupAbandonReceiptMatcher) applyingMatches() bool {
	branchRemoved := m.origBranch && !m.branchPresent
	return m.worktreeMatches(true) && m.branchMatches(true) && !(branchRemoved && m.inventory.WorktreePresent)
}

// Remote effects run before any local removal. Only unchanged local resources may retry.
func (m cleanupAbandonReceiptMatcher) remoteStepMatches() bool {
	return m.worktreeMatches(false) && m.branchMatches(false)
}

// ①′/③ 실패는 아직 아무것도 지우지 않은 상태다.
func (m cleanupAbandonReceiptMatcher) worktreeStepMatches() bool {
	return m.origWorktree && m.inventory.WorktreePresent && m.worktreeMatches(false) && m.branchMatches(false)
}

// ④ 실패는 worktree 제거 뒤다. 다시 나타난 worktree는 외부 변경이므로 거부한다.
func (m cleanupAbandonReceiptMatcher) branchDeleteMatches() bool {
	return m.origBranch && m.branchPresent && !m.inventory.WorktreePresent && m.branchMatches(false)
}

func (m cleanupAbandonReceiptMatcher) recordDeleteMatches() bool {
	return !m.inventory.WorktreePresent && !m.branchPresent && m.worktreeMatches(true) && m.branchMatches(true)
}

func cleanupAbandonValidFingerprint(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func CleanupAbandonRemovalPlan(record issueops.IssueOpsRecord, inventory issueops.CleanupAbandonInventory) []string {
	plan := []string{}
	if inventory.WorktreePresent {
		plan = append(plan, "local_worktree:"+inventory.WorktreeRoot)
	}
	if inventory.BranchOID != "" {
		plan = append(plan, "local_branch:"+inventory.Branch+"@"+inventory.BranchOID)
	}
	plan = append(plan, "record:"+record.ID)
	for _, operationID := range CleanupAbandonIntentOperationIDs(record) {
		plan = append(plan, "intent:"+operationID)
	}
	return plan
}

func CleanupAbandonIntentOperationIDs(record issueops.IssueOpsRecord) []string {
	if record.Execution == nil {
		return nil
	}
	candidates := []string{}
	if record.Execution.Pending != nil {
		candidates = append(candidates, record.Execution.Pending.OperationID)
	}
	if record.Execution.Failure != nil {
		candidates = append(candidates, record.Execution.Failure.OperationID)
	}
	out := []string{}
	seen := map[string]bool{}
	for _, id := range candidates {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func CleanupAbandonUnresolvedChildren(record issueops.IssueOpsRecord, resolved map[string]bool) []string {
	var unresolved []string
	for _, child := range record.ChildCycles {
		id := strings.TrimSpace(child.CycleID)
		if id == "" || resolved[id] {
			continue
		}
		unresolved = append(unresolved, id)
	}
	issueURL := strings.TrimSpace(record.IssueURL)
	for _, link := range record.IssueLinks {
		if link.Type == "child" && strings.TrimSpace(link.CloseVerifiedAt) == "" &&
			(issueURL == "" || strings.TrimSpace(link.URL) != issueURL) {
			unresolved = append(unresolved, strings.TrimSpace(link.URL))
		}
	}
	return unresolved
}

func ValidateCleanupAbandonReason(reason string) error {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return fmt.Errorf("--reason is required")
	}
	if len(trimmed) > CleanupAbandonReasonLimit {
		return fmt.Errorf("--reason must not exceed %d bytes", CleanupAbandonReasonLimit)
	}
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("--reason must not contain control characters")
		}
		if strings.ContainsRune(cleanupAbandonReasonForbidden, r) {
			return fmt.Errorf("--reason must not contain %q; the lease guard parses this command exactly and rejects active shell characters", r)
		}
	}
	return nil
}

func CleanupAbandonTargets(record issueops.IssueOpsRecord) issueops.CleanupAbandonInventory {
	inventory := issueops.CleanupAbandonInventory{
		ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(record.Branch),
		Phase: string(record.Phase), LeaseStatus: "none",
	}
	if record.Execution != nil {
		inventory.LeaseStatus = string(record.Execution.Lease.Status)
		inventory.WorktreeRoot = strings.TrimSpace(record.Execution.Workspace.Root)
		if branch := strings.TrimSpace(record.Execution.Workspace.Branch); branch != "" {
			inventory.Branch = branch
		}
		if record.Execution.Pending != nil {
			inventory.PendingOperationID = strings.TrimSpace(record.Execution.Pending.OperationID)
		}
	}
	if inventory.WorktreeRoot == "" {
		inventory.WorktreeRoot = strings.TrimSpace(record.WorktreePath)
	}
	return inventory
}

// Only an observed done record resolves a child. Missing or unreadable child
// records provide no completion evidence.
func CleanupAbandonResolvedChildren(children []issueops.IssueOpsRecord) map[string]bool {
	resolved := map[string]bool{}
	for _, child := range children {
		if child.Phase == issueops.IssueOpsPhaseDone {
			resolved[child.ID] = true
		}
	}
	return resolved
}
