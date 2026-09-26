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

type Effects interface {
	Dir() (string, error)
	EnsureDir(string) error
	WithLock(context.Context, string, string, func(context.Context) error) error
	Read(string) (workercontract.WorkerJob, error)
	Write(workercontract.WorkerJob) error
	Now() time.Time
	PID() int
	Run(policycontract.CommandPolicyRequest) policycontract.CommandRunResult
	ListIDs(string) ([]string, error)
	PIDAlive(int) bool
}

type Service struct{ Effects Effects }

func (service Service) Enqueue(kind, payload string) (workercontract.WorkerJob, error) {
	kind, err := workerdomain.NormalizeKind(kind)
	if err != nil {
		return workercontract.WorkerJob{OK: false}, err
	}
	dir, err := service.Effects.Dir()
	if err != nil {
		return workercontract.WorkerJob{OK: false}, err
	}
	if err := service.Effects.EnsureDir(dir); err != nil {
		return workercontract.WorkerJob{OK: false}, err
	}
	now := service.Effects.Now().UTC()
	id := workerdomain.MakeID(kind, payload, now)
	job, err := workerdomain.NewQueued(kind, policydomain.RedactFreeform(payload), id, dir, now.Format(time.RFC3339Nano))
	if err != nil {
		return job, err
	}
	return job, service.Effects.WithLock(context.Background(), dir, id, func(context.Context) error { return service.Effects.Write(job) })
}

func (service Service) Cancel(id string) (workercontract.WorkerJob, error) {
	dir, err := service.Effects.Dir()
	if err != nil {
		return workercontract.WorkerJob{OK: false, ID: id}, err
	}
	var job workercontract.WorkerJob
	err = service.Effects.WithLock(context.Background(), dir, id, func(context.Context) error {
		current, readErr := service.Effects.Read(id)
		job = current
		if readErr != nil {
			return readErr
		}
		if current.Status == workercontract.WorkerStatusCancelled {
			return nil
		}
		next, transitionErr := workerdomain.Cancel(current, service.Effects.Now().UTC().Format(time.RFC3339Nano))
		if transitionErr != nil {
			return transitionErr
		}
		job = next
		return service.Effects.Write(next)
	})
	return job, err
}

func (service Service) RunReadOnly(kind, payload string, request policycontract.CommandPolicyRequest) (workercontract.WorkerJob, error) {
	job, err := service.Enqueue(kind, payload)
	if err != nil {
		return job, err
	}
	dir, err := service.Effects.Dir()
	if err != nil {
		return job, err
	}
	if err := service.Effects.WithLock(context.Background(), dir, job.ID, func(context.Context) error {
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
		return service.Effects.Write(next)
	}); err != nil {
		return job, err
	}
	result := service.Effects.Run(request)
	if err := service.Effects.WithLock(context.Background(), dir, job.ID, func(context.Context) error {
		current, readErr := service.Effects.Read(job.ID)
		if readErr != nil {
			return readErr
		}
		job = current
		current.Result = &result
		current = workerdomain.Finish(current, result.OK, service.Effects.Now().UTC().Format(time.RFC3339Nano))
		job = current
		return service.Effects.Write(current)
	}); err != nil {
		return job, err
	}
	return job, nil
}

func (service Service) DetectStuck() (workercontract.WorkerListResult, error) {
	dir, err := service.Effects.Dir()
	if err != nil {
		return workercontract.WorkerListResult{OK: false}, err
	}
	result := workercontract.WorkerListResult{OK: true, WorkerDir: dir, Jobs: []workercontract.WorkerJob{}}
	ids, err := service.Effects.ListIDs(dir)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		job, readErr := service.Effects.Read(id)
		if readErr != nil || job.Status != workercontract.WorkerStatusRunning || service.Effects.PIDAlive(job.PID) {
			continue
		}
		fixed := false
		lockErr := service.Effects.WithLock(context.Background(), dir, id, func(context.Context) error {
			current, reReadErr := service.Effects.Read(id)
			if reReadErr != nil {
				return reReadErr
			}
			if current.Status != workercontract.WorkerStatusRunning || service.Effects.PIDAlive(current.PID) {
				return nil
			}
			next, changed := workerdomain.MarkStuck(current, false, service.Effects.Now().UTC().Format(time.RFC3339Nano))
			if !changed {
				return nil
			}
			if err := service.Effects.Write(next); err != nil {
				return err
			}
			job = next
			fixed = true
			return nil
		})
		if lockErr == nil && fixed {
			result.Jobs = append(result.Jobs, job)
		}
	}
	sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].CreatedAt > result.Jobs[j].CreatedAt })
	return result, nil
}
