package workercli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"issueops/cmd/issueops/cliflag"
	"issueops/cmd/issueops/jsonout"
)

func (command Command) RunEnqueue(args []string) error {
	fs := flag.NewFlagSet("worker enqueue", flag.ContinueOnError)
	kind := fs.String("kind", "", "job kind")
	payload := fs.String("payload", "", "redacted job payload")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}
	job, err := command.Service.Enqueue(context.Background(), *kind, *payload)
	if *jsonOut {
		return jsonout.PrintResultError(os.Stdout, job, err)
	}
	if err == nil && !*jsonOut {
		fmt.Printf("queued %s %s\n", job.ID, job.Kind)
	}
	return err
}

func (command Command) RunStatus(args []string) error {
	fs := flag.NewFlagSet("worker status", flag.ContinueOnError)
	id := fs.String("id", "", "job id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}
	job, err := command.Service.Read(*id)
	if *jsonOut {
		return jsonout.PrintResultError(os.Stdout, job, err)
	}
	if err == nil && !*jsonOut {
		fmt.Printf("%s %s\n", job.ID, job.Status)
	}
	return err
}

func (command Command) RunList(args []string) error {
	fs := flag.NewFlagSet("worker list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}
	result, err := command.Service.List()
	if *jsonOut {
		return jsonout.PrintResultError(os.Stdout, result, err)
	}
	if err == nil && !*jsonOut {
		for _, job := range result.Jobs {
			fmt.Printf("%s %s %s\n", job.ID, job.Status, job.Kind)
		}
	}
	return err
}

func (command Command) RunCleanupStuck(args []string) error {
	fs := flag.NewFlagSet("worker cleanup-stuck", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}
	result, err := command.Service.DetectStuck(context.Background())
	if *jsonOut {
		return jsonout.PrintResultError(os.Stdout, result, err)
	}
	if err == nil && !*jsonOut {
		for _, job := range result.Jobs {
			fmt.Printf("%s %s %s\n", job.ID, job.Status, job.Kind)
		}
	}
	return err
}

func (command Command) RunCancel(args []string) error {
	fs := flag.NewFlagSet("worker cancel", flag.ContinueOnError)
	id := fs.String("id", "", "job id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}
	job, err := command.Service.Cancel(context.Background(), *id)
	if *jsonOut {
		return jsonout.PrintResultError(os.Stdout, job, err)
	}
	if err == nil && !*jsonOut {
		fmt.Printf("cancelled %s\n", job.ID)
	}
	return err
}
