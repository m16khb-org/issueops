package updatecli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	updateapp "issueops/internal/application/update"
)

var installScriptCommandRunner = runInstallScriptExec

func runUpdate(args []string) error {
	return runInstallScriptCommand("update", args)
}

func runBootstrap(args []string) error {
	return runInstallScriptCommand("bootstrap", args)
}

func runInstallScriptCommand(commandName string, args []string) error {
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

	return (updateapp.Service{Installer: updateInstaller{}}).Run(updateapp.Options{
		Root: deps.IssueOpsRoot(), ProjectLocal: *projectLocal, DryRun: *dryRun,
		PathMode: *pathMode, Interactive: *interactive, JSON: *jsonOut, SkipBuild: *skipBuild,
	})
}

func runInstallScriptExec(script string, args ...string) error {
	cmd := exec.Command(script, args...)
	cmd.Dir = deps.IssueOpsRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
