package artifactverify

import (
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/issueopsremote"
)

type Store struct {
	Read       func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
}

func Verify(store Store, stateRoot, id string, req model.IssueOpsRemoteArtifactVerificationRequest) (model.IssueOpsRecord, error) {
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	artifact, err := Projection(record, req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record.RemoteArtifact = &artifact
	return store.TouchWrite(stateRoot, record)
}

func Validate(store Store, stateRoot, id string, req model.IssueOpsRemoteArtifactVerificationRequest) (model.IssueOpsRecord, error) {
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	_, err = Projection(record, req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	return record, nil
}

func Projection(record model.IssueOpsRecord, req model.IssueOpsRemoteArtifactVerificationRequest) (model.IssueOpsRemoteArtifactVerification, error) {
	authority := remote.ArtifactAuthority{Phase: string(record.Phase), IssueURL: record.IssueURL}
	if record.BranchPrepare != nil {
		authority.CodeProjectKey = record.BranchPrepare.CodeProjectKey
	}
	artifact, err := remote.ProjectArtifact(authority, remote.Artifact{
		Provider: req.Provider, Kind: req.Kind, URL: req.URL,
		Labels: req.Labels, Assignees: req.Assignees, TargetBranch: req.TargetBranch,
	})
	if err != nil {
		return model.IssueOpsRemoteArtifactVerification{}, err
	}
	return model.IssueOpsRemoteArtifactVerification{
		Provider: artifact.Provider, Kind: artifact.Kind, URL: artifact.URL,
		Labels: artifact.Labels, Assignees: artifact.Assignees, TargetBranch: artifact.TargetBranch,
		VerifiedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}
