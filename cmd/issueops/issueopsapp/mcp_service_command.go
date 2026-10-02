package issueopsapp

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"issueops/internal/contract/mcpservice"
	"issueops/internal/port"
)

type mcpServiceCommand struct {
	service port.MCPService
	stdout  io.Writer
}

func runMCPService(args []string) error {
	return mcpServiceCommand{service: newMCPService(), stdout: os.Stdout}.Run(args)
}

// Run maps start/stop/status onto the injected service port. The DTO carries
// no secret, so it is printed as-is.
func (command mcpServiceCommand) Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("issueops mcp service requires start, stop, or status")
	}
	action := args[0]
	fs := flag.NewFlagSet("issueops mcp service "+action, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected mcp service argument %q", fs.Arg(0))
	}
	var call func(context.Context) (mcpservice.Status, error)
	switch action {
	case "start":
		call = command.service.Start
	case "stop":
		call = command.service.Stop
	case "status":
		call = command.service.Status
	default:
		return fmt.Errorf("unknown mcp service action %q", action)
	}
	status, err := call(context.Background())
	if *jsonOut {
		encoder := json.NewEncoder(command.stdout)
		encoder.SetIndent("", "  ")
		if printErr := encoder.Encode(status); printErr != nil {
			return printErr
		}
		return err
	}
	if _, printErr := fmt.Fprintf(command.stdout, "status: %s\nurl: %s\npid: %d\nerror_code: %s\n", status.Status, status.URL, status.PID, status.ErrorCode); printErr != nil {
		return printErr
	}
	return err
}

func newMCPService() port.MCPService { return newSupervisorMCPService() }
