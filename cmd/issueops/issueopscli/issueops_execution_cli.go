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
		BaseSync:    deps.BaseSync,
		Claim:       deps.Claim,
		Release:     deps.Release,
		Status:      deps.Status,
		Replace:     deps.Replace,
		Reseed:      deps.Reseed,
		Complete:    deps.Complete,
		Publication: deps.Publication,
		Provenance:  deps.Provenance,
		HandoffCmux: deps.HandoffCmux,
		PrintJSON:   printJSON,
		PrintError:  printIssueOpsErrorJSON,
	}
}
