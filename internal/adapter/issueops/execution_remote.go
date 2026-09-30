package issueops

import (
	"fmt"

	"issueops/internal/contract/issueops"
)

var externalIntentBucket = fmt.Sprintf("external_intent_v%d", issueops.IssueOpsSchemaVersion)

type RemoteArtifactVerifyFunc func(issueops.IssueOpsRemoteArtifactVerificationRequest) error
