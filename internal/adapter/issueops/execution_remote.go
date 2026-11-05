package issueops

import (
	"fmt"

	"issueops/internal/contract/issueops"
)

var externalIntentBucket = fmt.Sprintf("external_intent_v%d", issueops.SchemaVersion)

// RemoteArtifactVerifyFunc verifies one remote artifact against its request and returns an
// error when the artifact cannot be proven.
type RemoteArtifactVerifyFunc func(issueops.RemoteArtifactVerificationRequest) error
