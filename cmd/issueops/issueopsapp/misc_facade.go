package issueopsapp

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"issueops/cmd/issueops/apidoc"
	"issueops/cmd/issueops/contractcli"
	"issueops/cmd/issueops/pathutil"
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	app "issueops/internal/application/selfverify"
)

type CompatibilityContract = contractcli.CompatibilityContract

var (
	errSelfVerificationGateFailed = app.ErrSelfVerificationGateFailed
)

func runContract(args []string) error {
	return contractcli.Run(args, clicatalog.Commands(), mcpTools(), newContractConformance())
}

func compatibilityContract() CompatibilityContract {
	return contractcli.BuildCompatibilityContract(clicatalog.Commands(), mcpTools())
}

func isSelfVerificationGateError(err error) bool {
	return errors.Is(err, errSelfVerificationGateFailed)
}

func printJSONTo(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func readHarnessFile(parts ...string) (string, error) {
	return pathutil.ReadHarnessFile(issueOpsRoot(), parts...)
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
	return apidoc.Run(args)
}
