package issueopsremote

import (
	"context"
	"strings"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	publicationdomain "issueops/internal/domain/issueopspublication"
	"issueops/internal/domain/issueopsremote"
)

type publicationVerificationReader interface {
	DecodeSnapshot(contract.Intent) (model.Record, contract.IntentPayload, error)
	Read(context.Context, string) (model.Record, error)
}

// PublicationVerifier 타입은 게시 의도에서 나온 후보와 생성된 PR/MR이 의도·프로젝트 권한과 일치하는지 검증한다.
type PublicationVerifier struct {
	reader publicationVerificationReader
	verify func(model.RemoteArtifactVerificationRequest) error
}

// NewPublicationVerifier 함수는 의도 해독·기록 조회기와 live 검증 함수를 묶어 PublicationVerifier를 만든다. verify가 nil이면 live 검증은 정적 검사만 한다.
func NewPublicationVerifier(reader publicationVerificationReader, verify func(model.RemoteArtifactVerificationRequest) error) *PublicationVerifier {
	return &PublicationVerifier{reader: reader, verify: verify}
}

func (v *PublicationVerifier) current(ctx context.Context, intent contract.Intent) (model.Record, contract.IntentPayload, error) {
	snapshot, payload, err := v.reader.DecodeSnapshot(intent)
	if err != nil {
		return model.Record{}, payload, err
	}
	record, err := v.reader.Read(ctx, snapshot.ID)
	return record, payload, err
}

// VerifyCandidate 메서드는 복구 후보가 의도 요청과 일치하고 URL이 provider·종류·프로젝트 권한에 맞는지 확인한다. provider를 호출하지 않는다.
func (v *PublicationVerifier) VerifyCandidate(ctx context.Context, intent contract.Intent, candidate contract.Candidate) error {
	record, payload, err := v.current(ctx, intent)
	if err != nil {
		return err
	}
	if err := publicationdomain.ValidateCandidate(payload.Request, candidate, payload.KnownURL); err != nil {
		return err
	}
	if err := issueopsremote.ValidateArtifactURL(candidate.URL, payload.Provider, payload.Kind); err != nil {
		return err
	}
	codeProjectKey := ""
	if record.BranchPrepare != nil {
		codeProjectKey = record.BranchPrepare.CodeProjectKey
	}
	return issueopsremote.ValidateArtifactMatchesProject(issueopsremote.EffectiveProjectKey(codeProjectKey, record.IssueURL, payload.Provider), candidate.URL, payload.Provider, payload.Kind)
}

// VerifyLive 메서드는 생성된 URL이 기록의 아티팩트 권한에 맞는지 확인한 뒤 verify 함수로 provider에서 live 검증한다.
func (v *PublicationVerifier) VerifyLive(ctx context.Context, intent contract.Intent, url string) error {
	record, payload, err := v.current(ctx, intent)
	if err != nil {
		return err
	}
	req := model.RemoteArtifactVerificationRequest{Provider: payload.Provider, Kind: payload.Kind, URL: strings.TrimSpace(url), Labels: payload.Request.Labels, Assignees: payload.Request.Assignees, TargetBranch: payload.Request.BaseBranch}
	if _, err := projectRemoteArtifact(record, req); err != nil {
		return err
	}
	if v.verify != nil {
		return v.verify(req)
	}
	return nil
}
