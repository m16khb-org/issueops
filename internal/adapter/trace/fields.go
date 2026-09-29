package trace

import "strings"

func nestedMap(doc map[string]any, key string) map[string]any {
	raw, ok := doc[key]
	if !ok {
		return nil
	}
	child, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return child
}

func stringField(doc map[string]any, key string) string {
	raw, ok := doc[key]
	if !ok {
		return ""
	}
	if s, ok := raw.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
func intField(doc map[string]any, key string) int {
	raw, ok := doc[key]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func boolField(doc map[string]any, key string) bool {
	raw, ok := doc[key]
	if !ok {
		return true
	}
	v, ok := raw.(bool)
	return ok && v
}

func stringSliceField(doc map[string]any, key string) []string {
	raw, ok := doc[key]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := []string{}
	for _, item := range items {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
