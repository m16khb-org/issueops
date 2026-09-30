package remotecmd

import (
	"fmt"
	statestore "issueops/internal/adapter/outbound/state"
	model "issueops/internal/contract/issueops"
	"path/filepath"
)

func issueOpsStateRootForTest() string {
	return filepath.Join(statestore.StateDir(), fmt.Sprintf("issueops_v%d", model.IssueOpsSchemaVersion))
}
