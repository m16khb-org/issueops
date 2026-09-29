package updatecli

import (
	"flag"
	"fmt"

	updateapp "issueops/internal/application/update"
)

type Command struct {
	Root    string
	Service updateapp.Service
}

func (command Command) Run(commandName string, args []string) error {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	projectLocal := fs.Bool("project-local", false, "also write explicit project-local files")
	dryRun := fs.Bool("dry-run", false, "show install plan without writing")
	pathMode := fs.String("path-mode", "", "manage ~/.local/bin PATH setup: auto, manual, or skip")
	interactive := fs.Bool("interactive", false, "ask for install choices before applying the plan")
	jsonOut := fs.Bool("json", false, "print JSON from install")
	skipBuild := fs.Bool("skip-build", false, "do not rebuild bin/issueops")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	return command.Service.Run(updateapp.Options{
		Root: command.Root, ProjectLocal: *projectLocal, DryRun: *dryRun,
		PathMode: *pathMode, Interactive: *interactive, JSON: *jsonOut, SkipBuild: *skipBuild,
	})
}
