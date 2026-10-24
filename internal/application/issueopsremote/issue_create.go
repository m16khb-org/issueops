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

type IssueCreateInvoker func(context.Context, port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error)

type IssueCreationEnvironment interface {
	InferProvider(string) (string, error)
	ProjectAuthority(string, string) (string, error)
	Resolve(string) (IssueCreateInvoker, error)
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

func (s *IssueCreator) Create(ctx context.Context, cmd IssueCreateCommand) (IssueCreateResult, error) {
	var result IssueCreateResult
	labels, assignees := remote.CleanValues(cmd.Labels), remote.CleanValues(cmd.Assignees)
	record, err := s.records.Read(ctx, cmd.ID)
	if err != nil {
		return result, err
	}
	if err := domain.ValidateIssueCreateTitle(cmd.Title); err != nil {
		return result, err
	}
	providerName, err := s.resolveProviderName(cmd, record)
	if err != nil {
		return result, err
	}
	provider, err := s.environment.Resolve(providerName)
	if err != nil {
		return result, err
	}
	resolved, err := s.bodies.Resolve(TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactIssue, Template: cmd.Template, Provider: providerName, Title: cmd.Title, Body: cmd.Body, BodyFile: cmd.BodyFile, Fields: cmd.Fields, ScoreFile: cmd.ScoreFile, Confirm: cmd.Confirm})
	if err != nil {
		return result, err
	}
	body := resolved.Body
	result.Readability = resolved.Readability
	if err := validateCreateInputs(cmd, body, labels, assignees); err != nil {
		return result, err
	}
	authority := ""
	if cmd.Confirm {
		authority, body, err = s.beginConfirmedIntent(record, providerName, cmd.Title, labels, assignees, body)
		if err != nil {
			return result, err
		}
	} else if _, err = s.environment.ProjectAuthority(record.Repo, providerName); err != nil {
		// The confirmed request resolves the same authority before it creates
		// anything; a preview that would pass and then fail at --confirm
		// hides the one problem the preview exists to show.
		return result, err
	}
	invokeContext := ctx
	if !cmd.Confirm {
		invokeContext = context.Background()
	}
	result.IssueProviderCreateIssueResult, err = provider(invokeContext, port.IssueProviderCreateIssueRequest{Repo: record.Repo, ProjectKey: authority, Title: cmd.Title, Body: body, Labels: labels, Assignees: assignees, Confirm: cmd.Confirm})
	if err != nil {
		if cmd.Confirm {
			err = s.recordCreateFailure(record.ID, result.URL, err)
		}
		return result, err
	}
	result.Provider = providerName
	result.Labels = remote.CleanValues(labels)
	result.Assignees = remote.CleanValues(assignees)
	if cmd.Confirm && strings.TrimSpace(result.URL) != "" {
		if err := s.verifyAndCompleteIntent(ctx, record.ID, providerName, result.URL, labels, assignees); err != nil {
			return result, err
		}
	}
	return result, nil
}

func validateCreateInputs(cmd IssueCreateCommand, body string, labels, assignees []string) error {
	if err := policy.ValidateRemoteCreateInputs("issue create", cmd.Title, body, labels, assignees); err != nil {
		return err
	}
	return remote.ValidateConfirmCreateMetadata(cmd.Confirm, labels, assignees)
}

func (s *IssueCreator) resolveProviderName(cmd IssueCreateCommand, record model.IssueOpsRecord) (string, error) {
	providerName := firstNonEmpty(cmd.Provider, domain.ResolveRecordProvider(record))
	if providerName != "" {
		return providerName, nil
	}
	return s.environment.InferProvider(record.Repo)
}

// beginConfirmedIntent seals the create request, resolves project authority and begins the durable intent.
func (s *IssueCreator) beginConfirmedIntent(record model.IssueOpsRecord, providerName, title string, labels, assignees []string, body string) (string, string, error) {
	request := model.IssueOpsIssueCreateIntentRequest{Provider: providerName, Title: title, Labels: append([]string(nil), labels...), Assignees: append([]string(nil), assignees...), StartedAt: s.timestamp()}
	request, body = domain.SealIssueCreateRequest(record, request, body)
	authority, err := s.environment.ProjectAuthority(record.Repo, providerName)
	if err != nil {
		return "", body, err
	}
	request.ProjectAuthority = authority
	if _, err := s.intents.Begin(context.Background(), record.ID, request); err != nil {
		return authority, body, err
	}
	return authority, body, nil
}

func (s *IssueCreator) recordCreateFailure(id, url string, err error) error {
	createErr, typed := errors.AsType[*port.IssueProviderCreateError](err)
	notInvoked := typed && !createErr.Invoked
	return s.recordFailure(id, domain.ClassifyIssueCreateFailure(notInvoked, url), url, err)
}

func (s *IssueCreator) verifyAndCompleteIntent(ctx context.Context, id, providerName, url string, labels, assignees []string) error {
	if err := s.verify(ctx, model.IssueOpsRemoteArtifactVerificationRequest{Provider: providerName, Kind: "issue", URL: url, Labels: labels, Assignees: assignees}); err != nil {
		return s.recordFailure(id, model.IssueCreateIntentVerificationFailed, url, err)
	}
	if _, err := s.intents.Complete(context.Background(), id, url, s.timestamp()); err != nil {
		return s.recordFailure(id, model.IssueCreateIntentReceiptFailed, url, err)
	}
	return nil
}

func (s *IssueCreator) recordFailure(id, status, url string, cause error) error {
	_, err := s.intents.Outcome(context.Background(), id, model.IssueOpsIssueCreateOutcome{Status: status, CanonicalURL: url, Failure: remote.IssueCreateFailure(cause), ObservedAt: s.timestamp()})
	if err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func (s *IssueCreator) timestamp() string { return s.now().UTC().Format(time.RFC3339Nano) }
