package installutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// MergeJSONMapFile reads and merges one named entry under the object at the
// key path parent without writing the file. The entry is built after decoding
// so malformed input takes precedence.
func MergeJSONMapFile(path string, parent []string, entry string, dryRun bool, value func() (map[string]any, error)) (map[string]any, error) {
	config := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &config); err != nil {
			return nil, err
		}
		if config == nil {
			return nil, fmt.Errorf("JSON config must be an object")
		}
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) && !dryRun {
		return nil, err
	}
	entries := jsonObjectAt(config, parent, true)
	server, err := value()
	if err != nil {
		return nil, err
	}
	entries[entry] = server
	return config, nil
}

// RemoveJSONMapEntry returns the config without the named entry under the key
// path parent. removed is false when the file or entry is absent, so callers
// leave the file untouched.
func RemoveJSONMapEntry(path string, parent []string, entry string) (map[string]any, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil, false, nil
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, false, err
	}
	if config == nil {
		return nil, false, fmt.Errorf("JSON config must be an object")
	}
	entries := jsonObjectAt(config, parent, false)
	if _, ok := entries[entry]; !ok {
		return nil, false, nil
	}
	delete(entries, entry)
	return config, true, nil
}

// VerifyJSONMapEntry hashes only the named entry under the key path parent,
// not unrelated host settings.
func VerifyJSONMapEntry(path string, parent []string, entry, context string, expected func() (map[string]any, error), digest func(any) (string, error)) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", err
	}
	entries := jsonObjectAt(config, parent, false)
	if entries == nil {
		return "", fmt.Errorf("%s has no %s object", context, strings.Join(parent, "."))
	}
	actual, ok := entries[entry]
	if !ok {
		return "", fmt.Errorf("%s has no %s server", context, entry)
	}
	actualDigest, err := digest(actual)
	if err != nil {
		return "", err
	}
	server, err := expected()
	if err != nil {
		return "", err
	}
	expectedDigest, err := digest(server)
	if err != nil {
		return "", err
	}
	if actualDigest != expectedDigest {
		return "", fmt.Errorf("%s does not target the canonical binary and ISSUEOPS_ROOT", context)
	}
	return expectedDigest, nil
}

// jsonObjectAt walks the key path through nested objects. With create, a
// missing or non-object level becomes a new empty object, as the one-level
// merge always did; without it, the walk returns nil.
func jsonObjectAt(config map[string]any, path []string, create bool) map[string]any {
	current := config
	for _, key := range path {
		next, _ := current[key].(map[string]any)
		if next == nil {
			if !create {
				return nil
			}
			next = map[string]any{}
			current[key] = next
		}
		current = next
	}
	return current
}
