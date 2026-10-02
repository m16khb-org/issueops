package worker

import (
	"context"
	"issueops/internal/adapter/outbound/sqlstore"
	policyadapter "issueops/internal/adapter/policy"
	workerapp "issueops/internal/application/worker"
	policycontract "issueops/internal/contract/policy"
	workercontract "issueops/internal/contract/worker"
	"path/filepath"
)

func testWorkerStore() Store {
	dir, err := ResolveDirectory()
	physical := dir
	if abs, e := filepath.Abs(dir); e == nil {
		physical = abs
	}
	policy := policyadapter.NewEvaluator(nil)
	return Store{Directory: dir, DirectoryError: err, FilesystemDirectory: physical, OpenDatabase: func(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }, RunCommand: policy.RunReadOnly}
}
func testWorkerService() workerapp.Service { return workerapp.Service{Effects: testWorkerStore()} }

func workerDir() (string, error)                     { return ResolveDirectory() }
func openWorkerDB(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }
func EnqueueWorkerJob(kind, payload string) (workercontract.WorkerJob, error) {
	return testWorkerService().Enqueue(context.Background(), kind, payload)
}
func CancelWorkerJob(id string) (workercontract.WorkerJob, error) {
	return testWorkerService().Cancel(context.Background(), id)
}
func ReadWorkerJob(id string) (workercontract.WorkerJob, error) { return testWorkerService().Read(id) }
func ListWorkerJobs() (workercontract.WorkerListResult, error)  { return testWorkerService().List() }
func DetectStuckWorkerJobs() (workercontract.WorkerListResult, error) {
	return testWorkerService().DetectStuck(context.Background())
}
func RunReadOnlyWorkerJob(kind, payload string, req policycontract.CommandPolicyRequest) (workercontract.WorkerJob, error) {
	return testWorkerService().RunReadOnly(context.Background(), kind, payload, req)
}
func writeWorkerJob(job workercontract.WorkerJob) error {
	return testWorkerStore().Write(context.Background(), job)
}
