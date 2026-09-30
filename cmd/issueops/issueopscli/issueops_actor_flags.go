package issueopscli

import (
	"flag"
	issueopscontract "issueops/internal/contract/issueops"
	"os"
)

type issueOpsActorFlags struct {
	host      *string
	sessionID *string
	agentID   *string
	cwd       *string
	observe   func(int) ([]issueopscontract.NativeProcessReceipt, error)
}

func (cli command) addIssueOpsActorFlags(fs *flag.FlagSet) issueOpsActorFlags {
	return issueOpsActorFlags{
		observe: cli.Runtime.ObserveNativeProcessAncestry,
		host:    fs.String("host", "", "native actor host"), sessionID: fs.String("session-id", "", "native actor session id"),
		agentID: fs.String("agent-id", "", "native actor agent id"), cwd: fs.String("cwd", "", "canonical actor cwd"),
	}
}

func (flags issueOpsActorFlags) actor() issueopscontract.IssueOpsActor {
	ancestry, _ := flags.observe(os.Getpid())
	return issueopscontract.IssueOpsActor{
		Host: *flags.host, SessionID: *flags.sessionID, AgentID: *flags.agentID, CWD: *flags.cwd,
		NativeProcessAncestry: ancestry,
	}
}
