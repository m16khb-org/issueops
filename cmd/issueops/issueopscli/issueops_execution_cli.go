package issueopscli

import (
	"issueops/cmd/issueops/issueopscli/executioncmd"
)

func (cli command) runIssueOpsExecutionWithDependencies(args []string, deps Dependencies) error {
	return executioncmd.Run(args, cli.issueOpsExecutionDeps(deps))
}

func (cli command) issueOpsExecutionDeps(deps Dependencies) executioncmd.Deps {
	return executioncmd.Deps{
		Runtime:     deps.Execution,
		StateRoot:   cli.Runtime.IssueOpsStateRoot,
		Prepare:     deps.Prepare,
		Orca:        deps.Orca,
		OrcaOwner:   deps.OrcaOwner,
		BaseSync:    deps.BaseSync,
		ReadIssue:   deps.ReadIssue,
		Claim:       deps.Claim,
		Release:     deps.Release,
		Replace:     deps.Replace,
		Reseed:      deps.Reseed,
		Resume:      deps.Resume,
		Reconcile:   deps.Reconcile,
		Complete:    deps.Complete,
		Publication: deps.Publication,
		Provenance:  deps.Provenance,
		HandoffCmux: deps.HandoffCmux,
		PrintJSON:   printJSON,
		PrintError:  printIssueOpsErrorJSON,
	}
}
