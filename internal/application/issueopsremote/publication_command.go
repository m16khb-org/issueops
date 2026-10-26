package issueopsremote

import (
	"context"
	executionissue "issueops/internal/contract/executionissue"
	"strings"

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
	// issueState reads the linked issue's provider state for the preview.
	issueState func(ctx context.Context, record model.IssueOpsRecord, provider string) (string, error)
	// readFile reads the repository's own PR/MR template for the preview.
	readFile func(string) ([]byte, error)
}

// WithIssueState lets the preview read the linked issue's state, so a PR is
// not prepared for an issue someone closed meanwhile without saying so.
func (s *PublicationCommandService) WithIssueState(read func(ctx context.Context, record model.IssueOpsRecord, provider string) (string, error)) *PublicationCommandService {
	s.issueState = read
	return s
}

// WithRepoTemplates lets the preview compare the body with the
// repository's own PR/MR template.
func (s *PublicationCommandService) WithRepoTemplates(read func(string) ([]byte, error)) *PublicationCommandService {
	s.readFile = read
	return s
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
	if !req.Confirm {
		result.Warnings = append(s.issueWarnings(ctx, record, req.Provider), s.templateWarnings(record, req.Provider, req.Body)...)
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

// issueWarnings names a linked issue that is no longer open. QA: an issue
// closed as a duplicate between sessions went unnoticed, and the next session
// built and published a PR that "Closes" it.
func (s *PublicationCommandService) issueWarnings(ctx context.Context, record model.IssueOpsRecord, provider string) []string {
	if s.issueState == nil || strings.TrimSpace(record.IssueURL) == "" {
		return nil
	}
	state, err := s.issueState(ctx, record, provider)
	if err != nil {
		return []string{"issue_state_unread: could not read the state of " + record.IssueURL + ": " + err.Error()}
	}
	if strings.EqualFold(strings.TrimSpace(state), "closed") {
		return []string{"issue_closed: " + record.IssueURL + " is closed on the provider. Read why (its last comments) and ask the user before creating a PR for it: reopen it, link another issue, or stop the cycle"}
	}
	return nil
}

// templateWarnings names repository PR/MR template sections the body lacks.
// QA: an agent published a body with only the issueops sections in a
// repository whose template asked for What, Risk and a checklist.
func (s *PublicationCommandService) templateWarnings(record model.IssueOpsRecord, provider, body string) []string {
	if s.readFile == nil {
		return nil
	}
	root := strings.TrimSpace(record.WorktreePath)
	if record.Execution != nil && strings.TrimSpace(record.Execution.Workspace.Root) != "" {
		root = strings.TrimSpace(record.Execution.Workspace.Root)
	}
	if root == "" {
		root = strings.TrimSpace(record.Repo)
	}
	if root == "" {
		return nil
	}
	for _, rel := range remote.RepoPullRequestTemplatePaths(provider) {
		content, err := s.readFile(strings.TrimRight(root, "/") + "/" + rel)
		if err != nil {
			continue
		}
		missing := remote.TemplateSectionsMissing(string(content), body)
		if len(missing) == 0 {
			return nil
		}
		return []string{"repo_template_sections_missing: " + rel + " asks for sections the body lacks: " + strings.Join(missing, ", ") + ". Keep the required sections and add these after them; check a checklist item only for what you did"}
	}
	return nil
}
