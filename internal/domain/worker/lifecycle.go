package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	workercontract "issueops/internal/contract/worker"
)

var workerIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func ValidID(id string) bool { return workerIDPattern.MatchString(id) && !strings.Contains(id, "..") }

func MakeID(kind, payload string, at time.Time) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + payload + "\x00" + at.Format(time.RFC3339Nano)))
	return "job-" + at.Format("20060102T150405Z") + "-" + hex.EncodeToString(sum[:])[:12]
}

func NormalizeKind(kind string) (string, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "", fmt.Errorf("worker job kind is required")
	}
	if strings.ContainsAny(kind, `/\`) || len(kind) > 80 {
		return "", fmt.Errorf("invalid worker job kind")
	}
	return kind, nil
}

func NewQueued(kind, redactedPayload, id, dir, timestamp string) (workercontract.WorkerJob, error) {
	var err error
	kind, err = NormalizeKind(kind)
	if err != nil {
		return workercontract.WorkerJob{OK: false}, err
	}
	return workercontract.WorkerJob{
		OK: true, ID: id, Kind: kind, Status: workercontract.WorkerStatusQueued,
		Payload: redactedPayload, CreatedAt: timestamp, UpdatedAt: timestamp, WorkerDir: dir,
		NoShell: true, SafetyNotice: "worker MVP records lifecycle state only; it never executes shell commands",
	}, nil
}

func Cancel(job workercontract.WorkerJob, timestamp string) (workercontract.WorkerJob, error) {
	if job.Status == workercontract.WorkerStatusCancelled {
		return job, nil
	}
	if job.Status != workercontract.WorkerStatusQueued {
		return job, fmt.Errorf("worker job %s cannot be cancelled from status %s", job.ID, job.Status)
	}
	job.Status = workercontract.WorkerStatusCancelled
	job.UpdatedAt = timestamp
	job.OK = true
	return job, nil
}

func Start(job workercontract.WorkerJob, command []string, pid int, startedAt, updatedAt string) (workercontract.WorkerJob, error) {
	if job.Status != workercontract.WorkerStatusQueued {
		return job, fmt.Errorf("worker job %s cannot run from status %s", job.ID, job.Status)
	}
	job.Status = workercontract.WorkerStatusRunning
	job.StartedAt = startedAt
	job.PID = pid
	job.NoShell = true
	job.Command = append([]string{}, command...)
	job.SafetyNotice = "worker read-only runner executes only argv commands that pass command policy with write/network/shell disabled"
	job.UpdatedAt = updatedAt
	return job, nil
}

func Finish(job workercontract.WorkerJob, ok bool, timestamp string) workercontract.WorkerJob {
	job.OK = ok
	if ok {
		job.Status = workercontract.WorkerStatusSucceeded
	} else {
		job.Status = workercontract.WorkerStatusFailed
	}
	job.UpdatedAt = timestamp
	return job
}

func MarkStuck(job workercontract.WorkerJob, alive bool, timestamp string) (workercontract.WorkerJob, bool) {
	if job.Status != workercontract.WorkerStatusRunning || alive {
		return job, false
	}
	job.Status = workercontract.WorkerStatusFailed
	job.UpdatedAt = timestamp
	job.OK = false
	job.Result = nil
	job.SafetyNotice = "worker job was stuck in running status with dead PID; auto-marked as failed"
	return job, true
}

func QueueStats(jobs []workercontract.WorkerJob) *workercontract.WorkerQueueStats {
	stats := &workercontract.WorkerQueueStats{Total: len(jobs)}
	for _, job := range jobs {
		switch job.Status {
		case workercontract.WorkerStatusQueued:
			stats.Queued++
		case workercontract.WorkerStatusRunning:
			stats.Running++
		case workercontract.WorkerStatusSucceeded:
			stats.Succeeded++
		case workercontract.WorkerStatusFailed:
			stats.Failed++
		case workercontract.WorkerStatusCancelled:
			stats.Cancelled++
		}
	}
	stats.Depth = stats.Queued + stats.Running
	return stats
}
