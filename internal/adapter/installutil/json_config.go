package installutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// MergeJSONMapFile reads and merges one named entry without writing the file.
// The entry is built after decoding so malformed input takes precedence.
func MergeJSONMapFile(path, parent, entry string, dryRun bool, value func() (map[string]any, error)) (map[string]any, error) {
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
	entries, _ := config[parent].(map[string]any)
	if entries == nil {
		entries = map[string]any{}
		config[parent] = entries
	}
	server, err := value()
	if err != nil {
		return nil, err
	}
	entries[entry] = server
	return config, nil
}

// RemoveJSONMapEntry returns the config without the named entry. removed is
// false when the file or entry is absent, so callers leave the file untouched.
func RemoveJSONMapEntry(path, parent, entry string) (map[string]any, bool, error) {
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
	entries, _ := config[parent].(map[string]any)
	if _, ok := entries[entry]; !ok {
		return nil, false, nil
	}
	delete(entries, entry)
	return config, true, nil
}

// VerifyJSONMapEntry hashes only the named entry, not unrelated host settings.
func VerifyJSONMapEntry(path, parent, entry, context string, expected func() (map[string]any, error), digest func(any) (string, error)) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", err
	}
	entries, ok := config[parent].(map[string]any)
	if !ok {
		return "", fmt.Errorf("%s has no %s object", context, parent)
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
