// Package worker holds the pure lifecycle rules of local worker jobs: id and kind validation,
// and the queued, running, finished, cancelled, and stuck transitions plus queue statistics.
// It performs no I/O; persistence and process execution belong to the application and adapter layers.
package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	workercontract "issueops/internal/contract/worker"
)

// ValidID matches ^[A-Za-z0-9._-]{1,128}$ by hand (the counted repetition
// compiles to a large regexp program at every process start) and rejects "..".
func ValidID(id string) bool {
	if len(id) == 0 || len(id) > 128 || strings.Contains(id, "..") {
		return false
	}
	for i := range len(id) {
		if !validIDByte(id[i]) {
			return false
		}
	}
	return true
}

func validIDByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-'
}

// MakeID returns a job id derived from the kind, payload, and creation time, so the same inputs at the same instant yield the same id.
func MakeID(kind, payload string, at time.Time) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + payload + "\x00" + at.Format(time.RFC3339Nano)))
	return "job-" + at.Format("20060102T150405Z") + "-" + hex.EncodeToString(sum[:])[:12]
}

// NormalizeKind trims kind and rejects empty kinds, kinds with path separators, and kinds longer than 80 bytes.
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

// NewQueued builds a queued, no-shell job record from the redacted payload; it fails when kind is invalid.
func NewQueued(kind, redactedPayload, id, dir, timestamp string) (workercontract.Job, error) {
	var err error
	kind, err = NormalizeKind(kind)
	if err != nil {
		return workercontract.Job{OK: false}, err
	}
	return workercontract.Job{
		OK: true, ID: id, Kind: kind, Status: workercontract.StatusQueued,
		Payload: redactedPayload, CreatedAt: timestamp, UpdatedAt: timestamp, WorkerDir: dir,
		NoShell: true, SafetyNotice: "worker MVP records lifecycle state only; it never executes shell commands",
	}, nil
}

// Cancel marks a queued job cancelled. An already cancelled job is returned unchanged; any other status is an error.
func Cancel(job workercontract.Job, timestamp string) (workercontract.Job, error) {
	if job.Status == workercontract.StatusCancelled {
		return job, nil
	}
	if job.Status != workercontract.StatusQueued {
		return job, fmt.Errorf("worker job %s cannot be cancelled from status %s", job.ID, job.Status)
	}
	job.Status = workercontract.StatusCancelled
	job.UpdatedAt = timestamp
	job.OK = true
	return job, nil
}

// Start moves a queued job to running, recording the command, PID, and start time.
// It fails for any status other than queued.
func Start(job workercontract.Job, command []string, pid int, startedAt, updatedAt string) (workercontract.Job, error) {
	if job.Status != workercontract.StatusQueued {
		return job, fmt.Errorf("worker job %s cannot run from status %s", job.ID, job.Status)
	}
	job.Status = workercontract.StatusRunning
	job.StartedAt = startedAt
	job.PID = pid
	job.NoShell = true
	job.Command = append([]string{}, command...)
	job.SafetyNotice = "worker read-only runner executes only argv commands that pass command policy with write/network/shell disabled"
	job.UpdatedAt = updatedAt
	return job, nil
}

// Finish moves a job to succeeded when ok and to failed otherwise, stamping timestamp.
func Finish(job workercontract.Job, ok bool, timestamp string) workercontract.Job {
	job.OK = ok
	if ok {
		job.Status = workercontract.StatusSucceeded
	} else {
		job.Status = workercontract.StatusFailed
	}
	job.UpdatedAt = timestamp
	return job
}

// MarkStuck fails a running job whose process is no longer alive and reports whether it changed the job.
// Jobs that are not running, or whose process is alive, are returned unchanged.
func MarkStuck(job workercontract.Job, alive bool, timestamp string) (workercontract.Job, bool) {
	if job.Status != workercontract.StatusRunning || alive {
		return job, false
	}
	job.Status = workercontract.StatusFailed
	job.UpdatedAt = timestamp
	job.OK = false
	job.Result = nil
	job.SafetyNotice = "worker job was stuck in running status with dead PID; auto-marked as failed"
	return job, true
}

// QueueStats counts jobs by status.
func QueueStats(jobs []workercontract.Job) *workercontract.QueueStats {
	stats := &workercontract.QueueStats{Total: len(jobs)}
	for _, job := range jobs {
		switch job.Status {
		case workercontract.StatusQueued:
			stats.Queued++
		case workercontract.StatusRunning:
			stats.Running++
		case workercontract.StatusSucceeded:
			stats.Succeeded++
		case workercontract.StatusFailed:
			stats.Failed++
		case workercontract.StatusCancelled:
			stats.Cancelled++
		}
	}
	stats.Depth = stats.Queued + stats.Running
	return stats
}
