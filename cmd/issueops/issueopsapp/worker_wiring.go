package issueopsapp

import (
	"issueops/cmd/issueops/workercli"

	"issueops/internal/adapter/outbound/sqlstore"
	policyadapter "issueops/internal/adapter/policy"
	workeradapter "issueops/internal/adapter/worker"
	workerapp "issueops/internal/application/worker"
	"path/filepath"
)

func newWorkerStore() workeradapter.Store {
	directory, err := workeradapter.ResolveDirectory()
	filesystemDirectory := directory
	if absolute, absErr := filepath.Abs(directory); absErr == nil {
		filesystemDirectory = absolute
	}
	reader := newActiveCycleReader(issueOpsStateRoot())
	policy := policyadapter.NewEvaluator(reader.PreparedBaseBranchForWorkspace)
	return workeradapter.Store{
		Directory: directory, DirectoryError: err, FilesystemDirectory: filesystemDirectory,
		OpenDatabase: func(dir string) (workeradapter.StateDatabase, error) { return sqlstore.Open(dir) },
		RunCommand:   policy.RunReadOnly,
	}
}

func newWorkerService() workerapp.Service { return workerapp.Service{Effects: newWorkerStore()} }
func newWorkerCommand() workercli.Command {
	root := resolveTarget("")
	return workercli.Command{Service: newWorkerService(), ResolveTarget: func(target string) string {
		if target != "" {
			return target
		}
		return root
	}}
}
