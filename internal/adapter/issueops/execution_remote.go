package issueops

import (
	"fmt"

	"issueops/internal/contract/issueops"
	publicationcontract "issueops/internal/contract/issueopspublication"
)

var externalIntentBucket = fmt.Sprintf("external_intent_v%d", issueops.IssueOpsSchemaVersion)

const (
	externalIntentRemotePR = publicationcontract.RemoteIntentKind
)

type RemoteArtifactVerifyFunc func(issueops.IssueOpsRemoteArtifactVerificationRequest) error
