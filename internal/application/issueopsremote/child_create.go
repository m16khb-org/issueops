package issueopsremote

import (
	"context"
	"fmt"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/artifacttemplate"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/domain/policy"
	"issueops/internal/port"
)

type ChildCreateCommand struct {
	ID, Provider, Title, Body, BodyFile, Template, ScoreFile string
	Fields, Labels, Assignees                                []string
	Confirm                                                  bool
	Actor                                                    model.IssueOpsActor
}

type ChildProvider interface {
	CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error)
}

type ChildCreator struct {
	Records   IssueRecordReader
	Resolve   func(string) (ChildProvider, error)
	Bodies    TemplateBodyResolver
	Authorize func(context.Context, string, model.IssueOpsActor) error
	Link      func(context.Context, string, string, string, model.IssueOpsActor) error
}

func (s ChildCreator) Create(ctx context.Context, cmd ChildCreateCommand, observe AncestryObserver) (port.IssueProviderCreateChildResult, error) {
	var result port.IssueProviderCreateChildResult
	labels, assignees := remote.CleanValues(cmd.Labels), remote.CleanValues(cmd.Assignees)
	record, err := s.Records.Read(ctx, cmd.ID)
	if err != nil {
		return result, err
	}
	if err := domain.ValidateChildCreation(record, cmd.Title, labels, assignees); err != nil {
		return result, err
	}
	providerName := firstNonEmpty(cmd.Provider, domain.ResolveRecordProvider(record))
	if providerName == "" {
		return result, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	provider, err := s.Resolve(providerName)
	if err != nil {
		return result, err
	}
	body, err := s.Bodies.Resolve(TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactChild, Template: cmd.Template, Provider: providerName, Title: cmd.Title, Body: cmd.Body, BodyFile: cmd.BodyFile, Fields: cmd.Fields, ScoreFile: cmd.ScoreFile})
	if err != nil {
		return result, err
	}
	if err := policy.ValidateRemoteCreateInputs("child create", cmd.Title, body, labels, assignees); err != nil {
		return result, err
	}
	var actor model.IssueOpsActor
	if cmd.Confirm && record.Execution != nil {
		ancestry, err := observe()
		if err != nil {
			return result, err
		}
		actor = cmd.Actor
		actor.NativeProcessAncestry = ancestry
		if err := s.Authorize(ctx, record.ID, actor); err != nil {
			return result, err
		}
	}
	if provider == nil {
		return result, fmt.Errorf("no issue provider configured")
	}
	result, err = provider.CreateChild(port.IssueProviderCreateChildRequest{Repo: record.Repo, ParentIssueURL: record.IssueURL, Title: cmd.Title, Body: body, Labels: labels, Assignees: assignees, Confirm: cmd.Confirm})
	if err != nil {
		return result, err
	}
	if cmd.Confirm {
		if err := domain.ValidateCreatedChild(result.HierarchyVerified, result.ChildURL); err != nil {
			return result, err
		}
		if err := s.Link(ctx, record.ID, result.ChildURL, cmd.Title, actor); err != nil {
			return result, err
		}
	}
	return result, nil
}
