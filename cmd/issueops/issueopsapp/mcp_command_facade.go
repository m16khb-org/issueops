package issueopsapp

import (
	"fmt"
	"strings"
)

func runMCPCommand(args []string) error {
	if len(args) == 0 {
		return runMCP()
	}
	switch args[0] {
	case "cleanup":
		return runMCPCleanup(args[1:])
	case "authorize":
		return runMCPAuthorize(args[1:])
	case "service":
		return runMCPService(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return runMCPHTTP(args)
		}
		return fmt.Errorf("unknown mcp subcommand %q", args[0])
	}
}

func runMCPCleanup(args []string) error { return newMCPCleanupCommand().Run(args) }
