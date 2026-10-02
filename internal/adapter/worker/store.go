package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	workercontract "issueops/internal/contract/worker"
	workerdomain "issueops/internal/domain/worker"
	"issueops/internal/port"
	"os"
	"path/filepath"
)

// workerBucket is the sqlstore bucket holding one row per worker job.
const workerBucket = "worker"

func (store Store) open() (StateDatabase, error) {
	return store.OpenDatabase(store.FilesystemDirectory)
}

func (store Store) Read(id string) (workercontract.WorkerJob, error) {
	if !workerdomain.ValidID(id) {
		return workercontract.WorkerJob{OK: false, ID: id}, fmt.Errorf("invalid worker job id")
	}
	dir, err := store.Dir()
	if err != nil {
		return workercontract.WorkerJob{OK: false, ID: id}, err
	}
	db, err := store.open()
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

func (store Store) Write(ctx context.Context, job workercontract.WorkerJob) error {
	if !workerdomain.ValidID(job.ID) {
		return fmt.Errorf("invalid worker job id")
	}
	dir, err := store.Dir()
	if err != nil {
		return err
	}
	db, err := store.open()
	if err != nil {
		return err
	}
	job.WorkerDir = dir
	b, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	// The span lock is the CALLER's responsibility (Store.WithLock); the row
	// upsert itself is atomic, so a crash mid-write can never leave a truncated
	// job record that the application list operation silently drops.
	return db.Apply(ctx, []port.RecordMutation{{Bucket: workerBucket, ID: job.ID, Data: append(b, '\n')}})
}

func ResolveDirectory() (string, error) {
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
