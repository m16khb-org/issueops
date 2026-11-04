package issueopsremote

import (
	"context"
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/issueopsremote"
)

type artifactVerificationStore interface {
	issueRecordReader
	RecordStore
	WithinTransaction(context.Context, string, func(context.Context) error) error
}

// ArtifactLiveVerifier 타입은 기록 전에 원격 아티팩트(PR/MR)가 실제로 존재하고 요청과 일치하는지 provider에서 확인하는 함수다.
type ArtifactLiveVerifier func(context.Context, model.RemoteArtifactVerificationRequest) error

// ArtifactVerificationService 타입은 원격 아티팩트를 live 검증한 뒤 그 영수증을 기록에 남긴다.
type ArtifactVerificationService struct {
	store     artifactVerificationStore
	authority authority
	verify    ArtifactLiveVerifier
	observe   AncestryObserver
	now       func() time.Time
}

// NewArtifactVerificationService 함수는 저장소, 권한 검사, live 검증기, 조상 프로세스 관찰기, 시계를 묶어 서비스를 만든다.
func NewArtifactVerificationService(store artifactVerificationStore, authority authority, verify ArtifactLiveVerifier, observe AncestryObserver, now func() time.Time) *ArtifactVerificationService {
	return &ArtifactVerificationService{store: store, authority: authority, verify: verify, observe: observe, now: now}
}

// Verify 메서드는 요청을 검증하고 provider에서 live 검증한 뒤, 조상 프로세스를 관찰한 행위자로 영수증을 기록한다.
func (s *ArtifactVerificationService) Verify(ctx context.Context, id string, req model.RemoteArtifactVerificationRequest, actor model.Actor) (model.Record, error) {
	if _, err := s.Validate(ctx, id, req); err != nil {
		return model.Record{}, err
	}
	if err := s.verify(ctx, req); err != nil {
		return model.Record{}, err
	}
	ancestry, err := s.observe()
	if err != nil {
		return model.Record{}, err
	}
	actor.NativeProcessAncestry = ancestry
	return s.Record(ctx, id, req, actor)
}

// Validate 메서드는 기록을 읽기만 해서 요청이 기록의 아티팩트 권한(phase, 이슈 URL, 코드 프로젝트)과 맞는지 확인하고 해당 기록을 돌려준다.
func (s *ArtifactVerificationService) Validate(ctx context.Context, id string, req model.RemoteArtifactVerificationRequest) (model.Record, error) {
	var record model.Record
	// Validation only reads, so it follows the caller's cancellation; Record
	// is the durable write that outlives it.
	err := s.store.WithinTransaction(ctx, id, func(ctx context.Context) error {
		var err error
		record, err = s.store.Read(ctx, id)
		if err != nil {
			return err
		}
		if _, err = projectRemoteArtifact(record, req); err != nil {
			record = model.Record{}
		}
		return err
	})
	return record, err
}

// Record 메서드는 행위자 권한을 확인한 뒤 검증된 아티팩트를 기록에 저장한다. 호출자 취소와 무관하게 영속화한다.
func (s *ArtifactVerificationService) Record(ctx context.Context, id string, req model.RemoteArtifactVerificationRequest, actor model.Actor) (model.Record, error) {
	// Keep receipt persistence independent of cancellation after live verification,
	// while keeping the request's caller authority values.
	persist := context.WithoutCancel(ctx)
	return s.store.Update(persist, id, func(record model.Record) (model.Record, error) {
		if err := s.authority.Authorize(persist, record, actor); err != nil {
			return record, err
		}
		artifact, err := projectRemoteArtifact(record, req)
		if err != nil {
			return record, err
		}
		artifact.VerifiedAt = s.now().UTC().Format(time.RFC3339Nano)
		record.RemoteArtifact = &artifact
		record.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
		return record, nil
	})
}

func projectRemoteArtifact(record model.Record, req model.RemoteArtifactVerificationRequest) (model.RemoteArtifactVerification, error) {
	authority := issueopsremote.ArtifactAuthority{Phase: string(record.Phase), IssueURL: record.IssueURL}
	if record.BranchPrepare != nil {
		authority.CodeProjectKey = record.BranchPrepare.CodeProjectKey
	}
	artifact, err := issueopsremote.ProjectArtifact(authority, issueopsremote.Artifact{Provider: req.Provider, Kind: req.Kind, URL: req.URL, Labels: req.Labels, Assignees: req.Assignees, TargetBranch: req.TargetBranch})
	if err != nil {
		return model.RemoteArtifactVerification{}, err
	}
	return model.RemoteArtifactVerification{Provider: artifact.Provider, Kind: artifact.Kind, URL: artifact.URL, Labels: artifact.Labels, Assignees: artifact.Assignees, TargetBranch: artifact.TargetBranch}, nil
}
