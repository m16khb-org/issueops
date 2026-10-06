package issueopsremote

import (
	"context"
	executionissue "issueops/internal/contract/executionissue"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/artifacttemplate"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/domain/policy"
)

type PublicationInput struct {
	Request                       model.RemotePullRequestRequest
	BodyFile, Template, ScoreFile string
	Fields                        []string
}

type PublicationInvoker func(context.Context, model.RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error)

type PublicationCommandService struct {
	records   IssueRecordReader
	bodies    TemplateBodyResolver
	observe   AncestryObserver
	normalize func(context.Context, model.NativeActor) (model.NativeActor, error)
	publish   PublicationInvoker
}

func NewPublicationCommandService(records IssueRecordReader, bodies TemplateBodyResolver, observe AncestryObserver, normalize func(context.Context, model.NativeActor) (model.NativeActor, error), publish PublicationInvoker) *PublicationCommandService {
	return &PublicationCommandService{records: records, bodies: bodies, observe: observe, normalize: normalize, publish: publish}
}

func (s *PublicationCommandService) Create(ctx context.Context, input PublicationInput) (PublicationResult, error) {
	var result PublicationResult
	req := input.Request
	req.Labels, req.Assignees = remote.CleanValues(req.Labels), remote.CleanValues(req.Assignees)
	record, err := s.records.Read(ctx, req.ID)
	if err != nil {
		return result, err
	}
	req, err = domain.ResolvePublicationDefaults(record, req)
	if err != nil {
		return result, err
	}
	resolved, err := s.bodies.Resolve(TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactPR, Template: input.Template, Provider: req.Provider, Title: req.Title, Body: req.Body, BodyFile: input.BodyFile, Fields: input.Fields, ScoreFile: input.ScoreFile, Confirm: req.Confirm})
	if err != nil {
		return result, err
	}
	req.Body = resolved.Body
	result.Readability = resolved.Readability
	if err := policy.ValidateRemoteCreateInputs("pr create", req.Title, req.Body, req.Labels, req.Assignees); err != nil {
		return result, err
	}
	if err := remote.ValidateConfirmCreateMetadata(req.Confirm, req.Labels, req.Assignees); err != nil {
		return result, err
	}
	if req.Confirm {
		req.Actor.ProcessAncestry, err = s.observe()
		if err != nil {
			return result, err
		}
	}
	if s.publish == nil {
		return result, model.ErrRemotePullRequestCreateHandlerUnavailable
	}
	if req.Confirm {
		req.Actor, err = s.normalize(ctx, req.Actor)
		if err != nil {
			return result, err
		}
	}
	result.IssueProviderCreatePullRequestResult, err = s.publish(ctx, req)
	return result, err
}
