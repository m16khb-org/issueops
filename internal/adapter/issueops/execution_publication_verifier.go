package issueops

import (
	"context"

	application "issueops/internal/application/issueopspublication"
	contract "issueops/internal/contract/issueopspublication"
)

type RemotePublicationVerifier struct {
	StateRoot string
	Verify    RemoteArtifactVerifyFunc
}

func (v RemotePublicationVerifier) VerifyCandidate(_ context.Context, intent contract.Intent, candidate contract.Candidate) error {
	record, payload, err := publicationIntentSnapshot(intent)
	if err != nil {
		return err
	}
	record, err = ReadIssueOps(v.StateRoot, record.ID)
	if err != nil {
		return err
	}
	return validateRemotePullRequestCandidate(record, payload, portPublicationCandidate(candidate))
}

func (v RemotePublicationVerifier) VerifyLive(_ context.Context, intent contract.Intent, url string) error {
	record, payload, err := publicationIntentSnapshot(intent)
	if err != nil {
		return err
	}
	record, err = ReadIssueOps(v.StateRoot, record.ID)
	if err != nil {
		return err
	}
	return verifyRemotePullRequestResult(record, payload, url, v.Verify)
}

var _ application.Verifier = RemotePublicationVerifier{}
