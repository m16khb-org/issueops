package issueopsapp

import (
	"encoding/json"
	"io"
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

func printJSONTo(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func issueOpsRoot() string {
	return pathutil.IssueOpsRoot(filepath.Join("skills", skillName, "SKILL.md"))
}

func resolveTarget(arg string) string {
	return pathutil.ResolveTarget(arg)
}

func exists(path string) bool {
	return pathutil.Exists(path)
}

func runAPIDoc(args []string) error {
	return newAPIDocCommand().Run(args)
}
