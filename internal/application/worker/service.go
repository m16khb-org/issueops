// Package worker is the application layer for no-shell local worker jobs: it enqueues,
// runs read-only commands, cancels, lists, and repairs jobs. State transitions live in the
// worker domain; filesystem, locking, clock, and process effects come in through Effects.
package worker

import (
	"context"
	"sort"
	"time"

	policycontract "issueops/internal/contract/policy"
	workercontract "issueops/internal/contract/worker"
	policydomain "issueops/internal/domain/policy"
	workerdomain "issueops/internal/domain/worker"
)

// Effects is the set of side effects Service needs: the job directory, per-job locking,
// job persistence, the clock, process identity and liveness, and command execution.
type Effects interface {
	Dir() (string, error)
	EnsureDir(string) error
	WithLock(context.Context, string, string, func(context.Context) error) error
	Read(string) (workercontract.Job, error)
	Write(context.Context, workercontract.Job) error
	Now() time.Time
	PID() int
	Run(policycontract.CommandPolicyRequest) policycontract.CommandRunResult
	ListIDs(string) ([]string, error)
	PIDAlive(int) bool
}

// Service implements the worker job use cases on top of Effects.
type Service struct{ Effects Effects }

// Enqueue validates kind, redacts payload, and writes a new queued job under its lock.
// On failure it returns the job built so far (or a not-OK job) with the error.
func (service Service) Enqueue(ctx context.Context, kind, payload string) (workercontract.Job, error) {
	kind, err := workerdomain.NormalizeKind(kind)
	if err != nil {
		return workercontract.Job{OK: false}, err
	}
	dir, err := service.Effects.Dir()
	if err != nil {
		return workercontract.Job{OK: false}, err
	}
	if err := service.Effects.EnsureDir(dir); err != nil {
		return workercontract.Job{OK: false}, err
	}
	now := service.Effects.Now().UTC()
	id := workerdomain.MakeID(kind, payload, now)
	job, err := workerdomain.NewQueued(kind, policydomain.RedactFreeform(payload), id, dir, now.Format(time.RFC3339Nano))
	if err != nil {
		return job, err
	}
	return job, service.Effects.WithLock(ctx, dir, id, func(spanCtx context.Context) error { return service.Effects.Write(spanCtx, job) })
}

// Cancel marks the job id cancelled under its lock. Cancelling an already cancelled job is
// a no-op; other illegal transitions are returned as errors.
func (service Service) Cancel(ctx context.Context, id string) (workercontract.Job, error) {
	dir, err := service.Effects.Dir()
	if err != nil {
		return workercontract.Job{OK: false, ID: id}, err
	}
	var job workercontract.Job
	err = service.Effects.WithLock(ctx, dir, id, func(spanCtx context.Context) error {
		current, readErr := service.Effects.Read(id)
		job = current
		if readErr != nil {
			return readErr
		}
		if current.Status == workercontract.StatusCancelled {
			return nil
		}
		next, transitionErr := workerdomain.Cancel(current, service.Effects.Now().UTC().Format(time.RFC3339Nano))
		if transitionErr != nil {
			return transitionErr
		}
		job = next
		return service.Effects.Write(spanCtx, next)
	})
	return job, err
}

