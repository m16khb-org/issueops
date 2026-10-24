package issueopsremote

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/artifacttemplate"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/domain/policy"
	"issueops/internal/port"
)

type ChildCreateCommand struct {
	ID, Provider, Title, Body, BodyFile, Template, ScoreFile string
	OperationID                                              string
	Fields, Labels, Assignees                                []string
	Confirm                                                  bool
	Actor                                                    model.IssueOpsActor
}

type ChildProvider interface {
	CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error)
}

type ChildCreator struct {
	Records        IssueRecordReader
	Resolve        func(string) (ChildProvider, error)
	Bodies         TemplateBodyResolver
	Authorize      func(context.Context, string, model.IssueOpsActor) error
	Intents        *ChildCreateIntents
	NewOperationID func() (string, error)
}

// childCreatePlan은 Create가 provider 호출 전에 확정한 입력이다.
type childCreatePlan struct {
	record       model.IssueOpsRecord
	providerName string
	provider     ChildProvider
	body         string
	labels       []string
	assignees    []string
}

func (s ChildCreator) Create(ctx context.Context, cmd ChildCreateCommand, observe AncestryObserver) (result ChildCreateResult, resultErr error) {
	defer func() {
		if resultErr != nil {
			result.OK = false
		}
	}()
	plan, err := s.prepareChildCreate(ctx, cmd, &result)
	if err != nil {
		return result, err
	}
	record, body := plan.record, plan.body
	actor, err := s.authorizeConfirmedCreate(ctx, cmd, record, observe)
	if err != nil {
		return result, err
	}
	if plan.provider == nil {
		return result, fmt.Errorf("no issue provider configured")
	}
	var operation model.ChildCreateOperation
	if cmd.Confirm {
		var invoke bool
		operation, body, invoke, err = s.beginConfirmedCreate(ctx, cmd, plan, actor, &result)
		if err != nil {
			return result, err
		}
		if !invoke {
			return result, nil
		}
	}
	result.IssueProviderCreateChildResult, err = plan.provider.CreateChild(port.IssueProviderCreateChildRequest{Repo: record.Repo, ParentIssueURL: record.IssueURL, Title: cmd.Title, Body: body, Labels: plan.labels, Assignees: plan.assignees, Confirm: cmd.Confirm})
	if err != nil {
		if cmd.Confirm {
			err = s.recordCreateChildFailure(ctx, record, operation, actor, result.ChildURL, err)
		}
		return result, err
	}
	if cmd.Confirm {
		if err := s.finalizeCreatedChild(ctx, record, operation, actor, result.ChildURL, result.HierarchyVerified); err != nil {
			return result, err
		}
		result.RecoveryCommand = ""
	}
	return result, nil
}

// prepareChildCreate는 기록 조회, 입력 검증, provider 결정, 본문 해석을 순서대로 수행한다.
func (s ChildCreator) prepareChildCreate(ctx context.Context, cmd ChildCreateCommand, result *ChildCreateResult) (childCreatePlan, error) {
	var plan childCreatePlan
	plan.labels, plan.assignees = remote.CleanValues(cmd.Labels), remote.CleanValues(cmd.Assignees)
	record, err := s.Records.Read(ctx, cmd.ID)
	if err != nil {
		return plan, err
	}
	plan.record = record
	if err := domain.ValidateChildCreation(record, cmd.Title, plan.labels, plan.assignees); err != nil {
		return plan, err
	}
	plan.providerName = firstNonEmpty(cmd.Provider, domain.ResolveRecordProvider(record))
	if plan.providerName == "" {
		return plan, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	plan.provider, err = s.Resolve(plan.providerName)
	if err != nil {
		return plan, err
	}
	resolved, err := s.Bodies.Resolve(TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactChild, Template: cmd.Template, Provider: plan.providerName, Title: cmd.Title, Body: cmd.Body, BodyFile: cmd.BodyFile, Fields: cmd.Fields, ScoreFile: cmd.ScoreFile, Confirm: cmd.Confirm})
	if err != nil {
		return plan, err
	}
	plan.body = resolved.Body
	result.Readability = resolved.Readability
	return plan, policy.ValidateRemoteCreateInputs("child create", cmd.Title, plan.body, plan.labels, plan.assignees)
}

// authorizeConfirmedCreate는 확정 생성 시 실행 lease가 있는 기록에 대해서만 조상 프로세스를 관찰하고 권한을 확인한다.
func (s ChildCreator) authorizeConfirmedCreate(ctx context.Context, cmd ChildCreateCommand, record model.IssueOpsRecord, observe AncestryObserver) (model.IssueOpsActor, error) {
	var actor model.IssueOpsActor
	if !cmd.Confirm || record.Execution == nil {
		return actor, nil
	}
	ancestry, err := observe()
	if err != nil {
		return actor, err
	}
	actor = cmd.Actor
	actor.NativeProcessAncestry = ancestry
	return actor, s.Authorize(ctx, record.ID, actor)
}

