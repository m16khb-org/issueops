package worker

import (
	"encoding/json"
	"fmt"
	"io/fs"
	workercontract "issueops/internal/contract/worker"
	workerdomain "issueops/internal/domain/worker"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// workerBucket is the sqlstore bucket holding one row per worker job.
const workerBucket = "worker"

func openWorkerDB(dir string) (StateDatabase, error) {
	return OpenStateDatabase(dir)
}

func ReadWorkerJob(id string) (workercontract.WorkerJob, error) {
	if !workerdomain.ValidID(id) {
		return workercontract.WorkerJob{OK: false, ID: id}, fmt.Errorf("invalid worker job id")
	}
	dir, err := workerDir()
	if err != nil {
		return workercontract.WorkerJob{OK: false, ID: id}, err
	}
	db, err := openWorkerDB(dir)
	if err != nil {
		return workercontract.WorkerJob{OK: false, ID: id, WorkerDir: dir}, err
	}
	b, ok, err := db.Get(workerBucket, id)
	if err != nil {
		return workercontract.WorkerJob{OK: false, ID: id, WorkerDir: dir}, err
	}
	if !ok {
		return workercontract.WorkerJob{OK: false, ID: id, WorkerDir: dir}, fmt.Errorf("worker job %s: %w", id, fs.ErrNotExist)
	}
	var job workercontract.WorkerJob
	if err := json.Unmarshal(b, &job); err != nil {
		return workercontract.WorkerJob{OK: false, ID: id, WorkerDir: dir}, err
	}
	job.WorkerDir = dir
	return job, nil
}

func ListWorkerJobs() (workercontract.WorkerListResult, error) {
	dir, err := workerDir()
	if err != nil {
		return workercontract.WorkerListResult{OK: false}, err
	}
	result := workercontract.WorkerListResult{OK: true, WorkerDir: dir, Jobs: []workercontract.WorkerJob{}}
	db, err := openWorkerDB(dir)
	if err != nil {
		return result, err
	}
	ids, err := db.List(workerBucket)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		job, err := ReadWorkerJob(id)
		if err == nil {
			result.Jobs = append(result.Jobs, job)
		}
	}
	sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].CreatedAt > result.Jobs[j].CreatedAt })
	result.Queue = summarizeWorkerQueue(result.Jobs)
	return result, nil
}

// summarizeWorkerQueue builds the status histogram + saturation depth (A2/G6).
func summarizeWorkerQueue(jobs []workercontract.WorkerJob) *workercontract.WorkerQueueStats {
	return workerdomain.QueueStats(jobs)
}

func writeWorkerJob(job workercontract.WorkerJob) error {
	if !workerdomain.ValidID(job.ID) {
		return fmt.Errorf("invalid worker job id")
	}
	dir, err := workerDir()
	if err != nil {
		return err
	}
	db, err := openWorkerDB(dir)
	if err != nil {
		return err
	}
	job.WorkerDir = dir
	b, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	// The span lock is the CALLER's responsibility (withWorkerJobLock); the row
	// upsert itself is atomic, so a crash mid-write can never leave a truncated
	// job record that ListWorkerJobs silently drops.
	return db.Put(workerBucket, job.ID, append(b, '\n'))
}

func workerDir() (string, error) {
	if dir := os.Getenv("ISSUEOPS_WORKER_DIR"); dir != "" {
		return filepath.Abs(dir)
	}
	dir := os.Getenv("ISSUEOPS_STATE_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("resolve home for worker dir: %w", err)
		}
		dir = filepath.Join(home, ".local", "state", "issueops")
	}
	return filepath.Join(dir, "worker"), nil
}

// DetectStuckWorkerJobs scans all worker jobs. For any job stuck in
// "running" status whose PID is no longer alive, it marks the job as
// "failed" with an error message. Returns the list of jobs that were
// detected and fixed.
func DetectStuckWorkerJobs() (workercontract.WorkerListResult, error) {
	return workerService().DetectStuck()
}

func makeWorkerJobID(kind, payload string, t time.Time) string {
	return workerdomain.MakeID(kind, payload, t)
}
