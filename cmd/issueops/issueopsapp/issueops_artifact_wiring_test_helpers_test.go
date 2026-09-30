package issueopsapp

import (
	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
)

func stageIssueOpsArtifact(
	stateRoot string,
	id string,
	name string,
	content []byte,
) (issueopsartifactcontract.Record, error) {
	return issueOpsArtifactHandlers().Stage(stateRoot, id, name, content)
}
