// Package mcpservice holds the contract DTO describing the managed MCP HTTP
// service status shared by the service adapter and the install/CLI callers.
package mcpservice

// Status values reported for the managed MCP HTTP service: running, stopped,
// stale (a published record whose process is gone) and conflict (observations
// that disagree).
const (
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusStale    = "stale"
	StatusConflict = "conflict"
)

// Status is the observed state of the managed MCP HTTP service. Status holds
// one of the Status* constants (or "" when no state could be determined);
// ErrorCode is set when OK is false.
type Status struct {
	OK        bool   `json:"ok"`
	Status    string `json:"status"`
	PID       int    `json:"pid"`
	BuildID   string `json:"build_id"`
	URL       string `json:"url"`
	ErrorCode string `json:"error_code"`
}
