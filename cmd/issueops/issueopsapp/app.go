package issueopsapp

import (
	"fmt"
	"io"
	"os"

	cliadapter "issueops/internal/adapter/inbound/catalog/cli"
	inspect "issueops/internal/adapter/inspect"
	inspectcontract "issueops/internal/contract/inspect"
)

const version = "0.1.0"
const skillName = "atomic-commit-push"
const selfVerifyCommandOutputBudgetBytes = 32 * 1024
const selfVerifyAggregateOutputBudgetBytes = 8 * 1024

func usage() {
	fprintUsage(os.Stderr)
}

func fprintUsage(w io.Writer) {
	fprintString(w, cliadapter.Usage(version))
}

func fprintString(w io.Writer, text string) {
	_, _ = fmt.Fprint(w, text)
}

func newHarnessInspector() func(string) inspectcontract.InspectInfo {
	root := issueOpsRoot()
	home, _ := os.UserHomeDir()
	defaultTarget := resolveTarget("")
	observer := inspect.Observer{ListDocs: newDocsService().List}
	return func(target string) inspectcontract.InspectInfo {
		if target == "" {
			target = defaultTarget
		}
		return observer.Inspect(root, target, home, version, skillName)
	}
}

func printJSON(v any) error {
	return printJSONTo(os.Stdout, v)
}
