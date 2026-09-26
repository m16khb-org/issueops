package worker

import (
	"context"
	"os"
	"time"

	workerapp "issueops/internal/application/worker"
	policycontract "issueops/internal/contract/policy"
	workercontract "issueops/internal/contract/worker"
)

type workerEffects struct{}

func workerService() workerapp.Service           { return workerapp.Service{Effects: workerEffects{}} }
func (workerEffects) Dir() (string, error)       { return workerDir() }
func (workerEffects) EnsureDir(dir string) error { return os.MkdirAll(dir, 0o700) }
func (workerEffects) WithLock(ctx context.Context, dir, id string, fn func(context.Context) error) error {
	return withWorkerJobLock(ctx, dir, id, fn)
}
func (workerEffects) Read(id string) (workercontract.WorkerJob, error) { return ReadWorkerJob(id) }
func (workerEffects) Write(job workercontract.WorkerJob) error         { return writeWorkerJob(job) }
func (workerEffects) Now() time.Time                                   { return time.Now() }
func (workerEffects) PID() int                                         { return os.Getpid() }
func (workerEffects) Run(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return RunReadOnlyCommand(request)
}
func (workerEffects) ListIDs(dir string) ([]string, error) {
	db, err := openWorkerDB(dir)
	if err != nil {
		return nil, err
	}
	return db.List(workerBucket)
}
func (workerEffects) PIDAlive(pid int) bool { return isPIDAlive(pid) }
