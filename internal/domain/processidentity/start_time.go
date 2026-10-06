package processidentity

import (
	"fmt"
	"strings"
	"time"
)

// CanonicalStartTime normalizes a C-locale `ps lstart` observation, an RFC3339
// value, or a Linux tick receipt into one comparable start-time string.
func CanonicalStartTime(value string, location *time.Location) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("process start time is empty")
	}
	if strings.HasPrefix(value, "linux:") {
		return value, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format(time.RFC3339Nano), nil
	}
	for _, layout := range []string{"Mon Jan _2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("unsupported process start time %q", value)
}
