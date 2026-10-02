package basiccli

import (
	docsapp "issueops/internal/application/docs"
	"os"
	"time"

	docsadapter "issueops/internal/adapter/docs"
	guardadapter "issueops/internal/adapter/guard"
	inspectadapter "issueops/internal/adapter/inspect"
	statestore "issueops/internal/adapter/outbound/state"
	preflightadapter "issueops/internal/adapter/preflight"
	traceadapter "issueops/internal/adapter/trace"
	guardapp "issueops/internal/application/guard"
	preflightapp "issueops/internal/application/preflight"
	traceapp "issueops/internal/application/trace"
	inspectcontract "issueops/internal/contract/inspect"
)

func testBasicCommand() Command {
	root := testIssueOpsRoot()
	target := testResolveTarget("")
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	return Command{IssueOpsRoot: root, DefaultTarget: target, Version: "0.1.0", DocsIndex: (docsapp.Service{Observer: docsadapter.Observer{}, Now: time.Now}).Index,
		InspectHarness: func(repo string, options inspectcontract.Options) inspectcontract.InspectInfo {
			if repo == "" {
				repo = target
			}
			return (inspectadapter.Observer{ListDocs: (docsapp.Service{Observer: docsadapter.Observer{}, Now: time.Now}).List}).Inspect(root, repo, home, "0.1.0", "atomic-commit-push", options)
		},
		Preflight: preflightapp.Service{Observer: preflightadapter.GitObserver{}}, Guard: guardapp.Service{Source: guardadapter.Source{BaseDir: cwd}}, Trace: traceapp.Service{Effects: traceadapter.Source{ReadState: statestore.StateRead}}}
}
func RunDocs(args []string) error { return testBasicCommand().RunDocs(args) }
func RunDocsWithRoot(args []string, root string) error {
	c := testBasicCommand()
	c.IssueOpsRoot = root
	return c.RunDocs(args)
}
func RunPreflight(args []string) error    { return testBasicCommand().RunPreflight(args) }
func RunInspect(args []string) error      { return testBasicCommand().RunInspect(args) }
func RunTrace(args []string) error        { return testBasicCommand().RunTrace(args) }
func RunTraceAnalyze(args []string) error { return testBasicCommand().runTraceAnalyze(args) }
func RunGuard(args []string) error        { return testBasicCommand().RunGuard(args) }
func RunGuardCheck(args []string) error   { return testBasicCommand().runGuardCheck(args) }
