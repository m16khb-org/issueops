package statuscli

import (
	"flag"
	"fmt"

	"issueops/cmd/issueops/cliflag"
	statusapp "issueops/internal/application/status"
)

type Command struct{ Service statusapp.Service }

func (command Command) Run(args []string) error {
	fs := flag.NewFlagSet("system-status", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	positional, err := cliflag.ParseOptionalPositional(fs, args)
	if err != nil {
		return err
	}
	if positional != "" {
		*repo = positional
	}
	status := command.Service.Run(*repo)
	if *jsonOut {
		return printJSON(status)
	}
	fmt.Printf("issueops system-status: ok=%v repo=%s\n", status.OK, status.Repo)
	fmt.Printf("doctor healthy: %v\n", status.Doctor.Healthy)
	fmt.Printf("state records: %d\n", len(status.State.Records))
	fmt.Printf("worker jobs: %d\n", len(status.Workers.Jobs))
	for _, warning := range status.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}
