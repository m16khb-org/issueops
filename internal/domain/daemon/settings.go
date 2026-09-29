package daemon

import (
	"strconv"
	"time"
)

const (
	DefaultMaxConnections  = 256
	AbsoluteMaxConnections = 4096
)

func MaxConnections(value string) int {
	if value == "" {
		return DefaultMaxConnections
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 || parsed > AbsoluteMaxConnections {
		return DefaultMaxConnections
	}
	return parsed
}

func IdleTimeout(value string) time.Duration {
	if d, err := time.ParseDuration(value); err == nil && d > 0 {
		return d
	}
	return 30 * time.Minute
}
