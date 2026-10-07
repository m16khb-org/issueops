package issueopsapp

import (
	"path/filepath"

	"issueops/cmd/issueops/contractcli"
	"issueops/cmd/issueops/pathutil"
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
)

func runContract(args []string) error {
	return contractcli.Run(args, clicatalog.Commands(), mcpTools(), newContractConformance())
}

func compatibilityContract() contractcli.CompatibilityContract {
	return contractcli.BuildCompatibilityContract(clicatalog.Commands(), mcpTools())
}

func issueOpsRoot() string {
	return pathutil.IssueOpsRoot(filepath.Join("skills", skillName, "SKILL.md"))
}

func runAPIDoc(args []string) error {
	return newAPIDocCommand().Run(args)
}
