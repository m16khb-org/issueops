package statuscli

import (
	"flag"
	"fmt"
	statusapp "issueops/internal/application/status"
)

type Command struct{ Service statusapp.Service }

func (command Command) Run(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	status := command.Service.Run(*repo)
	if *jsonOut {
		return printJSON(status)
	}
	fmt.Printf("issueops system-status: ok=%v repo=%s\n", status.OK, status.Repo)
	fmt.Printf("doctor healthy: %v\n", status.Doctor.Healthy)
	fmt.Printf("daemon running: %v (%s)\n", status.Daemon.Running, status.Daemon.Message)
	fmt.Printf("state records: %d\n", len(status.State.Records))
	fmt.Printf("worker jobs: %d\n", len(status.Workers.Jobs))
	for _, warning := range status.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}
