package issueopsapp

import (
	adapter "issueops/internal/adapter/issueops"
	health "issueops/internal/adapter/operationalhealth"
)

func newOperationalHealthCollector(root string, git health.GitRunner, orca health.OrcaInventory) health.Collector {
	return health.Collector{Git: git, Orca: orca, InspectNativeProcess: adapter.InspectNativeProcessReceipt,
		IssueOps: health.IssueOpsReader{StateRoot: root, ListIDs: adapter.ListIssueOpsIDs, ListLeaseHolders: adapter.ListLeaseHolderIndexes, Read: adapter.ReadIssueOpsExisting}}
}
