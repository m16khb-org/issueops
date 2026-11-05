package state

import (
	"fmt"
	"time"
)

// ParseTime parses a stored state timestamp in RFC 3339 form, with or without fractional seconds. It returns an error for an empty or malformed value.
func ParseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty state timestamp")
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}
