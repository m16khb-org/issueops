package operationalhealth

import (
	"strings"
	"time"
)

// parseTaskCompletedAt accepts the RFC3339 (and RFC3339Nano) completed_at
// values Orca emits; any other layout is invalid input.
func parseTaskCompletedAt(raw string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
}
