package issueops

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	model "issueops/internal/contract/issueops"
)

// CleanupStatusObservation 타입은 cleanup status가 도메인 밖에서 관측한 사실이다. worktree 존재
// 여부, `git status`와 원격 브랜치 점검의 종료 코드·출력·오류, 실제 체크아웃 브랜치를 담는다.
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

// CleanupStatusNeedsMergeReadback 함수는 호출자가 merge 확인을 요청했고 사이클이 done이며 원격 산출물
// 메타데이터가 모두 갖춰졌을 때만 참이다. 이때만 provider에서 merge 상태를 다시 읽을 가치가 있다.
func CleanupStatusNeedsMergeReadback(record model.Record, requested bool) bool {
	return requested && record.Phase == model.PhaseDone && len(RemoteArtifactMissing(record)) == 0
}

// WithCleanupFinishEvidence 함수는 이슈 본문, 이슈 상태, merge된 PR/MR 정보에서 cleanup
// finish 요청의 증거 필드를 채운다. 본문에 완료 반영 시작 마커가 있으면 CompletionReflected, 상태가
// closed이면 IssueClosed, 산출물의 base ref는 MergedBaseBranch가 된다.
func WithCleanupFinishEvidence(req model.CleanupFinishRequest, body, state string, artifact model.CleanupRemoteBranchArtifactHead) model.CleanupFinishRequest {
	req.CompletionReflected = strings.Contains(body, model.IssueBodyCompletionStartMarker)
	req.IssueClosed = strings.EqualFold(strings.TrimSpace(state), "closed")
	req.MergedBaseBranch = artifact.BaseRefName
	return req
}

// CleanupStatusFromFinish 함수는 cleanup finish 미리보기 결과를 cleanup status 계약으로
// 옮긴다. 구조 점검에서 얻은 원격 산출물 URL을 유지하고, 준비되었으면 finish 미리보기 명령을, 아니면 첫 미충족 항목의 해소
// 명령을 NextCommand로 둔다.
func CleanupStatusFromFinish(structural model.CleanupStatus, result model.CleanupFinishResult) model.CleanupStatus {
	status := finalizeCleanupStatus(model.CleanupStatus{
		OK: true, Ready: result.OK && len(result.Missing) == 0, ID: result.ID, Merged: true,
		Missing:  append([]string(nil), result.Missing...),
		Warnings: CleanupStatusWarnings(result), WorktreePath: result.WorktreePath, Branch: result.Branch,
		RemoteArtifactURL: structural.RemoteArtifactURL,
	})
	// QA: status listed completion_reflected and issue_closed without a next
	// step; the agent had to find reflect-completion → close-issue → finish
	// in the skill. The finish preview's remedy chain already orders them.
	if status.Ready {
		status.NextCommand = fmt.Sprintf("issueops cleanup finish --id %s --preview --json", status.ID)
	} else {
		status.NextCommand = CleanupFinishRemedyCommand(status.ID, status.Missing)
	}
	return status
}

