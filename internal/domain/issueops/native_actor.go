package issueops

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	model "issueops/internal/contract/issueops"
)

// ActorReceiptRemedy names what a caller missing the session receipt passes:
// agents copying the shorter record flags into execution commands hit this.
const ActorReceiptRemedy = "pass every ACTOR_FLAG, including --session-pid, --session-started-at, and --session-executable (issueops execution whoami --json prints them as claim_actor_flags)"

// EmptyActorHostMessage answers an empty --host: in QA it came from an unset
// shell variable, which "must be codex, claude, ..." did not point at.
const EmptyActorHostMessage = "native actor host is empty: pass --host codex|claude|omo|omp|opencode; issueops execution whoami --json prints the whole flag set"

func ValidateNativeActor(actor model.NativeActor) error {
	switch actor.Host {
	case "codex", "claude", "omo", "omp", "opencode":
	case "":
		return fmt.Errorf("%s", EmptyActorHostMessage)
	default:
		return fmt.Errorf("native actor host must be codex, claude, omo, omp, or opencode")
	}
	if strings.TrimSpace(actor.SessionID) == "" {
		return fmt.Errorf("native actor session_id is required")
	}
	if actor.SessionProcess == nil || actor.SessionProcess.PID <= 0 || actor.SessionProcess.StartedAt == "" || actor.SessionProcess.Executable == "" {
		return fmt.Errorf("native actor requires a PID reuse-safe session_process receipt: %s", ActorReceiptRemedy)
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
	// An opencode session is proven only by its own server process, never by a
	// long-lived ancestor such as the shell the user started opencode from.
	if actor.Host == "opencode" && !IsOpencodeExecutable(actor.SessionProcess.Executable) {
		return actor, fmt.Errorf("opencode session process receipt must be the opencode standalone server")
	}
	if slices.Contains(actor.ProcessAncestry, *actor.SessionProcess) {
		return actor, nil
	}
	return actor, fmt.Errorf("native session process receipt is not in the local process ancestry: pass the claim_actor_flags that `issueops execution whoami --json` prints in this session; if this session was resumed after the process that took the lease exited, `issueops next --id <ID> --json` routes it to the takeover chain")
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

// IsOpencodeExecutable reports whether a process executable is the opencode
// binary, with the same normalization the CLI uses to pick its receipt.
func IsOpencodeExecutable(executable string) bool {
	normalized := strings.ToLower(filepath.ToSlash(strings.TrimSpace(executable)))
	return strings.TrimSuffix(filepath.Base(normalized), ".exe") == "opencode"
}

// OpencodeStandaloneServer reports whether whitespace-split process arguments
// belong to the per-client `opencode serve --stdio` server that `--standalone`
// starts. The first opencode token is the executable (earlier fields are pieces
// of a path with spaces), so the subcommand must follow it directly; a prompt
// that merely contains "opencode serve --stdio" does not match. A path whose
// earlier piece also ends in "opencode" fails closed.
func OpencodeStandaloneServer(fields []string) bool {
	for index, field := range fields {
		if !IsOpencodeExecutable(field) {
			continue
		}
		rest := fields[index+1:]
		return len(rest) > 0 && rest[0] == "serve" && slices.Contains(rest[1:], "--stdio")
	}
	return false
}
