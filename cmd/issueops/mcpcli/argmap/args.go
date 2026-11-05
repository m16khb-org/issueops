// Package argmap reads typed values out of the loosely typed argument maps of MCP tool calls. Each accessor tolerates a nil map, a missing key or a value of the wrong type.
package argmap

import (
	"strconv"
	"strings"
)

// String returns args[key] when it is a string, and "" otherwise.
func String(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// StringDefault returns args[key] when it is a non-empty string, and fallback otherwise.
func StringDefault(args map[string]any, key, fallback string) string {
	if v := String(args, key); v != "" {
		return v
	}
	return fallback
}

// StringSlice returns a copy of args[key] as a string slice. It accepts a []string, a []any (skipping non-string items) or a comma-separated string whose parts are trimmed and non-empty. A nil map or any other type yields nil.
func StringSlice(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	switch v := args[key].(type) {
	case []string:
		return append([]string{}, v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return splitCSV(v)
	default:
		return nil
	}
}

// Bool returns args[key] when it is a bool or a string accepted by strconv.ParseBool, and false otherwise.
func Bool(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(v)
		return err == nil && parsed
	default:
		return false
	}
}

// Int returns args[key] as an int when it is a float64 (truncated), an int or a decimal string, and fallback otherwise.
func Int(args map[string]any, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}

// Int64 returns args[key] as an int64 when it is a float64 (truncated), an int64, an int or a decimal string, and fallback otherwise.
func Int64(args map[string]any, key string, fallback int64) int64 {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

// Float returns args[key] as a float64 when it is a float64, an int, an int64 or a parsable string, and fallback otherwise.
func Float(args map[string]any, key string, fallback float64) float64 {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
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
