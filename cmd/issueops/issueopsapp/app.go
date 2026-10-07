package issueopsapp

import (
	"fmt"
	"io"
	"issueops/cmd/issueops/jsonout"
	pathutil "issueops/cmd/issueops/pathutil"
	"os"

	cliadapter "issueops/internal/adapter/inbound/catalog/cli"
	inspect "issueops/internal/adapter/inspect"
	inspectcontract "issueops/internal/contract/inspect"
)

const version = "0.1.0"
const skillName = "atomic-commit-push"
const selfVerifyCommandOutputBudgetBytes = 32 * 1024

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
	inspector := newHarnessHostInspector()
	return func(target string) inspectcontract.InspectInfo { return inspector(target, inspectcontract.Options{}) }
}

func newHarnessHostInspector() func(string, inspectcontract.Options) inspectcontract.InspectInfo {
	return harnessInspectorWithDefault(pathutil.ResolveTarget(""), "")
}

func scopedHarnessInspector(defaultTarget string) func(string, string) any {
	inspector := harnessInspectorWithDefault(defaultTarget, defaultTarget)
	return func(repo, hostReceipts string) any {
		return inspector(repo, inspectcontract.Options{HostReceipts: hostReceipts})
	}
}

func harnessInspectorWithDefault(defaultTarget, receiptRoot string) func(string, inspectcontract.Options) inspectcontract.InspectInfo {
	root := issueOpsRoot()
	home, _ := os.UserHomeDir()
	observer := inspect.Observer{ListDocs: newDocsService().List, HostVersion: inspect.CommandHostVersion, ReceiptRoot: receiptRoot}
	return func(target string, options inspectcontract.Options) inspectcontract.InspectInfo {
		if target == "" {
			target = defaultTarget
		}
		if options.CodexHome == "" {
			options.CodexHome = os.Getenv("CODEX_HOME")
		}
		return observer.Inspect(root, target, home, version, skillName, options)
	}
}

var printJSON = jsonout.Print
