// Package actorflags declares RECORD_ACTOR_FLAGS once for every record-style
// IssueOps command: --host, --session-id, --agent-id, --cwd and the optional
// native session receipt (--session-pid, --session-started-at,
// --session-executable) that `execution whoami` prints in claim_actor_flags,
// so one copied actor vector works for every IssueOps mutation.
//
// The live native process ancestry stays the authority. A supplied receipt
// must name, all three fields together, one entry of that ancestry; otherwise
// the ancestry is withheld and authorization fails closed.
package actorflags

import (
	"flag"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

type Flags struct {
	host       *string
	sessionID  *string
	agentID    *string
	cwd        *string
	pid        *int
	startedAt  *string
	executable *string
}

// Add registers RECORD_ACTOR_FLAGS on fs.
func Add(fs *flag.FlagSet) Flags {
	return Flags{
		host:       fs.String("host", "", "native actor host"),
		sessionID:  fs.String("session-id", "", "native actor session id"),
		agentID:    fs.String("agent-id", "", "optional native actor agent id"),
		cwd:        fs.String("cwd", "", "canonical actor cwd"),
		pid:        fs.Int("session-pid", 0, "optional native session process id (must match the live ancestry)"),
		startedAt:  fs.String("session-started-at", "", "optional native session process start time"),
		executable: fs.String("session-executable", "", "optional native session executable"),
	}
}

// Actor returns the declared actor without process ancestry; pair it with
// Observer when the core observes ancestry itself.
func (f Flags) Actor() issueopscontract.IssueOpsActor {
	return issueopscontract.IssueOpsActor{Host: *f.host, SessionID: *f.sessionID, AgentID: *f.agentID, CWD: *f.cwd}
}

// ActorWithAncestry returns the declared actor carrying the observed ancestry
// after Filter. An observation error leaves the ancestry empty (fail closed).
func (f Flags) ActorWithAncestry(ancestry []issueopscontract.NativeProcessReceipt) issueopscontract.IssueOpsActor {
	actor := f.Actor()
	actor.NativeProcessAncestry = f.Filter(ancestry)
	return actor
}

// Filter returns ancestry when no receipt was supplied or the supplied receipt
// is one of its entries, and nil otherwise.
func (f Flags) Filter(ancestry []issueopscontract.NativeProcessReceipt) []issueopscontract.NativeProcessReceipt {
	pid, startedAt, executable := *f.pid, strings.TrimSpace(*f.startedAt), strings.TrimSpace(*f.executable)
	if pid == 0 && startedAt == "" && executable == "" {
		return ancestry
	}
	if pid == 0 || startedAt == "" || executable == "" {
		return nil
	}
	for _, receipt := range ancestry {
		if receipt.PID == pid && receipt.StartedAt == startedAt && receipt.Executable == executable {
			return ancestry
		}
	}
	return nil
}

// Observer wraps an ancestry observer with Filter.
func (f Flags) Observer(observe func() ([]issueopscontract.NativeProcessReceipt, error)) func() ([]issueopscontract.NativeProcessReceipt, error) {
	return func() ([]issueopscontract.NativeProcessReceipt, error) {
		ancestry, err := observe()
		if err != nil {
			return nil, err
		}
		return f.Filter(ancestry), nil
	}
}
