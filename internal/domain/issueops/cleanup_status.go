package issueops

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	model "issueops/internal/contract/issueops"
)

type CleanupStatusObservation struct {
	WorktreeExists bool
	StatusCode     int
	StatusOutput   string
	StatusError    string
	Branch         string
	Remote         string
	RemoteCode     int
	RemoteOutput   string
	RemoteError    string
}

func hasCleanupMetadata(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && !strings.Contains(value, "\x00") {
			return true
		}
	}
	return false
}

func cleanupStatusSortedValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return slices.Compact(out)
}

func CleanupStatusNeedsMergeReadback(record model.IssueOpsRecord, requested bool) bool {
	return requested && record.Phase == model.IssueOpsPhaseDone && len(RemoteArtifactMissing(record)) == 0
}

func WithCleanupFinishEvidence(req model.CleanupFinishRequest, body, state string, artifact model.CleanupRemoteBranchArtifactHead) model.CleanupFinishRequest {
	req.CompletionReflected = strings.Contains(body, model.IssueBodyCompletionStartMarker)
	req.IssueClosed = strings.EqualFold(strings.TrimSpace(state), "closed")
	req.MergedBaseBranch = artifact.BaseRefName
	return req
}

func CleanupStatusFromFinish(structural model.IssueOpsCleanupStatus, result model.CleanupFinishResult) model.IssueOpsCleanupStatus {
	return FinalizeCleanupStatus(model.IssueOpsCleanupStatus{
		OK: true, Ready: result.OK && len(result.Missing) == 0, ID: result.ID, Merged: true,
		Missing:  append([]string(nil), result.Missing...),
		Warnings: CleanupStatusWarnings(result), WorktreePath: result.WorktreePath, Branch: result.Branch,
		RemoteArtifactURL: structural.RemoteArtifactURL,
	})
}

func RemoteArtifactMissing(record model.IssueOpsRecord) []string {
	if record.RemoteArtifact == nil {
		return []string{"remote_artifact"}
	}
	missing := []string{}
	if strings.TrimSpace(record.RemoteArtifact.Provider) == "" {
		missing = append(missing, "remote_artifact_provider")
	}
	if strings.TrimSpace(record.RemoteArtifact.Kind) == "" {
		missing = append(missing, "remote_artifact_kind")
	}
	if strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		missing = append(missing, "remote_artifact_url")
	}
	if !hasCleanupMetadata(record.RemoteArtifact.Labels) {
		missing = append(missing, "remote_artifact_labels")
	}
	if !hasCleanupMetadata(record.RemoteArtifact.Assignees) {
		missing = append(missing, "remote_artifact_assignees")
	}
	return cleanupStatusSortedValues(missing)
}

