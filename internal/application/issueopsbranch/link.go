package issueopsbranch

import (
	"context"
	"strings"
	"time"

	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
)

type Linker struct {
	Records   CycleRecords
	Authority cycleapp.MutationAuthority
	Now       func() time.Time
}

func (s Linker) Issue(ctx context.Context, id, issueURL string, actor *model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return s.mutate(ctx, id, actor, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		return domain.ApplyIssueLink(record, issueURL, s.Now().UTC().Format(time.RFC3339Nano))
	})
}

func (s Linker) Child(ctx context.Context, id, childURL, title string, actor *model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return s.mutate(ctx, id, actor, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		u := strings.TrimSpace(childURL)
		if err := domain.ValidateRelationURL(u, "child_url"); err != nil {
			return model.IssueOpsRecord{OK: false}, err
		}
		if err := domain.ValidateChildLinkParent(record); err != nil {
			return model.IssueOpsRecord{OK: false}, err
		}
		if err := remote.ValidateChildLinkProvider(record.IssueURL, u); err != nil {
			return model.IssueOpsRecord{OK: false}, err
		}
		if err := remote.ValidateChildMatchesParent(record.IssueURL, u); err != nil {
			return model.IssueOpsRecord{OK: false}, err
		}
		return domain.AppendIssueRelation(record, model.IssueOpsIssueLink{Type: "child", URL: u, Title: title, Provider: remote.ProviderFromURL(u)}, s.Now().UTC().Format(time.RFC3339Nano))
	})
}

func (s Linker) Related(ctx context.Context, id, linkType, relatedURL, title string, actor *model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return s.mutate(ctx, id, actor, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		lt, u := strings.TrimSpace(linkType), strings.TrimSpace(relatedURL)
		if err := domain.ValidateRelatedLinkType(lt); err != nil {
			return model.IssueOpsRecord{OK: false}, err
		}
		if err := domain.ValidateRelationURL(u, "related_url"); err != nil {
			return model.IssueOpsRecord{OK: false}, err
		}
		return domain.AppendIssueRelation(record, model.IssueOpsIssueLink{Type: lt, URL: u, Title: title, Provider: remote.ProviderFromURL(u)}, s.Now().UTC().Format(time.RFC3339Nano))
	})
}

func (s Linker) mutate(ctx context.Context, id string, actor *model.IssueOpsActor, change func(model.IssueOpsRecord) (model.IssueOpsRecord, error)) (model.IssueOpsRecord, error) {
	var result model.IssueOpsRecord
	err := s.Records.WithinLock(ctx, id, func(spanCtx context.Context) error {
		record, err := s.Records.Load(id)
		if err != nil {
			return err
		}
		if err = s.Authority.Validate(ctx, record, actor); err != nil {
			return err
		}
		result, err = change(record)
		if err != nil {
			return err
		}
		result, err = s.Records.Save(spanCtx, result)
		return err
	})
	return result, err
}
