package issueopsremote

import (
	"context"
	"fmt"
	"slices"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type assignProviderResolver func(string) (port.IssueProviderAssigner, error)

// RemoteAssignService adds user-requested assignees to the cycle's issue or
// PR/MR. QA: asked to add an assignee, an agent ran raw `gh issue edit
// --add-assignee`. Assignees are only added; the verified artifact names its
// assignee, and removing one is not something a request to "add" implies.
type RemoteAssignService struct {
	records issueRecordReader
	resolve assignProviderResolver
}

// NewRemoteAssignService 함수는 기록 조회와 provider 해석 함수를 묶어 서비스를 만든다.
func NewRemoteAssignService(records issueRecordReader, resolve assignProviderResolver) *RemoteAssignService {
	return &RemoteAssignService{records: records, resolve: resolve}
}

// AddAssignees 메서드는 on(issue 또는 PR/MR)에 담당자를 추가한다. confirm이 false이면 미리보기만 하고, 확정하면 provider에 쓴 뒤 읽어 되돌려 확인한다.
func (s *RemoteAssignService) AddAssignees(ctx context.Context, id, on string, users []string, confirm bool) (model.RemoteAssignResult, error) {
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return model.RemoteAssignResult{}, err
	}
	target, err := commentTarget(record, on)
	if err != nil {
		return model.RemoteAssignResult{}, err
	}
	provider := domain.ResolveRecordProvider(record)
	if provider == "" {
		return model.RemoteAssignResult{}, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	add := cleanLabels(users)
	result := model.RemoteAssignResult{ID: record.ID, Provider: provider, On: on, TargetURL: target, Add: add}
	if len(add) == 0 {
		return result, fmt.Errorf("--add USER is required (repeatable)")
	}
	if err := validateAssigneeNames(add); err != nil {
		return result, err
	}
	if !confirm {
		result.OK = true
		return result, nil
	}
	assigner, err := s.resolve(provider)
	if err != nil {
		return result, err
	}
	readback, err := assigner.AddAssignees(ctx, record.Repo, target, on, add)
	if err != nil {
		return result, fmt.Errorf("%w; adding an assignee again is harmless, but read the %s first", err, on)
	}
	result.Assignees = readback
	if err := requireAssigneesRead(add, readback, on); err != nil {
		return result, err
	}
	result.OK, result.Assigned = true, true
	return result, nil
}

func validateAssigneeNames(add []string) error {
	for _, user := range add {
		// A leading @, +, - or ! is CLI syntax (glab reads -/! as removal),
		// never part of a username.
		if strings.ContainsAny(user[:1], "@+-!") || strings.ContainsAny(user, " \t") {
			return fmt.Errorf("--add takes a bare username, got %q", user)
		}
	}
	return nil
}

func requireAssigneesRead(add, readback []string, on string) error {
	for _, user := range add {
		if !slices.Contains(readback, user) {
			return fmt.Errorf("assignee %q was not read back on the %s", user, on)
		}
	}
	return nil
}
