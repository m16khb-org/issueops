package issueopsremote

import (
	"context"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsbodysync"
	domain "issueops/internal/domain/issueops"
	bodysync "issueops/internal/domain/issueopsbodysync"
	"issueops/internal/domain/policy"
)

type BodySyncInput struct {
	Provider, BodyFile string
	Command            contract.Command
	Actor              model.IssueOpsActor
}

type BodySyncOperation func(context.Context, string, contract.Command, model.IssueOpsActor) (model.IssueOpsRecord, contract.Result, error)
type BodySyncResolver func(string) (BodySyncOperation, error)

type BodySyncCommandService struct {
	records IssueRecordReader
	bodies  TemplateBodyResolver
	resolve BodySyncResolver
	observe AncestryObserver
}

func NewBodySyncCommandService(records IssueRecordReader, bodies TemplateBodyResolver, resolve BodySyncResolver, observe AncestryObserver) *BodySyncCommandService {
	return &BodySyncCommandService{records: records, bodies: bodies, resolve: resolve, observe: observe}
}

func (s *BodySyncCommandService) Sync(ctx context.Context, input BodySyncInput) (model.IssueOpsRecord, contract.Result, error) {
	var result contract.Result
	record, err := s.records.Read(ctx, input.Command.ID)
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	name, err := domain.ResolveBodySyncProvider(record, input.Provider)
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	sync, err := s.resolve(name)
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	cmd := input.Command
	cmd.ID = record.ID
	cmd.ProposedBody, err = s.bodies.ReadBody(cmd.ProposedBody, input.BodyFile)
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	if err := bodysync.ValidateCommandBody(cmd.ProposedBody); err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	if err := policy.ValidateRemoteCreateInputs("issueops remote sync-"+cmd.Kind, "", cmd.ProposedBody, nil, nil); err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	actor := input.Actor
	actor.NativeProcessAncestry, err = s.observe()
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	return sync(ctx, record.ID, cmd, actor)
}
