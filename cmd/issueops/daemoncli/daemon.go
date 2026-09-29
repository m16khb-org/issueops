package daemoncli

import (
	"flag"
	"fmt"
	daemondomain "issueops/internal/domain/daemon"
	"os"

	contract "issueops/internal/contract/daemon"
)

type Command struct {
	Start  func() (contract.Status, error)
	Status func() contract.Status
	Stop   func() (contract.Status, error)
	Serve  func() error
}

func (command Command) Run(args []string) error {
	if len(args) > 0 && args[0] == "--internal" {
		return command.Serve()
	}
	if len(args) == 0 {
		daemonUsage()
		return fmt.Errorf("missing daemon subcommand")
	}
	sub := args[0]
	fs := flag.NewFlagSet("daemon "+sub, flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch sub {
	case "start":
		status, err := command.Start()
		if *jsonOut {
			if printErr := printJSON(status); printErr != nil {
				return printErr
			}
			return err
		}
		if err != nil {
			return err
		}
		fmt.Printf("issueops daemon running pid=%d socket=%s\n", status.PID, status.Paths.Socket)
		return nil
	case "status":
		status := command.Status()
		if *jsonOut {
			return printJSON(status)
		}
		if daemondomain.IsReady(status) {
			fmt.Printf("running pid=%d socket=%s\n", status.PID, status.Paths.Socket)
		} else if status.Running || status.Reachable || status.PID > 0 {
			fmt.Printf("unverified code=%s pid=%d socket=%s\n", status.Code, status.PID, status.Paths.Socket)
		} else {
			fmt.Printf("stopped socket=%s\n", status.Paths.Socket)
		}
		return nil
	case "stop":
		status, err := command.Stop()
		if *jsonOut {
			if printErr := printJSON(status); printErr != nil {
				return printErr
			}
			return err
		}
		if err != nil {
			return err
		}
		fmt.Println(status.Message)
		return nil
	default:
		daemonUsage()
		return fmt.Errorf("unknown daemon subcommand %q", sub)
	}
}

func daemonUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops daemon start [--json]
  issueops daemon status [--json]
  issueops daemon stop [--json]
`)
}
