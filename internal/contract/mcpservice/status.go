package mcpservice

const (
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusStale    = "stale"
	StatusConflict = "conflict"
)

type Status struct {
	OK        bool   `json:"ok"`
	Status    string `json:"status"`
	PID       int    `json:"pid"`
	BuildID   string `json:"build_id"`
	URL       string `json:"url"`
	ErrorCode string `json:"error_code"`
}
