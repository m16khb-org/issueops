package policycli

import (
	"encoding/json"
	auditapp "issueops/internal/application/audit"
	policyapp "issueops/internal/application/policy"
	"os"
	"strings"
)

// Command binds a policy transport to one workspace and application instance.
type Command struct {
	DefaultRoot string
	Policy      policyapp.Service
	Audit       auditapp.Service
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

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
