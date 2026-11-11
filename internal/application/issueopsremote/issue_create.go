package issueopsremote

import (
	"context"
	"errors"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/artifacttemplate"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsremote"
	"issueops/internal/domain/policy"
	"issueops/internal/port"
)

// IssueCreateCommand 타입은 이슈 생성 명령의 입력이다. Confirm이 false이면 미리보기만 하고 provider에 쓰지 않는다.
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

// IssueCreateInvoker 타입은 provider에 이슈 생성을 요청하는 함수다.
type IssueCreateInvoker func(context.Context, port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error)

// IssueCreationEnvironment 타입은 이슈 생성에 필요한 provider 추론, 프로젝트 권한 해석, 생성 함수 조회를 제공한다.
type IssueCreationEnvironment interface {
	InferProvider(string) (string, error)
	ProjectAuthority(string, string) (string, error)
	Resolve(string) (IssueCreateInvoker, error)
}

// IssueCreator 타입은 기록을 검증하고 본문을 해석해 이슈를 만든다. 확정 생성은 durable 의도 기록과 live 검증 영수증을 남긴다.
type IssueCreator struct {
	records     issueRecordReader
	environment IssueCreationEnvironment
	bodies      TemplateBodyResolver
	intents     *IssueCreateIntents
	verify      IssueLiveVerifier
	now         func() time.Time
}

// NewIssueCreator 함수는 기록 조회, 환경, 본문 해석기, 의도 저장소, live 검증기, 시계를 묶어 IssueCreator를 만든다.
func NewIssueCreator(records issueRecordReader, environment IssueCreationEnvironment, bodies TemplateBodyResolver, intents *IssueCreateIntents, verify IssueLiveVerifier, now func() time.Time) *IssueCreator {
	return &IssueCreator{records: records, environment: environment, bodies: bodies, intents: intents, verify: verify, now: now}
}

// Create 메서드는 제목·본문·메타데이터를 검증하고 provider를 호출해 이슈를 만든다. 확정 생성이면 생성된 이슈를 live 검증한 뒤 의도를 완료하고, 실패는 의도에 남긴다.
func (s *IssueCreator) Create(ctx context.Context, cmd IssueCreateCommand) (IssueCreateResult, error) {
	var result IssueCreateResult
	labels, assignees := issueopsremote.CleanValues(cmd.Labels), issueopsremote.CleanValues(cmd.Assignees)
	record, err := s.records.Read(ctx, cmd.ID)
	if err != nil {
		return result, err
	}
	providerName, provider, err := s.resolveProvider(cmd, record)
	if err != nil {
		return result, err
	}
	resolved, err := s.resolveCreateBody(cmd, providerName, labels, assignees)
	result.Readability = resolved.Readability
	if err != nil {
		return result, err
	}
	body := resolved.Body
	authority, body, err := s.createAuthority(record, cmd, providerName, labels, assignees, body)
	if err != nil {
		return result, err
	}
	result.IssueProviderCreateIssueResult, err = s.invokeProvider(ctx, provider, record, cmd, authority, body, labels, assignees)
	if err != nil {
		return result, err
	}
	result.Provider = providerName
	result.Labels = issueopsremote.CleanValues(labels)
	result.Assignees = issueopsremote.CleanValues(assignees)
	if cmd.Confirm && strings.TrimSpace(result.URL) != "" {
		if err := s.verifyAndCompleteIntent(ctx, record.ID, providerName, result.URL, labels, assignees); err != nil {
			return result, err
		}
	}
	return result, nil
}

// resolveProvider validates the title and resolves the provider name and its create invoker.
func (s *IssueCreator) resolveProvider(cmd IssueCreateCommand, record model.Record) (string, IssueCreateInvoker, error) {
	if err := domain.ValidateIssueCreateTitle(cmd.Title); err != nil {
		return "", nil, err
	}
	if err := issueopsremote.ValidateProviderTitleLength("issue", cmd.Title); err != nil {
		return "", nil, err
	}
	providerName, err := s.resolveProviderName(cmd, record)
	if err != nil {
		return "", nil, err
	}
	provider, err := s.environment.Resolve(providerName)
	if err != nil {
		return "", nil, err
	}
	return providerName, provider, nil
}

