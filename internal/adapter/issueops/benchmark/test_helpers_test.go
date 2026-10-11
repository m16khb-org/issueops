package benchmark

import "slices"

func containsString(items []string, want string) bool {
	return slices.Contains(items, want)
}
