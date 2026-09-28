package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ValidateNewCycleRequest(req model.IssueOpsStartRequest) error {
	if req.New && strings.TrimSpace(req.Branch) != "" {
		return fmt.Errorf("a new issueops cycle requires a branchless start")
	}
	return nil
}

// ReuseStartRecord decides whether the observed record can be resumed. Missing
// is supplied by the caller so the domain does not depend on storage errors.
func ReuseStartRecord(id string, fresh bool, readErr error, missing bool) (bool, error) {
	if readErr == nil {
		if fresh {
			return false, fmt.Errorf("refusing to create new issueops record %s: id already exists", id)
		}
		return true, nil
	}
	if fresh && !missing {
		return false, fmt.Errorf("refusing to create new issueops record %s: existing state is unreadable: %w", id, readErr)
	}
	return false, nil
}

func NewCycleRecord(id, repo, branch, now string) model.IssueOpsRecord {
	return model.IssueOpsRecord{
		OK: true, SchemaVersion: model.IssueOpsSchemaVersion,
		ID: id, Repo: repo, Branch: branch, Phase: model.IssueOpsPhaseProblem,
		Feedback: []model.IssueOpsFeedbackItem{}, CreatedAt: now, UpdatedAt: now,
	}
}