func BuildCleanupStatus(record model.IssueOpsRecord, req model.IssueOpsCleanupStatusRequest, facts CleanupStatusObservation) model.IssueOpsCleanupStatus {
	status := model.IssueOpsCleanupStatus{
		OK:           true,
		ID:           record.ID,
		Merged:       req.Merged,
		WorktreePath: strings.TrimSpace(record.WorktreePath),
		Branch:       strings.TrimSpace(record.Branch),
	}
	if record.RemoteArtifact != nil {
		status.RemoteArtifactURL = strings.TrimSpace(record.RemoteArtifact.URL)
	}
	if IssueOpsPhaseRank(record.Phase) < IssueOpsPhaseRank(model.IssueOpsPhasePR) {
		status.Missing = append(status.Missing, "pr_phase")
	}
	status.Missing = append(status.Missing, RemoteArtifactMissing(record)...)
	if !req.Merged {
		status.Missing = append(status.Missing, "remote_artifact_merged")
	}
	if hasUnverifiedChildClose(record) {
		status.Missing = append(status.Missing, "child_tasks_closed")
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		status.Missing = append(status.Missing, "worktree_path")
		return FinalizeCleanupStatus(status)
	}
	if !facts.WorktreeExists {
		status.Missing = append(status.Missing, "worktree_exists")
		return FinalizeCleanupStatus(status)
	}
	if code, out, stderr := facts.StatusCode, facts.StatusOutput, facts.StatusError; code != 0 {
		status.Missing = append(status.Missing, "worktree_git_status")
		if strings.TrimSpace(stderr) != "" {
			status.Warnings = append(status.Warnings, strings.TrimSpace(stderr))
		}
	} else if strings.TrimSpace(out) != "" {
		// worktree_clean: `missing`은 충족되지 않은 요구의 목록이므로 상태 차단도
		// 요구형으로 적는다. `worktree_dirty`처럼 차단 사실을 적으면 "dirty라는
		// 요구가 미충족"으로 읽힌다(#185). cleanup finish와 switch-mode 게이트가
		// 같은 극성을 쓴다.
		status.Missing = append(status.Missing, "worktree_clean")
	}
	actualBranch := strings.TrimSpace(facts.Branch)
	if actualBranch == "" {
		status.Missing = append(status.Missing, "branch")
	} else if strings.TrimSpace(record.Branch) != "" && actualBranch != strings.TrimSpace(record.Branch) {
		status.Missing = append(status.Missing, "branch_match")
	}
	remote := strings.TrimSpace(facts.Remote)
	if remote == "" {
		status.Missing = append(status.Missing, "remote_branch_check_unavailable")
	} else if actualBranch != "" {
		if code, out, stderr := facts.RemoteCode, facts.RemoteOutput, facts.RemoteError; code != 0 {
			status.Missing = append(status.Missing, "remote_branch_check_failed")
			if strings.TrimSpace(stderr) != "" {
				status.Warnings = append(status.Warnings, strings.TrimSpace(stderr))
			}
		} else if strings.TrimSpace(out) != "" {
			// remote_branch_absent: cleanup finish가 같은 상태에 쓰는 슬러그다.
			// 여기서 `remote_branch_present`를 쓰면 두 명령이 같은 상태를 반대로
			// 읽히게 보고하고, 운영자는 브랜치가 이미 없다고 판단해
			// `cleanup remote-branch`를 건너뛴다(#185, #181 정리에서 실측).
			//
			// execution sync-base의 동명 슬러그는 브랜치가 **있어야** 한다는 진짜
			// 요구이므로 별개다.
			status.Missing = append(status.Missing, "remote_branch_absent")
		}
	}
	return FinalizeCleanupStatus(status)
}

func hasUnverifiedChildClose(record model.IssueOpsRecord) bool {
	for _, link := range record.IssueLinks {
		if link.Type != "child" {
			continue
		}
		if strings.TrimSpace(link.CloseVerifiedAt) == "" {
			return true
		}
	}
	return false
}

// FinalizeCleanupStatus applies the cleanup status contract's stable sorting, readiness, and
// three-choice presentation to status assembled by either structural inspection
// or the cleanup finish readiness oracle.
func FinalizeCleanupStatus(status model.IssueOpsCleanupStatus) model.IssueOpsCleanupStatus {
	status.Missing = cleanupStatusSortedValues(status.Missing)
	status.Warnings = cleanupStatusSortedValues(status.Warnings)
	status.Ready = len(status.Missing) == 0
	if status.Ready {
		status.Choices = []string{
			"1. 정리 진행: merged PR/MR worktree와 local branch를 삭제합니다. (추천)",
			"2. 보류: worktree는 유지하고 나중에 확인합니다.",
			"3. 확장 정리: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.",
		}
	} else {
		status.Choices = []string{
			"1. 차단 해소: missing 항목의 merge/worktree/remote branch 증거를 먼저 보강합니다. (추천)",
			"2. 보류: worktree는 유지하고 나중에 다시 확인합니다.",
			"3. 확장 점검: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.",
		}
	}
	return status
}

func CleanupStatusWarnings(result model.CleanupFinishResult) []string {
	warnings := make([]string, 0, len(result.WorkspaceProcesses)+1)
	for _, process := range result.WorkspaceProcesses {
		warnings = append(warnings, fmt.Sprintf("%d:%s:%s", process.PID, process.Command, process.StartedAt))
	}
	if len(result.WorkspaceProcesses) > 0 || len(result.OrcaTerminals) > 0 {
		warnings = append(warnings, fmt.Sprintf("apply가 프로세스 %d개와 Orca 터미널 %d개를 종료합니다", len(result.WorkspaceProcesses), len(result.OrcaTerminals)))
	}
	return warnings
}