// RunReadOnly enqueues a job, marks it running with the current PID, runs the command
// request through Effects.Run, and records the result. The result is recorded even if ctx
// is cancelled while the command runs, so the job never stays running under a live PID.
func (service Service) RunReadOnly(ctx context.Context, kind, payload string, request policycontract.CommandPolicyRequest) (workercontract.Job, error) {
	job, err := service.Enqueue(ctx, kind, payload)
	if err != nil {
		return job, err
	}
	dir, err := service.Effects.Dir()
	if err != nil {
		return job, err
	}
	if err := service.Effects.WithLock(ctx, dir, job.ID, func(spanCtx context.Context) error {
		current, readErr := service.Effects.Read(job.ID)
		if readErr != nil {
			return readErr
		}
		job = current
		startedAt := service.Effects.Now().UTC().Format(time.RFC3339Nano)
		pid := service.Effects.PID()
		updatedAt := service.Effects.Now().UTC().Format(time.RFC3339Nano)
		next, transitionErr := workerdomain.Start(current, request.Argv, pid, startedAt, updatedAt)
		if transitionErr != nil {
			return transitionErr
		}
		job = next
		return service.Effects.Write(spanCtx, next)
	}); err != nil {
		return job, err
	}
	result := service.Effects.Run(request)
	// The command already ran; its result is recorded even if the request was
	// cancelled meanwhile, or the job would stay running under a live server PID.
	if err := service.Effects.WithLock(context.WithoutCancel(ctx), dir, job.ID, func(spanCtx context.Context) error {
		current, readErr := service.Effects.Read(job.ID)
		if readErr != nil {
			return readErr
		}
		job = current
		current.Result = &result
		current = workerdomain.Finish(current, result.OK, service.Effects.Now().UTC().Format(time.RFC3339Nano))
		job = current
		return service.Effects.Write(spanCtx, current)
	}); err != nil {
		return job, err
	}
	return job, nil
}

// DetectStuck finds running jobs whose recorded PID is no longer alive and marks them
// stuck under their lock. It returns the jobs it rewrote, newest first.
func (service Service) DetectStuck(ctx context.Context) (workercontract.ListResult, error) {
	dir, err := service.Effects.Dir()
	if err != nil {
		return workercontract.ListResult{OK: false}, err
	}
	result := workercontract.ListResult{OK: true, WorkerDir: dir, Jobs: []workercontract.Job{}}
	ids, err := service.Effects.ListIDs(dir)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		job, readErr := service.Effects.Read(id)
		if readErr != nil || job.Status != workercontract.StatusRunning || service.Effects.PIDAlive(job.PID) {
			continue
		}
		if next, fixed := service.markStuckLocked(ctx, dir, id); fixed {
			result.Jobs = append(result.Jobs, next)
		}
	}
	sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].CreatedAt > result.Jobs[j].CreatedAt })
	return result, nil
}

// markStuckLocked re-reads one running job under its lock and records it as
// stuck when its PID is still dead. fixed reports whether the job was rewritten.
func (service Service) markStuckLocked(ctx context.Context, dir, id string) (workercontract.Job, bool) {
	var job workercontract.Job
	fixed := false
	lockErr := service.Effects.WithLock(ctx, dir, id, func(spanCtx context.Context) error {
		current, reReadErr := service.Effects.Read(id)
		if reReadErr != nil {
			return reReadErr
		}
		if current.Status != workercontract.StatusRunning || service.Effects.PIDAlive(current.PID) {
			return nil
		}
		next, changed := workerdomain.MarkStuck(current, false, service.Effects.Now().UTC().Format(time.RFC3339Nano))
		if !changed {
			return nil
		}
		if err := service.Effects.Write(spanCtx, next); err != nil {
			return err
		}
		job = next
		fixed = true
		return nil
	})
	return job, lockErr == nil && fixed
}

// Read returns the stored job with the given id.
func (service Service) Read(id string) (workercontract.Job, error) {
	return service.Effects.Read(id)
}

// List returns every readable job, newest first, together with queue statistics.
// Jobs that cannot be read are skipped.
func (service Service) List() (workercontract.ListResult, error) {
	dir, err := service.Effects.Dir()
	if err != nil {
		return workercontract.ListResult{OK: false}, err
	}
	result := workercontract.ListResult{OK: true, WorkerDir: dir, Jobs: []workercontract.Job{}}
	ids, err := service.Effects.ListIDs(dir)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		job, err := service.Effects.Read(id)
		if err == nil {
			result.Jobs = append(result.Jobs, job)
		}
	}
	sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].CreatedAt > result.Jobs[j].CreatedAt })
	result.Queue = workerdomain.QueueStats(result.Jobs)
	return result, nil
}
