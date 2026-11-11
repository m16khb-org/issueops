package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyOverrideWarningSurvivesWrapping(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, path string)
		want  string
	}{
		{"parse", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "policy_override_parse_failed"},
		{"read", func(t *testing.T, path string) {
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}, "policy_override_read_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			path := filepath.Join(repo, ".issueops", "policy.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, path)
			_, err := readPolicyOverrides(repo)
			if err == nil {
				t.Fatal("readPolicyOverrides returned no error")
			}
			if got := policyOverrideWarning(err); got != tc.want {
				t.Fatalf("warning = %q, want %q", got, tc.want)
			}
			if got := policyOverrideWarning(fmt.Errorf("load: %w", err)); got != tc.want {
				t.Fatalf("wrapped warning = %q, want %q", got, tc.want)
			}
		})
	}
	if got := policyOverrideWarning(errors.New("something else")); got != "policy_override_failed" {
		t.Fatalf("unknown error warning = %q", got)
	}
}

func TestPolicyOverrideLoaderClassifiesReadAndParseFailures(t *testing.T) {
	setup := func(t *testing.T) (repo, path string) {
		t.Helper()
		repo = t.TempDir()
		path = filepath.Join(repo, ".issueops", "policy.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		return repo, path
	}

	t.Run("parse failure", func(t *testing.T) {
		repo, path := setup(t)
		if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		snapshot := OverrideLoader{}.Load(repo)
		if snapshot.Warning != "policy_override_parse_failed" || snapshot.Values != nil {
			t.Fatalf("snapshot = %+v", snapshot)
		}
		_, err := readPolicyOverrides(repo)
		if err == nil || !strings.HasPrefix(err.Error(), "policy_override_parse_failed: ") {
			t.Fatalf("readPolicyOverrides error = %v", err)
		}
	})

	t.Run("read failure", func(t *testing.T) {
		repo, path := setup(t)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		snapshot := OverrideLoader{}.Load(repo)
		if snapshot.Warning != "policy_override_read_failed" || snapshot.Values != nil {
			t.Fatalf("snapshot = %+v", snapshot)
		}
		_, err := readPolicyOverrides(repo)
		if err == nil || !strings.HasPrefix(err.Error(), "policy_override_read_failed: ") {
			t.Fatalf("readPolicyOverrides error = %v", err)
		}
	})
}
