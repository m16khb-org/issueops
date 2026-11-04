package issueopsbranch

import (
	"context"
	"strings"
	"time"

	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsremote"
)

// Linker records issue and relation links on a cycle under the cycle lock,
// after the mutation authority has accepted the actor.
type Linker struct {
	Records   CycleRecords
	Authority cycleapp.MutationAuthority
	Now       func() time.Time
}

// Issue links the cycle to issueURL and returns the saved record.
func (s Linker) Issue(ctx context.Context, id, issueURL string, actor *model.Actor) (model.Record, error) {
	return s.mutate(ctx, id, actor, func(record model.Record) (model.Record, error) {
		return domain.ApplyIssueLink(record, issueURL, s.Now().UTC().Format(time.RFC3339Nano))
	})
}

// Child records childURL as a child issue of the cycle's linked issue. It
// fails when the cycle has no valid parent issue or the child is on another
// provider or outside the parent's project.
func (s Linker) Child(ctx context.Context, id, childURL, title string, actor *model.Actor) (model.Record, error) {
	return s.mutate(ctx, id, actor, func(record model.Record) (model.Record, error) {
		u := strings.TrimSpace(childURL)
		if err := domain.ValidateRelationURL(u, "child_url"); err != nil {
			return model.Record{OK: false}, err
		}
		if err := domain.ValidateChildLinkParent(record); err != nil {
			return model.Record{OK: false}, err
		}
		if err := issueopsremote.ValidateChildLinkProvider(record.IssueURL, u); err != nil {
			return model.Record{OK: false}, err
		}
		if err := issueopsremote.ValidateChildMatchesParent(record.IssueURL, u); err != nil {
			return model.Record{OK: false}, err
		}
		return domain.AppendIssueRelation(record, model.IssueLink{Type: "child", URL: u, Title: title, Provider: issueopsremote.ProviderFromURL(u)}, s.Now().UTC().Format(time.RFC3339Nano))
	})
}

// Related records relatedURL with the given linkType as a related issue of
// the cycle, rejecting unknown link types and malformed URLs.
func (s Linker) Related(ctx context.Context, id, linkType, relatedURL, title string, actor *model.Actor) (model.Record, error) {
	return s.mutate(ctx, id, actor, func(record model.Record) (model.Record, error) {
		lt, u := strings.TrimSpace(linkType), strings.TrimSpace(relatedURL)
		if err := domain.ValidateRelatedLinkType(lt); err != nil {
			return model.Record{OK: false}, err
		}
		if err := domain.ValidateRelationURL(u, "related_url"); err != nil {
			return model.Record{OK: false}, err
		}
		return domain.AppendIssueRelation(record, model.IssueLink{Type: lt, URL: u, Title: title, Provider: issueopsremote.ProviderFromURL(u)}, s.Now().UTC().Format(time.RFC3339Nano))
	})
}

func (s Linker) mutate(ctx context.Context, id string, actor *model.Actor, change func(model.Record) (model.Record, error)) (model.Record, error) {
	var result model.Record
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
