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

type LabelProviderResolver func(string) (port.IssueProviderLabeler, error)

// RemoteLabelService adds user-requested labels to the cycle's issue or
// PR/MR. QA: asked to add a label, one agent stopped (no issueops path) and
// another ran raw `gh issue|pr edit --add-label` without readback. Labels are
// only added; removing one could drop a label the verified artifact needs.
type RemoteLabelService struct {
	records IssueRecordReader
	resolve LabelProviderResolver
}

func NewRemoteLabelService(records IssueRecordReader, resolve LabelProviderResolver) *RemoteLabelService {
	return &RemoteLabelService{records: records, resolve: resolve}
}

func (s *RemoteLabelService) AddLabels(ctx context.Context, id, on string, labels []string, confirm bool) (model.RemoteLabelResult, error) {
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return model.RemoteLabelResult{}, err
	}
	target, err := commentTarget(record, on)
	if err != nil {
		return model.RemoteLabelResult{}, err
	}
	provider := domain.ResolveRecordProvider(record)
	if provider == "" {
		return model.RemoteLabelResult{}, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	add := cleanLabels(labels)
	result := model.RemoteLabelResult{ID: record.ID, Provider: provider, On: on, TargetURL: target, Add: add}
	if len(add) == 0 {
		return result, fmt.Errorf("--add LABEL is required (repeatable)")
	}
	if !confirm {
		result.OK = true
		return result, nil
	}
	labeler, err := s.resolve(provider)
	if err != nil {
		return result, err
	}
	readback, err := labeler.AddLabels(ctx, record.Repo, target, on, add)
	if err != nil {
		return result, fmt.Errorf("%w; adding a label again is harmless, but read the %s's labels first: the provider refuses a label the repository does not define", err, on)
	}
	result.Labels = readback
	for _, label := range add {
		if !slices.Contains(readback, label) {
			return result, fmt.Errorf("label %q was not read back on the %s", label, on)
		}
	}
	result.OK, result.Added = true, true
	return result, nil
}

func cleanLabels(labels []string) []string {
	out := []string{}
	for _, label := range labels {
		for part := range strings.SplitSeq(label, ",") {
			if part = strings.TrimSpace(part); part != "" && !slices.Contains(out, part) {
				out = append(out, part)
			}
		}
	}
	return out
}
