package issueopscli

import (
	"flag"
	"issueops/cmd/issueops/issueopscli/actorflags"
	issueopscontract "issueops/internal/contract/issueops"
	"os"
)

type issueOpsActorFlags struct {
	flags   actorflags.Flags
	observe func(int) ([]issueopscontract.NativeProcessReceipt, error)
}

func (cli command) addIssueOpsActorFlags(fs *flag.FlagSet) issueOpsActorFlags {
	return issueOpsActorFlags{flags: actorflags.Add(fs), observe: cli.Runtime.ObserveNativeProcessAncestry}
}

func (flags issueOpsActorFlags) actor() issueopscontract.IssueOpsActor {
	ancestry, _ := flags.observe(os.Getpid())
	return flags.flags.ActorWithAncestry(ancestry)
}
