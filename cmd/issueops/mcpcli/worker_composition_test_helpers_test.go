package mcpcli

import (
	"issueops/internal/adapter/outbound/sqlstore"
	policyadapter "issueops/internal/adapter/policy"
	workeradapter "issueops/internal/adapter/worker"
	workerapp "issueops/internal/application/worker"
	"path/filepath"
)

func testWorkerStore() workeradapter.Store {
	dir, err := workeradapter.ResolveDirectory()
	physical := dir
	if abs, e := filepath.Abs(dir); e == nil {
		physical = abs
	}
	policy := policyadapter.NewEvaluator(nil)
	return workeradapter.Store{Directory: dir, DirectoryError: err, FilesystemDirectory: physical, OpenDatabase: func(dir string) (workeradapter.StateDatabase, error) { return sqlstore.Open(dir) }, RunCommand: policy.RunReadOnly}
}
func testWorkerService() workerapp.Service { return workerapp.Service{Effects: testWorkerStore()} }

func testHandleAssistantWorkerMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleAssistantWorkerMCPToolCall(call, testWorkerService(), testDaemonReader())
}
