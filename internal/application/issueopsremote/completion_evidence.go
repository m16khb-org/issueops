package issueopsremote

import (
	"context"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type CompletionArtifactReader interface {
	Read(model.IssueOpsRecord, string, string) (string, bool)
}
type CompletionCollector struct{ artifacts CompletionArtifactReader }

func NewCompletionCollector(artifacts CompletionArtifactReader) CompletionCollector {
	return CompletionCollector{artifacts: artifacts}
}

func (c CompletionCollector) Collect(record model.IssueOpsRecord) model.RemoteCompletionSection {
	var observations []domain.CompletionArtifact
	if root := domain.CompletionArtifactRoot(record); root != "" {
		for _, name := range domain.CompletionArtifactNames() {
			body, present := c.artifacts.Read(record, root, name)
			observations = append(observations, domain.CompletionArtifact{Name: name, Body: body, Present: present})
		}
	}
	return domain.ProjectRemoteCompletion(record, observations)
}

type CompletionReceipts struct {
	store RecordStore
	now   func() time.Time
}

func NewCompletionReceipts(store RecordStore, now func() time.Time) CompletionReceipts {
	return CompletionReceipts{store: store, now: now}
}

func (s CompletionReceipts) Reflected(ctx context.Context, id string) (model.IssueOpsRecord, error) {
	return s.update(ctx, id, domain.MarkRemoteCompletionReflected)
}

func (s CompletionReceipts) Closed(ctx context.Context, id string) (model.IssueOpsRecord, error) {
	return s.update(ctx, id, domain.MarkRemoteIssueClosed)
}

func (s CompletionReceipts) update(ctx context.Context, id string, apply func(model.IssueOpsRecord, string) model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	record, err := s.store.Update(ctx, id, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		return apply(record, s.now().UTC().Format(time.RFC3339Nano)), nil
	})
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	return record, nil
}