// beginConfirmedCreate는 작업을 봉인해 durable 저장소에 시작을 기록하고 provider에 보낼 본문을 돌려준다.
// invoke가 false이면 이미 완료된 작업이므로 result를 완료 상태로 채운다.
func (s ChildCreator) beginConfirmedCreate(ctx context.Context, cmd ChildCreateCommand, plan childCreatePlan, actor model.IssueOpsActor, result *ChildCreateResult) (model.ChildCreateOperation, string, bool, error) {
	record := plan.record
	if s.Intents == nil {
		return model.ChildCreateOperation{}, "", false, fmt.Errorf("child create durable store is required")
	}
	operation, err := s.sealChildOperation(record, cmd, plan)
	if err != nil {
		return operation, "", false, err
	}
	operation, invoke, err := s.Intents.Begin(ctx, record.ID, operation, strings.TrimSpace(cmd.OperationID), actor)
	result.OperationID = operation.OperationID
	result.ChildURL = operation.CanonicalURL
	result.RecoveryCommand = ChildRecoveryCommand(record.ID, operation.OperationID, actor)
	if err != nil {
		return operation, "", false, err
	}
	if !invoke {
		result.IssueProviderCreateChildResult = port.IssueProviderCreateChildResult{OK: true, Provider: operation.Provider, ChildURL: operation.CanonicalURL, HierarchyVerified: true, Labels: operation.Labels, Assignees: operation.Assignees}
		result.RecoveryCommand = ""
		return operation, "", false, nil
	}
	body := strings.TrimSpace(plan.body) + "\n\n" + operation.Marker
	if fmt.Sprintf("%x", sha256.Sum256([]byte(body))) != operation.BodySHA256 {
		err = fmt.Errorf("selected child operation body digest mismatch")
		return operation, "", false, errors.Join(err, s.Intents.Outcome(context.WithoutCancel(ctx), record.ID, operation.OperationID, model.IssueCreateIntentNotInvoked, "", remote.IssueCreateFailure(err), actor))
	}
	return operation, body, true, nil
}

// sealChildOperation은 명시 또는 암묵 operation id를 정해 요청 본문을 봉인한다.
func (s ChildCreator) sealChildOperation(record model.IssueOpsRecord, cmd ChildCreateCommand, plan childCreatePlan) (model.ChildCreateOperation, error) {
	digest := domain.ChildRequestDigest(plan.providerName, record.IssueURL, cmd.Title, plan.body, plan.labels, plan.assignees)
	id, origin := strings.TrimSpace(cmd.OperationID), "explicit"
	if id == "" {
		origin = "implicit"
		if existing, ok := domain.FindChildOperation(record, "", digest); ok {
			id = existing.OperationID
		} else {
			var err error
			id, err = s.NewOperationID()
			if err != nil {
				return model.ChildCreateOperation{}, err
			}
		}
	}
	operation, _, err := domain.SealChildOperation(record, id, origin, plan.providerName, cmd.Title, plan.body, digest, s.Intents.timestamp(), plan.labels, plan.assignees)
	return operation, err
}

// recordCreateChildFailure는 provider 생성 실패를 durable 저장소에 기록하고 두 오류를 합친다.
func (s ChildCreator) recordCreateChildFailure(ctx context.Context, record model.IssueOpsRecord, operation model.ChildCreateOperation, actor model.IssueOpsActor, childURL string, err error) error {
	createErr, typed := errors.AsType[*port.IssueProviderCreateError](err)
	return errors.Join(err, s.Intents.Outcome(context.WithoutCancel(ctx), record.ID, operation.OperationID, domain.ClassifyIssueCreateFailure(typed && !createErr.Invoked && childURL == "", childURL), childURL, remote.IssueCreateFailure(err), actor))
}

// finalizeCreatedChild는 생성된 자식의 계층을 검증하고 완료 영수증을 기록한다. 실패는 outcome으로 남긴다.
func (s ChildCreator) finalizeCreatedChild(ctx context.Context, record model.IssueOpsRecord, operation model.ChildCreateOperation, actor model.IssueOpsActor, childURL string, hierarchyVerified bool) error {
	if err := domain.ValidateCreatedChild(hierarchyVerified, childURL); err != nil {
		return errors.Join(err, s.Intents.Outcome(context.WithoutCancel(ctx), record.ID, operation.OperationID, model.IssueCreateIntentVerificationFailed, childURL, remote.IssueCreateFailure(err), actor))
	}
	if err := s.Intents.Complete(context.WithoutCancel(ctx), record, operation, childURL, actor, false); err != nil {
		return errors.Join(err, s.Intents.Outcome(context.WithoutCancel(ctx), record.ID, operation.OperationID, model.IssueCreateIntentReceiptFailed, childURL, remote.IssueCreateFailure(err), actor))
	}
	return nil
}

func ChildRecoveryCommand(id, operation string, actors ...model.IssueOpsActor) string {
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	command := "issueops remote reconcile-child --id " + quote(id) + " --operation-id " + quote(operation)
	if len(actors) > 0 && actors[0].Host != "" {
		actor := actors[0]
		command += " --host " + quote(actor.Host) + " --session-id " + quote(actor.SessionID) + " --cwd " + quote(actor.CWD)
		if actor.AgentID != "" {
			command += " --agent-id " + quote(actor.AgentID)
		}
	} else {
		// QA: the preview ran without actor flags and its command lacked
		// them, while --confirm requires them.
		command += " $RECORD_ACTOR_FLAGS"
	}
	return command + " --confirm --json"
}
