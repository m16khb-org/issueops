package installutil

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMergeJSONMapFile(t *testing.T) {
	for _, content := range []string{"", " \n", `{"theme":"dark","mcpServers":{"other":{"command":"keep"},"issueops":{"command":"old"}}}`} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			server := map[string]any{"command": "new"}
			calls := 0
			config, err := MergeJSONMapFile(path, "mcpServers", "issueops", false, func() (map[string]any, error) {
				calls++
				return server, nil
			})
			if err != nil || calls != 1 {
				t.Fatalf("merge calls=%d err=%v", calls, err)
			}
			servers := config["mcpServers"].(map[string]any)
			if !reflect.DeepEqual(servers["issueops"], server) {
				t.Fatalf("server=%#v", servers["issueops"])
			}
			if content != "" && content != " \n" {
				if config["theme"] != "dark" || !reflect.DeepEqual(servers["other"], map[string]any{"command": "keep"}) {
					t.Fatalf("unrelated config lost: %#v", config)
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil || string(raw) != content {
				t.Fatalf("merge must not write: raw=%q err=%v", raw, err)
			}
		})
	}
}

func TestMergeJSONMapFileReadFailures(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		for _, content := range []string{"{", "[]", "null"} {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			if _, err := MergeJSONMapFile(path, "mcpServers", "issueops", dryRun, func() (map[string]any, error) {
				calls++
				return nil, nil
			}); err == nil || calls != 0 {
				t.Fatalf("malformed input calls=%d err=%v", calls, err)
			}
		}
		path := filepath.Join(t.TempDir(), "missing.json")
		if _, err := MergeJSONMapFile(path, "mcpServers", "issueops", dryRun, func() (map[string]any, error) {
			return map[string]any{"command": "new"}, nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("missing file was created: %v", err)
		}
		_, err := MergeJSONMapFile(t.TempDir(), "mcpServers", "issueops", dryRun, func() (map[string]any, error) {
			return map[string]any{"command": "new"}, nil
		})
		if (err == nil) != dryRun {
			t.Fatalf("directory read dryRun=%v err=%v", dryRun, err)
		}
	}
	want := errors.New("server unavailable")
	_, err := MergeJSONMapFile(filepath.Join(t.TempDir(), "missing"), "mcpServers", "issueops", false, func() (map[string]any, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("server error=%v", err)
	}
}

func TestVerifyJSONMapEntry(t *testing.T) {
	expected := map[string]any{"command": "new", "args": []string{"mcp"}}
	for _, tc := range []struct {
		name, content string
		wantError     bool
		hashCalls     int
	}{
		{"canonical", `{"theme":"dark","mcpServers":{"other":{"command":"keep"},"issueops":{"args":["mcp"],"command":"new"}}}`, false, 2},
		{"malformed", `{`, true, 0},
		{"missing parent", `{}`, true, 0},
		{"wrong parent", `{"mcpServers":[]}`, true, 0},
		{"missing entry", `{"mcpServers":{}}`, true, 0},
		{"wrong entry", `{"mcpServers":{"issueops":"opaque"}}`, true, 2},
		{"stale entry", `{"mcpServers":{"issueops":{"command":"old"}}}`, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			hashCalls := 0
			digest, err := VerifyJSONMapEntry(path, "mcpServers", "issueops", "host MCP readback",
				func() (map[string]any, error) { return expected, nil },
				func(value any) (string, error) {
					hashCalls++
					return SemanticSHA256(value)
				})
			if (err != nil) != tc.wantError || hashCalls != tc.hashCalls {
				t.Fatalf("digest=%s err=%v hash calls=%d want=%d", digest, err, hashCalls, tc.hashCalls)
			}
			if !tc.wantError {
				want, err := SemanticSHA256(expected)
				if err != nil || digest != want {
					t.Fatalf("entry digest=%s want=%s err=%v", digest, want, err)
				}
			}
		})
	}
}

func TestVerifyJSONMapEntryPropagatesErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	expectedCalls := 0
	expected := func() (map[string]any, error) {
		expectedCalls++
		return map[string]any{"command": "new"}, nil
	}
	if _, err := VerifyJSONMapEntry(path, "mcpServers", "issueops", "host MCP readback", expected, SemanticSHA256); !os.IsNotExist(err) || expectedCalls != 0 {
		t.Fatalf("missing file err=%v expected calls=%d", err, expectedCalls)
	}
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"issueops":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, failAt := range []int{1, 2} {
		calls := 0
		want := errors.New("digest unavailable")
		_, err := VerifyJSONMapEntry(path, "mcpServers", "issueops", "host MCP readback", expected, func(value any) (string, error) {
			calls++
			if calls == failAt {
				return "", want
			}
			return SemanticSHA256(value)
		})
		if !errors.Is(err, want) || calls != failAt {
			t.Fatalf("digest error=%v calls=%d", err, calls)
		}
	}
	want := errors.New("expected server unavailable")
	_, err := VerifyJSONMapEntry(path, "mcpServers", "issueops", "host MCP readback",
		func() (map[string]any, error) { return nil, want }, SemanticSHA256)
	if !errors.Is(err, want) {
		t.Fatalf("expected server error=%v", err)
	}
}
