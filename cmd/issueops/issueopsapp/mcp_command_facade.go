package issueopsapp

import (
	"fmt"
)

func runMCPCommand(args []string) error {
	if len(args) == 0 {
		return runMCP()
	}
	if args[0] != "cleanup" {
		return fmt.Errorf("unknown mcp subcommand %q", args[0])
	}
	return runMCPCleanup(args[1:])
}

func runMCPCleanup(args []string) error { return newMCPCleanupCommand().Run(args) }
