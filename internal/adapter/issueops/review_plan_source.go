package issueops

import model "issueops/internal/contract/issueops"

type ReviewPlanSource struct{}

func (ReviewPlanSource) LinkedDigest(record model.IssueOpsRecord) (string, error) {
	identity, err := readLinkedPlanIdentity(record)
	if err != nil {
		return "", err
	}
	return identity.Digest, nil
}

func (ReviewPlanSource) StagedPlans(stateRoot, id string) (map[string]string, error) {
	return readStagedArtifacts(stateRoot, id)
}
