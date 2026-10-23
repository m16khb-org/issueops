package mcpcli

import (
	"fmt"
	"path/filepath"

	statestore "issueops/internal/adapter/outbound/state"
	model "issueops/internal/contract/issueops"
)

func issueOpsStateRootForTest() string {
	return filepath.Join(statestore.StateDir(), fmt.Sprintf("issueops_v%d", model.IssueOpsSchemaVersion))
}
