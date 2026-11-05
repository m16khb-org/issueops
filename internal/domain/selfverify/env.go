package selfverify

import "strings"

// MergeEnvOverrides merges KEY=VALUE entries from base and overrides, keeping the first position of each
// key and letting later entries replace earlier values. Entries without a key are dropped.
func MergeEnvOverrides(base []string, overrides []string) []string {
	result := make([]string, 0, len(base)+len(overrides))
	indexByKey := map[string]int{}
	for _, entry := range base {
		key, ok := envEntryKey(entry)
		if !ok {
			continue
		}
		if idx, exists := indexByKey[key]; exists {
			result[idx] = entry
			continue
		}
		indexByKey[key] = len(result)
		result = append(result, entry)
	}
	for _, entry := range overrides {
		key, ok := envEntryKey(entry)
		if !ok {
			continue
		}
		if idx, exists := indexByKey[key]; exists {
			result[idx] = entry
			continue
		}
		indexByKey[key] = len(result)
		result = append(result, entry)
	}
	return result
}

func envEntryKey(entry string) (string, bool) {
	idx := strings.IndexByte(entry, '=')
	if idx <= 0 {
		return "", false
	}
	return entry[:idx], true
}
