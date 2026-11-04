package issueopsremote

import (
	"context"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsbodysync"
	domain "issueops/internal/domain/issueops"
	bodysync "issueops/internal/domain/issueopsbodysync"
	"issueops/internal/domain/policy"
)

// BodySyncInput 타입은 본문 동기화 명령에 provider 이름, 본문 파일, 동기화 명령, 행위자를 담는다.
type BodySyncInput struct {
	Provider, BodyFile string
	Command            contract.Command
	Actor              model.Actor
}

// BodySyncOperation 타입은 provider별로 해석된 본문 동기화 실행 함수다.
type BodySyncOperation func(context.Context, string, contract.Command, model.Actor) (model.Record, contract.Result, error)
type bodySyncResolver func(string) (BodySyncOperation, error)

// BodySyncCommandService 타입은 본문 입력을 해석·검증한 뒤 provider별 동기화 함수에 넘긴다.
type BodySyncCommandService struct {
	records issueRecordReader
	bodies  TemplateBodyResolver
	resolve bodySyncResolver
	observe AncestryObserver
}

// NewBodySyncCommandService 함수는 기록 조회, 본문 해석기, provider 해석 함수, 조상 프로세스 관찰기를 묶어 서비스를 만든다.
func NewBodySyncCommandService(records issueRecordReader, bodies TemplateBodyResolver, resolve bodySyncResolver, observe AncestryObserver) *BodySyncCommandService {
	return &BodySyncCommandService{records: records, bodies: bodies, resolve: resolve, observe: observe}
}

// Sync 메서드는 기록에서 provider를 정하고 본문을 읽어 검증한 뒤 조상 프로세스를 관찰한 행위자로 동기화를 실행한다.
func (s *BodySyncCommandService) Sync(ctx context.Context, input BodySyncInput) (model.Record, contract.Result, error) {
	var result contract.Result
	record, err := s.records.Read(ctx, input.Command.ID)
	if err != nil {
		return model.Record{}, result, err
	}
	name, err := domain.ResolveBodySyncProvider(record, input.Provider)
	if err != nil {
		return model.Record{}, result, err
	}
	sync, err := s.resolve(name)
	if err != nil {
		return model.Record{}, result, err
	}
	cmd := input.Command
	cmd.ID = record.ID
	cmd.ProposedBody, err = s.bodies.readBody(cmd.ProposedBody, input.BodyFile)
	if err != nil {
		return model.Record{}, result, err
	}
	if err := bodysync.ValidateCommandBody(cmd.ProposedBody); err != nil {
		return model.Record{}, result, err
	}
	if err := policy.ValidateRemoteCreateInputs("issueops remote sync-"+cmd.Kind, "", cmd.ProposedBody, nil, nil); err != nil {
		return model.Record{}, result, err
	}
	actor := input.Actor
	actor.NativeProcessAncestry, err = s.observe()
	if err != nil {
		return model.Record{}, result, err
	}
	return sync(ctx, record.ID, cmd, actor)
}
