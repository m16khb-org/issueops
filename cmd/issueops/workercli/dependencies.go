// Package workercli implements the `issueops worker` command: enqueueing, running read-only, inspecting,
// listing, cleaning up, and cancelling local worker jobs.
package workercli

import (
	"issueops/cmd/issueops/jsonout"
	workerapp "issueops/internal/application/worker"
	"strings"
)

// Command runs the `issueops worker` subcommands against a worker Service. ResolveTarget maps a repo flag
// to the target workspace root.
type Command struct {
	Service       workerapp.Service
	ResolveTarget func(string) string
}

var printJSON = jsonout.Print

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
