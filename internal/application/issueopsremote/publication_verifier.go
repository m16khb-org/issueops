package issueopsremote

import (
	"context"
	"strings"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	publicationdomain "issueops/internal/domain/issueopspublication"
	remote "issueops/internal/domain/issueopsremote"
)

type PublicationVerificationReader interface {
	DecodeSnapshot(contract.Intent) (model.IssueOpsRecord, contract.IntentPayload, error)
	Read(context.Context, string) (model.IssueOpsRecord, error)
}

type PublicationVerifier struct {
	reader PublicationVerificationReader
	verify func(model.IssueOpsRemoteArtifactVerificationRequest) error
}

func NewPublicationVerifier(reader PublicationVerificationReader, verify func(model.IssueOpsRemoteArtifactVerificationRequest) error) *PublicationVerifier {
	return &PublicationVerifier{reader: reader, verify: verify}
}

func (v *PublicationVerifier) current(ctx context.Context, intent contract.Intent) (model.IssueOpsRecord, contract.IntentPayload, error) {
	snapshot, payload, err := v.reader.DecodeSnapshot(intent)
	if err != nil {
		return model.IssueOpsRecord{}, payload, err
	}
	record, err := v.reader.Read(ctx, snapshot.ID)
	return record, payload, err
}

func (v *PublicationVerifier) VerifyCandidate(ctx context.Context, intent contract.Intent, candidate contract.Candidate) error {
	record, payload, err := v.current(ctx, intent)
	if err != nil {
		return err
	}
	if err := publicationdomain.ValidateCandidate(payload.Request, candidate, payload.KnownURL); err != nil {
		return err
	}
	if err := remote.ValidateArtifactURL(candidate.URL, payload.Provider, payload.Kind); err != nil {
		return err
	}
	codeProjectKey := ""
	if record.BranchPrepare != nil {
		codeProjectKey = record.BranchPrepare.CodeProjectKey
	}
	return remote.ValidateArtifactMatchesProject(remote.EffectiveProjectKey(codeProjectKey, record.IssueURL, payload.Provider), candidate.URL, payload.Provider, payload.Kind)
}

func (v *PublicationVerifier) VerifyLive(ctx context.Context, intent contract.Intent, url string) error {
	record, payload, err := v.current(ctx, intent)
	if err != nil {
		return err
	}
	req := model.IssueOpsRemoteArtifactVerificationRequest{Provider: payload.Provider, Kind: payload.Kind, URL: strings.TrimSpace(url), Labels: payload.Request.Labels, Assignees: payload.Request.Assignees, TargetBranch: payload.Request.BaseBranch}
	if _, err := projectRemoteArtifact(record, req); err != nil {
		return err
	}
	if v.verify != nil {
		return v.verify(req)
	}
	return nil
}
