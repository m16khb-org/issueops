package selfverify

import (
	"reflect"
	"testing"
)

func TestMergeEnvOverridesReplacesExistingKeys(t *testing.T) {
	env := MergeEnvOverrides(
		[]string{"HOME=/real-home", "PATH=/bin", "HOME=/duplicate-home"},
		[]string{"HOME=/fixture-home", "ISSUEOPS_ROOT=/fixture-root"},
	)
	values := map[string]string{}
	counts := map[string]int{}
	for _, entry := range env {
		key, ok := EnvEntryKey(entry)
		if !ok {
			t.Fatalf("invalid env entry remained: %q", entry)
		}
		counts[key]++
		values[key] = entry
	}
	if counts["HOME"] != 1 || values["HOME"] != "HOME=/fixture-home" {
		t.Fatalf("HOME override was not unique and last-wins: counts=%v values=%v env=%v", counts, values, env)
	}
	if values["PATH"] != "PATH=/bin" {
		t.Fatalf("PATH was not preserved: %v", env)
	}
	if values["ISSUEOPS_ROOT"] != "ISSUEOPS_ROOT=/fixture-root" {
		t.Fatalf("ISSUEOPS_ROOT override missing: %v", env)
	}
}

func TestMergeEnvOverridesDropsEntriesWithoutKeyAndAcceptsMoreOverridesThanBase(t *testing.T) {
	got := MergeEnvOverrides([]string{"A=1"}, []string{"=novalue", "NOEQ", "B=2", "C=3", "D=4"})
	want := []string{"A=1", "B=2", "C=3", "D=4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	if key, ok := EnvEntryKey("=x"); ok || key != "" {
		t.Fatalf("empty key must be rejected: %q %v", key, ok)
	}
}
