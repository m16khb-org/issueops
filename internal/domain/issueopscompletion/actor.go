package issueopscompletion

import (
	"fmt"
	"strings"
)

// NormalizeActor validates the supplied identity against observed ancestry.
func NormalizeActor(actor Actor, ancestry []ProcessReceipt) (Actor, error) {
	actor.Host = strings.ToLower(strings.TrimSpace(actor.Host))
	actor.SessionID = strings.TrimSpace(actor.SessionID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	if actor.Process != nil {
		process := *actor.Process
		process.StartedAt = strings.TrimSpace(process.StartedAt)
		process.Executable = strings.TrimSpace(process.Executable)
		actor.Process = &process
	}
	if actor.Host != "codex" && actor.Host != "claude" && actor.Host != "omo" {
		return Actor{}, fmt.Errorf("native actor host must be codex, claude, or omo")
	}
	if actor.SessionID == "" {
		return Actor{}, fmt.Errorf("native actor session_id is required")
	}
	if actor.Process == nil || actor.Process.PID <= 0 || actor.Process.StartedAt == "" || actor.Process.Executable == "" {
		return Actor{}, fmt.Errorf("native actor requires a PID reuse-safe session_process receipt")
	}
	found := false
	for _, observed := range ancestry {
		if observed == *actor.Process {
			found = true
			break
		}
	}
	if !found {
		return Actor{}, fmt.Errorf("native session process receipt is not in the local process ancestry")
	}
	return actor, nil
}

// ValidateLiveActor checks a normalized actor against the process observation.
func ValidateLiveActor(actor Actor, status string, observed ProcessReceipt) error {
	if status != "live" {
		return fmt.Errorf("native process identity is not live: pid=%d status=%s", actor.Process.PID, status)
	}
	if observed != *actor.Process {
		return fmt.Errorf("native process identity does not match live PID %d", actor.Process.PID)
	}
	return nil
}
