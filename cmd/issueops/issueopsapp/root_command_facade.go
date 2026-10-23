package issueopsapp

import (
	"errors"
	"os"
	"sync"

	policycontract "issueops/internal/contract/policy"

	"issueops/cmd/issueops/channelcli"
	"issueops/cmd/issueops/gatescli"
	"issueops/cmd/issueops/jsonout"
	"issueops/cmd/issueops/modelcli"
	"issueops/cmd/issueops/rootcmd"
	clicontract "issueops/internal/contract/cli"
	guard "issueops/internal/contract/guard"
)

var dependencyWiring sync.Once

func RunRootCommand(args []string) int {
	wireDependencies()
	if rootcmd.HelpRequested(args) {
		// Usage is the answer to a help request, not a diagnostic: print it on
		// stdout so `issueops X --help | grep` works. Subcommands write usage
		// to os.Stderr directly (flag.FlagSet default output).
		os.Stderr = os.Stdout
	}
	return rootCommand().Run(args)
}

// wireDependencies injects the harness implementations into the leaf CLI
// packages. The composition root calls this explicitly at startup instead of
// relying on package init() side effects, so dependency wiring is ordered,
// visible, and not import-order sensitive.
func wireDependencies() {
	dependencyWiring.Do(func() {
		wireBasicCLIDeps()
	})
}

func rootCommand() rootcmd.Command {
	command := rootcmd.Command{
		Version: version,
		Usage:   usage,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Runners: map[string]rootcmd.Runner{
			"inspect":       runInspect,
			"preflight":     runPreflight,
			"system-status": runStatus,
			"doctor":        runDoctor,
			"docs":          runDocs,
			"policy":        runPolicy,
			"verify-work":   runVerifyWork,
			"trace":         runTrace,
			"guard":         runGuard,
			"quality":       runQuality,
			"self-verify":   runSelfVerify,
			"self-augment":  runSelfAugment,
			"contract":      runContract,
			"state":         runState,
			"api-doc":       runAPIDoc,
			"hook":          runHook,
			"project":       runProject,
			"install":       runInstall,
			"update":        runUpdate,
			"bootstrap":     runBootstrap,
			"worker":        runWorker,
			"loop":          runLoop,
			"gates":         runGates,
			"channel":       runChannel,
			"web-fetch":     runWebFetch,
			"mcp":           runMCPCommand,
			"model":         runModel,
		},
		ErrorExitCode: rootSubcommandErrorExitCode,
	}
	for _, lifecycle := range clicontract.LifecycleCommands() {
		name := lifecycle.Name
		command.Runners[name] = func(args []string) error { return runIssueOps(append([]string{name}, args...)) }
	}
	return command
}

func rootSubcommandErrorExitCode(name string, err error) int {
	err = jsonout.Unreported(err)
	switch name {
	case "policy":
		if policycontract.IsPolicyDenied(err) {
			return 3
		}
	case "guard":
		if guard.IsGuardBlocked(err) {
			return 3
		}
	case "gates":
		if _, ok := errors.AsType[gatescli.UsageError](err); ok {
			return 2
		}
	case "channel":
		if _, ok := errors.AsType[channelcli.TimedOutError](err); ok {
			return 1
		}
	case "model":
		if _, ok := errors.AsType[modelcli.UsageError](err); ok {
			return 2
		}
	}
	return 1
}
