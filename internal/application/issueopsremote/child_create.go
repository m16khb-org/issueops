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

func (s ChildCreator) Create(ctx context.Context, cmd ChildCreateCommand, observe AncestryObserver) (result ChildCreateResult, resultErr error) {
	defer func() {
		if resultErr != nil {
			result.OK = false
		}
	}()
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
	resolved, err := s.Bodies.Resolve(TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactChild, Template: cmd.Template, Provider: providerName, Title: cmd.Title, Body: cmd.Body, BodyFile: cmd.BodyFile, Fields: cmd.Fields, ScoreFile: cmd.ScoreFile, Confirm: cmd.Confirm})
	if err != nil {
		return result, err
	}
	body := resolved.Body
	result.Readability = resolved.Readability
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
	var operation model.ChildCreateOperation
	if cmd.Confirm {
		if s.Intents == nil {
			return result, fmt.Errorf("child create durable store is required")
		}
		digest := domain.ChildRequestDigest(providerName, record.IssueURL, cmd.Title, body, labels, assignees)
		id, origin := strings.TrimSpace(cmd.OperationID), "explicit"
		if id == "" {
			origin = "implicit"
			if existing, ok := domain.FindChildOperation(record, "", digest); ok {
				id = existing.OperationID
			} else {
				id, err = s.NewOperationID()
				if err != nil {
					return result, err
				}
			}
		}
		operation, _, err = domain.SealChildOperation(record, id, origin, providerName, cmd.Title, body, digest, s.Intents.timestamp(), labels, assignees)
		if err != nil {
			return result, err
		}
		var invoke bool
		operation, invoke, err = s.Intents.Begin(ctx, record.ID, operation, strings.TrimSpace(cmd.OperationID), actor)
		result.OperationID = operation.OperationID
		result.ChildURL = operation.CanonicalURL
		result.RecoveryCommand = ChildRecoveryCommand(record.ID, operation.OperationID, actor)
		if err != nil {
			return result, err
		}
		if !invoke {
			result.IssueProviderCreateChildResult = port.IssueProviderCreateChildResult{OK: true, Provider: operation.Provider, ChildURL: operation.CanonicalURL, HierarchyVerified: true, Labels: operation.Labels, Assignees: operation.Assignees}
			result.RecoveryCommand = ""
			return result, nil
		}

		body = strings.TrimSpace(resolved.Body) + "\n\n" + operation.Marker
		if fmt.Sprintf("%x", sha256.Sum256([]byte(body))) != operation.BodySHA256 {
			err = fmt.Errorf("selected child operation body digest mismatch")
			return result, errors.Join(err, s.Intents.Outcome(context.Background(), record.ID, operation.OperationID, model.IssueCreateIntentNotInvoked, "", remote.IssueCreateFailure(err), actor))
		}
	}
	result.IssueProviderCreateChildResult, err = provider.CreateChild(port.IssueProviderCreateChildRequest{Repo: record.Repo, ParentIssueURL: record.IssueURL, Title: cmd.Title, Body: body, Labels: labels, Assignees: assignees, Confirm: cmd.Confirm})
	if err != nil {
		if cmd.Confirm {
			createErr, typed := errors.AsType[*port.IssueProviderCreateError](err)
			err = errors.Join(err, s.Intents.Outcome(context.Background(), record.ID, operation.OperationID, domain.ClassifyIssueCreateFailure(typed && !createErr.Invoked && result.ChildURL == "", result.ChildURL), result.ChildURL, remote.IssueCreateFailure(err), actor))
		}
		return result, err
	}
	if cmd.Confirm {
		if err := domain.ValidateCreatedChild(result.HierarchyVerified, result.ChildURL); err != nil {
			return result, errors.Join(err, s.Intents.Outcome(context.Background(), record.ID, operation.OperationID, model.IssueCreateIntentVerificationFailed, result.ChildURL, remote.IssueCreateFailure(err), actor))
		}
		if err := s.Intents.Complete(context.Background(), record, operation, result.ChildURL, actor, false); err != nil {
			return result, errors.Join(err, s.Intents.Outcome(context.Background(), record.ID, operation.OperationID, model.IssueCreateIntentReceiptFailed, result.ChildURL, remote.IssueCreateFailure(err), actor))
		}
		result.RecoveryCommand = ""
	}
	return result, nil
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
	}
	return command + " --confirm --json"
}
