package workercli

import (
	"issueops/cmd/issueops/jsonout"
	workerapp "issueops/internal/application/worker"
	"strings"
)

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
