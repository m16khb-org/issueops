package issueopsremote

import (
	"context"
	"time"

	model "issueops/internal/contract/issueops"
	remote "issueops/internal/domain/issueopsremote"
)

type ArtifactVerificationStore interface {
	IssueRecordReader
	RecordStore
	WithinTransaction(context.Context, string, func(context.Context) error) error
}

type ArtifactVerificationAuthority interface {
	Authorize(context.Context, model.IssueOpsRecord, model.IssueOpsActor) error
}

type ArtifactLiveVerifier func(context.Context, model.IssueOpsRemoteArtifactVerificationRequest) error

type ArtifactVerificationService struct {
	store     ArtifactVerificationStore
	authority ArtifactVerificationAuthority
	verify    ArtifactLiveVerifier
	observe   AncestryObserver
	now       func() time.Time
}

func NewArtifactVerificationService(store ArtifactVerificationStore, authority ArtifactVerificationAuthority, verify ArtifactLiveVerifier, observe AncestryObserver, now func() time.Time) *ArtifactVerificationService {
	return &ArtifactVerificationService{store: store, authority: authority, verify: verify, observe: observe, now: now}
}

func (s *ArtifactVerificationService) Verify(ctx context.Context, id string, req model.IssueOpsRemoteArtifactVerificationRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	if _, err := s.Validate(ctx, id, req); err != nil {
		return model.IssueOpsRecord{}, err
	}
	if err := s.verify(ctx, req); err != nil {
		return model.IssueOpsRecord{}, err
	}
	ancestry, err := s.observe()
	if err != nil {
		return model.IssueOpsRecord{}, err
	}
	actor.NativeProcessAncestry = ancestry
	return s.Record(ctx, id, req, actor)
}

func (s *ArtifactVerificationService) Validate(_ context.Context, id string, req model.IssueOpsRemoteArtifactVerificationRequest) (model.IssueOpsRecord, error) {
	var record model.IssueOpsRecord
	err := s.store.WithinTransaction(context.Background(), id, func(ctx context.Context) error {
		var err error
		record, err = s.store.Read(ctx, id)
		if err != nil {
			return err
		}
		if _, err = projectRemoteArtifact(record, req); err != nil {
			record = model.IssueOpsRecord{}
		}
		return err
	})
	return record, err
}

func (s *ArtifactVerificationService) Record(ctx context.Context, id string, req model.IssueOpsRemoteArtifactVerificationRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	// Keep receipt persistence independent of cancellation after live verification,
	// while keeping the request's caller authority values.
	persist := context.WithoutCancel(ctx)
	return s.store.Update(persist, id, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
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

func projectRemoteArtifact(record model.IssueOpsRecord, req model.IssueOpsRemoteArtifactVerificationRequest) (model.IssueOpsRemoteArtifactVerification, error) {
	authority := remote.ArtifactAuthority{Phase: string(record.Phase), IssueURL: record.IssueURL}
	if record.BranchPrepare != nil {
		authority.CodeProjectKey = record.BranchPrepare.CodeProjectKey
	}
	artifact, err := remote.ProjectArtifact(authority, remote.Artifact{Provider: req.Provider, Kind: req.Kind, URL: req.URL, Labels: req.Labels, Assignees: req.Assignees, TargetBranch: req.TargetBranch})
	if err != nil {
		return model.IssueOpsRemoteArtifactVerification{}, err
	}
	return model.IssueOpsRemoteArtifactVerification{Provider: artifact.Provider, Kind: artifact.Kind, URL: artifact.URL, Labels: artifact.Labels, Assignees: artifact.Assignees, TargetBranch: artifact.TargetBranch}, nil
}