// RemoteArtifactMissing 함수는 기록된 원격 산출물에서 비어 있는 필드(provider, kind, URL,
// labels, assignees)를 정렬된 슬러그로 돌려준다. 산출물이 아예 없으면 `remote_artifact` 하나다.
func RemoteArtifactMissing(record model.Record) []string {
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

// BuildCleanupStatus 함수는 record의 구조와 관측된 worktree·브랜치 사실로 cleanup status를
// 만든다. 미충족 항목은 PR 단계, 원격 산출물, merge 주장, 자식 작업 종료, worktree 경로·존재·청결, 브랜치 일치,
// 원격 브랜치 점검 순서로 모으고 마지막에 정렬한다.
func BuildCleanupStatus(record model.Record, req model.CleanupStatusRequest, facts CleanupStatusObservation) model.CleanupStatus {
	status := model.CleanupStatus{
		OK:           true,
		ID:           record.ID,
		Merged:       req.Merged,
		WorktreePath: strings.TrimSpace(record.WorktreePath),
		Branch:       strings.TrimSpace(record.Branch),
	}
	if record.RemoteArtifact != nil {
		status.RemoteArtifactURL = strings.TrimSpace(record.RemoteArtifact.URL)
	}
	if PhaseRank(record.Phase) < PhaseRank(model.PhasePR) {
		status.Missing = append(status.Missing, "pr_phase")
	}
	status.Missing = append(status.Missing, RemoteArtifactMissing(record)...)
	if !req.Merged {
		// QA: an agent read this key next to merge-pr's already_merged:true
		// as "the merge was not recorded"; status does not read the PR.
		status.Missing = append(status.Missing, "remote_artifact_merged")
		status.Warnings = append(status.Warnings, "remote_artifact_merged is the caller's --merged assertion (status does not read the PR): re-run with --merged once the PR/MR reads back merged, e.g. remote merge-pr answers already_merged")
	}
	if hasUnverifiedChildClose(record) {
		status.Missing = append(status.Missing, "child_tasks_closed")
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		status.Missing = append(status.Missing, "worktree_path")
		return finalizeCleanupStatus(status)
	}
	if !facts.WorktreeExists {
		status.Missing = append(status.Missing, "worktree_exists")
		return finalizeCleanupStatus(status)
	}
	appendWorktreeStatusMissing(&status, facts)
	actualBranch := strings.TrimSpace(facts.Branch)
	appendBranchMissing(&status, record, actualBranch)
	appendRemoteBranchMissing(&status, facts, actualBranch)
	return finalizeCleanupStatus(status)
}

// appendWorktreeStatusMissing는 `git status` 관측 결과를 missing/warnings에 반영한다.
func appendWorktreeStatusMissing(status *model.CleanupStatus, facts CleanupStatusObservation) {
	if code, out, stderr := facts.StatusCode, facts.StatusOutput, facts.StatusError; code != 0 {
		status.Missing = append(status.Missing, "worktree_git_status")
		if strings.TrimSpace(stderr) != "" {
			status.Warnings = append(status.Warnings, strings.TrimSpace(stderr))
		}
	} else if strings.TrimSpace(out) != "" {
		// worktree_clean: `missing`은 충족되지 않은 요구의 목록이므로 상태 차단도
		// 요구형으로 적는다. `worktree_dirty`처럼 차단 사실을 적으면 "dirty라는
		// 요구가 미충족"으로 읽힌다(#185). cleanup finish 게이트가
		// 같은 극성을 쓴다.
		status.Missing = append(status.Missing, "worktree_clean")
	}
}

// appendBranchMissing는 실제 체크아웃 브랜치가 비었거나 기록된 브랜치와 다른 경우를 missing에 반영한다.
func appendBranchMissing(status *model.CleanupStatus, record model.Record, actualBranch string) {
	if actualBranch == "" {
		status.Missing = append(status.Missing, "branch")
	} else if strings.TrimSpace(record.Branch) != "" && actualBranch != strings.TrimSpace(record.Branch) {
		status.Missing = append(status.Missing, "branch_match")
	}
}

// appendRemoteBranchMissing는 원격 브랜치 점검 관측 결과를 missing/warnings에 반영한다.
func appendRemoteBranchMissing(status *model.CleanupStatus, facts CleanupStatusObservation, actualBranch string) {
	remote := strings.TrimSpace(facts.Remote)
	if remote == "" {
		status.Missing = append(status.Missing, "remote_branch_check_unavailable")
		return
	}
	if actualBranch == "" {
		return
	}
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

func hasUnverifiedChildClose(record model.Record) bool {
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

// finalizeCleanupStatus applies the cleanup status contract's stable sorting, readiness, and
// three-choice presentation to status assembled by either structural inspection
// or the cleanup finish readiness oracle.
func finalizeCleanupStatus(status model.CleanupStatus) model.CleanupStatus {
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
			"1. 차단 해소: missing에 적힌 항목을 먼저 채웁니다. (추천)",
			"2. 보류: worktree는 유지하고 나중에 다시 확인합니다.",
			"3. 확장 점검: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.",
		}
	}
	return status
}

// CleanupStatusWarnings 함수는 cleanup finish 결과의 워크스페이스 점유 프로세스를
// `pid:command:started_at` 경고로 옮기고, 요청자 자신이 점유 중인지에 따라 안내 문장을 덧붙인다.
func CleanupStatusWarnings(result model.CleanupFinishResult) []string {
	warnings := make([]string, 0, len(result.WorkspaceProcesses)+1)
	for _, process := range result.WorkspaceProcesses {
		warnings = append(warnings, fmt.Sprintf("%d:%s:%s", process.PID, process.Command, process.StartedAt))
	}
	switch {
	case len(result.WorkspaceProcesses) > 0 && slices.Contains(result.Missing, "requester_occupies_worktree"):
		// QA: run from inside the worktree, the list held the caller's own
		// shell and issueops, read as "apply will stop my session".
		warnings = append(warnings, "이 명령을 워크트리 안에서 실행해 호출한 셸도 목록에 들어 있습니다; source checkout에서 다시 실행하면 실제로 종료할 프로세스만 보입니다")
	case len(result.WorkspaceProcesses) > 0:
		warnings = append(warnings, fmt.Sprintf("apply가 프로세스 %d개를 종료합니다", len(result.WorkspaceProcesses)))
	}
	return warnings
}
