package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ValidateNativeActor(actor model.NativeActor) error {
	if actor.Host != "codex" && actor.Host != "claude" && actor.Host != "omo" {
		return fmt.Errorf("native actor host must be codex, claude, or omo")
	}
	if strings.TrimSpace(actor.SessionID) == "" {
		return fmt.Errorf("native actor session_id is required")
	}
	if actor.SessionProcess == nil || actor.SessionProcess.PID <= 0 || actor.SessionProcess.StartedAt == "" || actor.SessionProcess.Executable == "" {
		return fmt.Errorf("native actor requires a PID reuse-safe session_process receipt")
	}
	return nil
}

func NormalizeNativeActor(actor model.NativeActor) (model.NativeActor, error) {
	actor.Host = strings.ToLower(strings.TrimSpace(actor.Host))
	actor.SessionID = strings.TrimSpace(actor.SessionID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	if actor.SessionProcess != nil {
		receipt := *actor.SessionProcess
		receipt.StartedAt = strings.TrimSpace(receipt.StartedAt)
		receipt.Executable = strings.TrimSpace(receipt.Executable)
		actor.SessionProcess = &receipt
	}
	actor.ProcessAncestry = append([]model.NativeProcessReceipt(nil), actor.ProcessAncestry...)
	if err := ValidateNativeActor(actor); err != nil {
		return actor, err
	}
	for _, receipt := range actor.ProcessAncestry {
		if receipt == *actor.SessionProcess {
			return actor, nil
		}
	}
	return actor, fmt.Errorf("native session process receipt is not in the local process ancestry")
}

func ValidateObservedNativeProcess(receipt model.NativeProcessReceipt, status string, observed model.NativeProcessReceipt) error {
	if status != "live" {
		return fmt.Errorf("native process identity is not live: pid=%d status=%s", receipt.PID, status)
	}
	if observed.StartedAt != receipt.StartedAt || observed.Executable != receipt.Executable {
		return fmt.Errorf("native process identity does not match live PID %d", receipt.PID)
	}
	return nil
}
