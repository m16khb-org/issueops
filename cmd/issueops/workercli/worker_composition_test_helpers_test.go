package workercli

import (
	"issueops/internal/adapter/outbound/sqlstore"
	policyadapter "issueops/internal/adapter/policy"
	workeradapter "issueops/internal/adapter/worker"
	workerapp "issueops/internal/application/worker"
	"os"
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

func testWorkerCommand() Command {
	return Command{Service: testWorkerService(), ResolveTarget: func(target string) string {
		if target != "" {
			return target
		}
		cwd, err := os.Getwd()
		if err != nil {
			return "."
		}
		return cwd
	}}
}
func runWorker(args []string) error             { return testWorkerCommand().Run(args) }
func runWorkerEnqueue(args []string) error      { return testWorkerCommand().RunEnqueue(args) }
func runWorkerRun(args []string) error          { return testWorkerCommand().RunReadOnly(args) }
func runWorkerStatus(args []string) error       { return testWorkerCommand().RunStatus(args) }
func runWorkerList(args []string) error         { return testWorkerCommand().RunList(args) }
func runWorkerCleanupStuck(args []string) error { return testWorkerCommand().RunCleanupStuck(args) }
func runWorkerCancel(args []string) error       { return testWorkerCommand().RunCancel(args) }

func Run(args []string) error { return testWorkerCommand().Run(args) }

func RunEnqueue(args []string) error { return testWorkerCommand().RunEnqueue(args) }

func RunReadOnly(args []string) error { return testWorkerCommand().RunReadOnly(args) }

func RunStatus(args []string) error { return testWorkerCommand().RunStatus(args) }

func RunList(args []string) error { return testWorkerCommand().RunList(args) }

func RunCancel(args []string) error { return testWorkerCommand().RunCancel(args) }
