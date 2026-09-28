package issueopsremote

import (
	"context"
	"errors"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/artifacttemplate"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/domain/policy"
	"issueops/internal/port"
)

type IssueCreateCommand struct {
	ID        string
	Provider  string
	Title     string
	Body      string
	BodyFile  string
	Template  string
	ScoreFile string
	Fields    []string
	Labels    []string
	Assignees []string
	Confirm   bool
}

type IssueCreationProvider interface {
	Create(context.Context, port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error)
}

type IssueCreationEnvironment interface {
	InferProvider(string) (string, error)
	ProjectAuthority(string, string) (string, error)
	Resolve(string) (IssueCreationProvider, error)
}

type IssueCreator struct {
	records     IssueRecordReader
	environment IssueCreationEnvironment
	bodies      TemplateBodyResolver
	intents     *IssueCreateIntents
	verify      IssueLiveVerifier
	now         func() time.Time
}

func NewIssueCreator(records IssueRecordReader, environment IssueCreationEnvironment, bodies TemplateBodyResolver, intents *IssueCreateIntents, verify IssueLiveVerifier, now func() time.Time) *IssueCreator {
	return &IssueCreator{records: records, environment: environment, bodies: bodies, intents: intents, verify: verify, now: now}
}

func (s *IssueCreator) Create(ctx context.Context, cmd IssueCreateCommand) (port.IssueProviderCreateIssueResult, error) {
	var result port.IssueProviderCreateIssueResult
	labels, assignees := remote.CleanValues(cmd.Labels), remote.CleanValues(cmd.Assignees)
	record, err := s.records.Read(ctx, cmd.ID)
	if err != nil {
		return result, err
	}
	if err := domain.ValidateIssueCreateTitle(cmd.Title); err != nil {
		return result, err
	}
	providerName := firstNonEmpty(cmd.Provider, domain.ResolveRecordProvider(record))
	if providerName == "" {
		providerName, err = s.environment.InferProvider(record.Repo)
		if err != nil {
			return result, err
		}
	}
	provider, err := s.environment.Resolve(providerName)
	if err != nil {
		return result, err
	}
	body, err := s.bodies.Resolve(TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactIssue, Template: cmd.Template, Provider: providerName, Title: cmd.Title, Body: cmd.Body, BodyFile: cmd.BodyFile, Fields: cmd.Fields, ScoreFile: cmd.ScoreFile})
	if err != nil {
		return result, err
	}
	if err := policy.ValidateRemoteCreateInputs("issue create", cmd.Title, body, labels, assignees); err != nil {
		return result, err
	}
	if err := remote.ValidateConfirmCreateMetadata(cmd.Confirm, labels, assignees); err != nil {
		return result, err
	}
	authority := ""
	if cmd.Confirm {
		request := model.IssueOpsIssueCreateIntentRequest{Provider: providerName, Title: cmd.Title, Labels: append([]string(nil), labels...), Assignees: append([]string(nil), assignees...), StartedAt: s.timestamp()}
		request, body = domain.SealIssueCreateRequest(record, request, body)
		authority, err = s.environment.ProjectAuthority(record.Repo, providerName)
		if err != nil {
			return result, err
		}
		request.ProjectAuthority = authority
		if _, err := s.intents.Begin(context.Background(), record.ID, request); err != nil {
			return result, err
		}
	}
	result, err = provider.Create(ctx, port.IssueProviderCreateIssueRequest{Repo: record.Repo, ProjectKey: authority, Title: cmd.Title, Body: body, Labels: labels, Assignees: assignees, Confirm: cmd.Confirm})
	if err != nil {
		if cmd.Confirm {
			createErr, typed := errors.AsType[*port.IssueProviderCreateError](err)
			notInvoked := typed && !createErr.Invoked
			err = s.recordFailure(record.ID, domain.ClassifyIssueCreateFailure(notInvoked, result.URL), result.URL, err)
		}
		return result, err
	}
	result.Provider = providerName
	result.Labels = remote.CleanValues(labels)
	result.Assignees = remote.CleanValues(assignees)
	if cmd.Confirm && strings.TrimSpace(result.URL) != "" {
		if err := s.verify(ctx, model.IssueOpsRemoteArtifactVerificationRequest{Provider: providerName, Kind: "issue", URL: result.URL, Labels: labels, Assignees: assignees}); err != nil {
			return result, s.recordFailure(record.ID, model.IssueCreateIntentVerificationFailed, result.URL, err)
		}
		if _, err := s.intents.Complete(context.Background(), record.ID, result.URL, s.timestamp()); err != nil {
			return result, s.recordFailure(record.ID, model.IssueCreateIntentReceiptFailed, result.URL, err)
		}
	}
	return result, nil
}

func (s *IssueCreator) recordFailure(id, status, url string, cause error) error {
	_, err := s.intents.Outcome(context.Background(), id, model.IssueOpsIssueCreateOutcome{Status: status, CanonicalURL: url, Failure: IssueCreateFailure(cause), ObservedAt: s.timestamp()})
	if err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func (s *IssueCreator) timestamp() string { return s.now().UTC().Format(time.RFC3339Nano) }
