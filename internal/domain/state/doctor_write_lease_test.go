package state

import (
	statecontract "issueops/internal/contract/state"
	"reflect"
	"strings"
	"testing"
)

func TestDoctorRecognizesOnlyTheCanonicalRecordWriteLeaseFile(t *testing.T) {
	entries := []DoctorEntry{
		{Name: statecontract.RecordWriteLeaseFile, Path: "state/" + statecontract.RecordWriteLeaseFile},
		{Name: "another.lock", Path: "state/another.lock"},
		{Name: statecontract.RecordWriteLeaseFile + ".bak", Path: "state/" + statecontract.RecordWriteLeaseFile + ".bak"},
	}
	result := InspectDoctor("state", entries, nil)
	if len(result.Issues) != 2 {
		t.Fatalf("expected only the two foreign files, got %+v", result.Issues)
	}
	for _, issue := range result.Issues {
		if issue.Code != "unexpected_file" || issue.Path == entries[0].Path {
			t.Fatalf("canonical lease file misclassified: %+v", issue)
		}
	}
}

func TestDoctorClassifiesCurrentAndRetiredStateEntries(t *testing.T) {
	entries := []DoctorEntry{
		{Name: "channel", Path: "state/channel", IsDir: true},
		{Name: "upstream", Path: "state/upstream", IsDir: true},
		{Name: "daemon", Path: "state/daemon", IsDir: true},
		{Name: "hook-failures.jsonl", Path: "state/hook-failures.jsonl"},
		{Name: "hook-metrics.jsonl", Path: "state/hook-metrics.jsonl"},
		{Name: ".last-store-maintain", Path: "state/.last-store-maintain"},
		{Name: "issueops-migration-receipt.json", Path: "state/issueops-migration-receipt.json"},
		{Name: "stray", Path: "state/stray", IsDir: true},
	}
	result := InspectDoctor("state", entries, nil)
	codes := map[string]string{}
	for _, issue := range result.Issues {
		codes[issue.Path] = issue.Code
		if issue.Code == "retired_path" && (issue.Severity != "warning" || !strings.Contains(issue.Message, "issueops update")) {
			t.Fatalf("retired issue = %+v", issue)
		}
	}
	want := map[string]string{
		"state/daemon": "retired_path", "state/hook-failures.jsonl": "retired_path", "state/hook-metrics.jsonl": "retired_path",
		"state/.last-store-maintain": "retired_path", "state/issueops-migration-receipt.json": "retired_path", "state/stray": "unexpected_directory",
	}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("issue codes = %v, want %v", codes, want)
	}
	names := []string{}
	for _, entry := range RetiredStateEntries() {
		names = append(names, entry.Name)
	}
	if !reflect.DeepEqual(names, []string{".last-store-maintain", "daemon", "hook-failures.jsonl", "hook-metrics.jsonl", "issueops-migration-receipt.json"}) {
		t.Fatalf("retired entries = %v", names)
	}
}

func TestInspectDoctorHealthReflectsIssuesAndOrdersThemByPathCodeMessage(t *testing.T) {
	healthy := InspectDoctor("state", nil, nil)
	if !healthy.OK || !healthy.Healthy {
		t.Fatalf("empty state must be healthy: %+v", healthy)
	}
	entries := []DoctorEntry{
		{Name: "zeta", Path: "state/zeta"},
		{Name: "alpha", Path: "state/alpha"},
	}
	rows := []DoctorRecord{
		{Key: "k", Path: "state/alpha", InvalidRecord: true},
		{Key: "k2", Path: "state/alpha", InvalidKey: "zzz"},
		{Key: "k3", Path: "state/alpha", InvalidKey: "aaa"},
	}
	result := InspectDoctor("state", entries, rows)
	if result.Checked != 3 {
		t.Fatalf("checked=%d want 3", result.Checked)
	}
	if !result.OK || result.Healthy {
		t.Fatalf("issues must make state unhealthy: %+v", result)
	}
	var got []string
	for _, issue := range result.Issues {
		got = append(got, issue.Path+" "+issue.Code+" "+issue.Message)
	}
	want := []string{"state/alpha invalid_key aaa", "state/alpha invalid_key zzz", "state/alpha invalid_state invalid state", "state/alpha unexpected_file state directory contains an unexpected file", "state/zeta unexpected_file state directory contains an unexpected file"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("issues=%v want %v", got, want)
	}
}
