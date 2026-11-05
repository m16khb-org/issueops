// Package updatecli implements the install/update CLI commands: the bootstrap
// install flow and MCP proxy cleanup. It parses flags and delegates to the update
// application service.
package updatecli

import (
	"errors"
	"flag"
	"fmt"

	"issueops/cmd/issueops/cliflag"
	updateapp "issueops/internal/application/update"
)

// Command runs the install/update bootstrap for the harness at Root through
// Service.
type Command struct {
	Root    string
	Service updateapp.Service
}

// Run parses the install flags for commandName (project-local, dry-run, path-mode,
// interactive, json, skip-build, mcp-transport) and runs the update service.
// --help returns nil and unexpected positional arguments are an error.
func (command Command) Run(commandName string, args []string) error {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	projectLocal := fs.Bool("project-local", false, "also write explicit project-local files")
	dryRun := fs.Bool("dry-run", false, "show install plan without writing")
	pathMode := fs.String("path-mode", "", "manage ~/.local/bin PATH setup: auto, manual, or skip")
	interactive := fs.Bool("interactive", false, "ask for install choices before applying the plan")
	jsonOut := fs.Bool("json", false, "print JSON from install")
	skipBuild := fs.Bool("skip-build", false, "do not rebuild bin/issueops")
	mcpTransport := fs.String("mcp-transport", "", "Codex/Claude/Omo/omp/opencode MCP transport: http (shared service) or stdio; default http on darwin/linux")
	if err := cliflag.Parse(fs, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	return command.Service.Run(updateapp.Options{
		Root: command.Root, ProjectLocal: *projectLocal, DryRun: *dryRun,
		PathMode: *pathMode, Interactive: *interactive, JSON: *jsonOut, SkipBuild: *skipBuild, MCPTransport: *mcpTransport,
	})
}
