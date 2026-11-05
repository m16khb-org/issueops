// Package worker 패키지는 worker capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package worker

import policycontract "issueops/internal/contract/policy"

// Job is the persisted record of one local worker job. Command is an argv run
// without a shell; Result is set once the command has finished.
type Job struct {
	OK           bool                             `json:"ok"`
	ID           string                           `json:"id"`
	Kind         string                           `json:"kind"`
	Status       string                           `json:"status"`
	Payload      string                           `json:"payload,omitempty"`
	CreatedAt    string                           `json:"created_at"`
	UpdatedAt    string                           `json:"updated_at"`
	StartedAt    string                           `json:"started_at,omitempty"`
	PID          int                              `json:"pid,omitempty"`
	WorkerDir    string                           `json:"worker_dir"`
	NoShell      bool                             `json:"no_shell"`
	SafetyNotice string                           `json:"safety_notice"`
	Command      []string                         `json:"command,omitempty"`
	Result       *policycontract.CommandRunResult `json:"result,omitempty"`
}

// QueueStats is a status histogram over all worker jobs (A2/G6). Depth is
// the saturation signal — only non-terminal work (queued+running) — so a backlog
// is visible before jobs time out, unlike the raw per-status counts alone.
type QueueStats struct {
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
	Total     int `json:"total"`
	Depth     int `json:"depth"`
}

// ListResult is a listing of worker jobs rooted at WorkerDir.
type ListResult struct {
	OK        bool   `json:"ok"`
	WorkerDir string `json:"worker_dir"`
	Jobs      []Job  `json:"jobs"`
	// Queue is the saturation histogram; populated by ListWorkerJobs (the full
	// listing) and left nil by DetectStuckWorkerJobs, whose Jobs is only the
	// fixed subset and would make a histogram misleading.
	Queue *QueueStats `json:"queue,omitempty"`
}

// Worker job statuses. Queued and running are non-terminal; the rest are final.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)
