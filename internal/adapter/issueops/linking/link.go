package linking

import (
	"fmt"
	"path/filepath"
	"strings"

	model "issueops/internal/contract/issueops"
)

type Store struct {
	Read                   func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite             func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	BranchEvidenceMissing  func(record model.IssueOpsRecord) []string
	DesignReviewMissing    func(record model.IssueOpsRecord) []string
	PlanPathExists         func(repo, path string) bool
	PlanSectionsMissing    func(path string) []string
	PlanPathInsideWorktree func(worktree, planPath string) bool
	WorktreePathValid      func(path string) bool
	UniqueSorted           func(values []string) []string
}

func LinkPlan(store Store, stateRoot, id, planPath string) (model.IssueOpsRecord, error) {
	path := strings.TrimSpace(planPath)
	if path == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path is required")
	}
	if strings.Contains(path, "\x00") || strings.Contains(path, "..") {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must not contain path traversal")
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if missing := store.BranchEvidenceMissing(record); len(missing) > 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot link plan before branch evidence: missing %s", strings.Join(missing, ", "))
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot link plan before linked worktree")
	}
	if missing := store.DesignReviewMissing(record); len(missing) > 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot link plan before approved design review: missing %s", strings.Join(store.UniqueSorted(missing), ", "))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(worktree, path)
	}
	if !store.PlanPathExists(record.Repo, path) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path does not exist: %s", path)
	}
	if !store.PlanPathInsideWorktree(worktree, path) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must be inside linked worktree: %s", worktree)
	}
	if linked := strings.TrimSpace(record.PlanPath); linked != "" {
		if !filepath.IsAbs(linked) {
			linked = filepath.Join(worktree, linked)
		}
		if filepath.Clean(linked) == filepath.Clean(path) {
			return record, nil
		}
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path is already linked; edit the linked plan in place instead of replacing its identity")
	}
	// 필수 절 검사는 새로 연결할 때만 한다. 이미 연결된 계획의 identity 규칙이
	// 먼저이고, 그 계획의 본문은 devils-advocate digest가 따로 묶는다.
	if missing := store.PlanSectionsMissing(path); len(missing) > 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan is missing required sections: %s", strings.Join(missing, ", "))
	}
	record.PlanPath = path
	return store.TouchWrite(stateRoot, record)
}

func LinkWorktree(store Store, stateRoot, id, worktreePath string) (model.IssueOpsRecord, error) {
	path := strings.TrimSpace(worktreePath)
	if path == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("worktree_path is required")
	}
	if strings.Contains(path, "\x00") || strings.Contains(path, "..") {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("worktree_path must not contain path traversal")
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if missing := store.BranchEvidenceMissing(record); len(missing) > 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot link worktree before branch evidence: missing %s", strings.Join(missing, ", "))
	}
	if !store.WorktreePathValid(path) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("worktree_path does not exist or is not a directory: %s", path)
	}
	if err := ValidateIsolatedWorktreePath(record, path); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	if err := ValidateWorktreeBranch(record, path); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	if planPath := strings.TrimSpace(record.PlanPath); planPath != "" && !store.PlanPathInsideWorktree(path, planPath) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must be inside linked worktree: %s", path)
	}
	record.WorktreePath = path
	return store.TouchWrite(stateRoot, record)
}