// resolveCreateBody renders the body and validates it with the create inputs.
// The readability report is returned with a validation error so the caller can
// still show it; a body-resolution error carries none.
func (s *IssueCreator) resolveCreateBody(cmd IssueCreateCommand, providerName string, labels, assignees []string) (ResolvedTemplateBody, error) {
	resolved, err := s.bodies.Resolve(templateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactIssue, Template: cmd.Template, Provider: providerName, Title: cmd.Title, Body: cmd.Body, BodyFile: cmd.BodyFile, Fields: cmd.Fields, ScoreFile: cmd.ScoreFile, Confirm: cmd.Confirm})
	if err != nil {
		return ResolvedTemplateBody{}, err
	}
	if err := validateCreateInputs(cmd, resolved.Body, labels, assignees); err != nil {
		return resolved, err
	}
	return resolved, issueopsremote.ValidateProviderBodyLength(providerName, "issue", resolved.Body)
}

// createAuthority begins the durable intent on confirm; a preview only checks
// that the project authority resolves.
func (s *IssueCreator) createAuthority(record model.Record, cmd IssueCreateCommand, providerName string, labels, assignees []string, body string) (string, string, error) {
	if cmd.Confirm {
		return s.beginConfirmedIntent(record, providerName, cmd.Title, labels, assignees, body)
	}
	// The confirmed request resolves the same authority before it creates
	// anything; a preview that would pass and then fail at --confirm
	// hides the one problem the preview exists to show.
	_, err := s.environment.ProjectAuthority(record.Repo, providerName)
	return "", body, err
}

// invokeProvider calls the provider and records a confirmed create failure.
func (s *IssueCreator) invokeProvider(ctx context.Context, provider IssueCreateInvoker, record model.Record, cmd IssueCreateCommand, authority, body string, labels, assignees []string) (port.IssueProviderCreateIssueResult, error) {
	invokeContext := ctx
	if !cmd.Confirm {
		invokeContext = context.Background()
	}
	created, err := provider(invokeContext, port.IssueProviderCreateIssueRequest{Repo: record.Repo, ProjectKey: authority, Title: cmd.Title, Body: body, Labels: labels, Assignees: assignees, Confirm: cmd.Confirm})
	if err != nil && cmd.Confirm {
		err = s.recordCreateFailure(record.ID, created.URL, err)
	}
	return created, err
}

func validateCreateInputs(cmd IssueCreateCommand, body string, labels, assignees []string) error {
	if err := policy.ValidateRemoteCreateInputs("issue create", cmd.Title, body, labels, assignees); err != nil {
		return err
	}
	return issueopsremote.ValidateConfirmCreateMetadata(cmd.Confirm, labels, assignees)
}

func (s *IssueCreator) resolveProviderName(cmd IssueCreateCommand, record model.Record) (string, error) {
	providerName := firstNonEmpty(cmd.Provider, domain.ResolveRecordProvider(record))
	if providerName != "" {
		return providerName, nil
	}
	return s.environment.InferProvider(record.Repo)
}

// beginConfirmedIntent seals the create request, resolves project authority and begins the durable intent.
func (s *IssueCreator) beginConfirmedIntent(record model.Record, providerName, title string, labels, assignees []string, body string) (string, string, error) {
	request := model.IssueCreateIntentRequest{Provider: providerName, Title: title, Labels: append([]string(nil), labels...), Assignees: append([]string(nil), assignees...), StartedAt: s.timestamp()}
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
	outcome := model.IssueCreateOutcome{Status: domain.ClassifyIssueCreateFailure(typed && !createErr.Invoked, url), CanonicalURL: url}
	if typed {
		outcome.FailureCommand, outcome.FailureHTTPStatus = createErr.Command, createErr.HTTPStatus
	}
	return s.recordFailure(id, outcome, err)
}

func (s *IssueCreator) verifyAndCompleteIntent(ctx context.Context, id, providerName, url string, labels, assignees []string) error {
	if err := s.verify(ctx, model.RemoteArtifactVerificationRequest{Provider: providerName, Kind: "issue", URL: url, Labels: labels, Assignees: assignees}); err != nil {
		return s.recordFailure(id, model.IssueCreateOutcome{Status: model.IssueCreateIntentVerificationFailed, CanonicalURL: url}, err)
	}
	if _, err := s.intents.Complete(context.Background(), id, url, s.timestamp()); err != nil {
		return s.recordFailure(id, model.IssueCreateOutcome{Status: model.IssueCreateIntentReceiptFailed, CanonicalURL: url}, err)
	}
	return nil
}

// recordFailure는 outcome에 cause의 redaction된 실패 문자열과 관측 시각을 채워 기록하고, 기록 오류가 있으면 cause와 합친다.
func (s *IssueCreator) recordFailure(id string, outcome model.IssueCreateOutcome, cause error) error {
	outcome.Failure, outcome.ObservedAt = issueopsremote.IssueCreateFailure(cause), s.timestamp()
	_, err := s.intents.Outcome(context.Background(), id, outcome)
	if err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func (s *IssueCreator) timestamp() string { return s.now().UTC().Format(time.RFC3339Nano) }
